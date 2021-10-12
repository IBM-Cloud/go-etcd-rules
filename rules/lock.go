package rules

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/IBM-Cloud/go-etcd-rules/rules/concurrency"
	"go.etcd.io/etcd/clientv3"
	"golang.org/x/net/context"
)

type ruleLocker interface {
	lock(string) (ruleLock, error)
}

type ruleLock interface {
	unlock() error
}

func newV3Locker(cl *clientv3.Client, lockTimeout time.Duration, getSessn getSession) ruleLocker {
	locker := &v3Locker{
		getSession:  getSessn,
		lockTimeout: lockTimeout,
		lLocker:     newLocalLocker(),
	}
	return locker
}

type localLockItem struct {
	// The key to lock
	key string
	// When lock is true the request is to lock, otherwise it is to unlock
	lock bool
	// true is sent in the response channel if the operator was successful
	// unlocks are always successful.
	response chan<- bool
}

type localLocker struct {
	once      sync.Once
	stopCh    chan struct{}
	lockLocal chan localLockItem
}

func (ll localLocker) close() {
	ll.once.Do(func() {
		// This is thread safe because no goroutine is writing
		// to this channel.
		close(ll.stopCh)
	})
}

func (ll localLocker) toggle(key string, lock bool) bool {
	fmt.Println("***toggle called", lock)
	resp := make(chan bool)
	item := localLockItem{
		key:      key,
		response: resp,
		lock:     lock,
	}
	select {
	case <-ll.stopCh:
		// Return false if the locker is closed.
		return false
	case ll.lockLocal <- item:
	}
	out := <-resp
	fmt.Println("***Response received", out)
	return out
}

func newLocalLocker() localLocker {
	locker := localLocker{
		stopCh:    make(chan struct{}),
		lockLocal: make(chan localLockItem),
	}
	// Thread safety is achieved by allowing only one goroutine to access
	// this map and having it read from channels that multiple goroutines
	// writing to them.
	locks := make(map[string]bool)
	count := 0
	go func() {
		for item := range locker.lockLocal {
			count++
			fmt.Println(locks, count)
			fmt.Println("lockLocal", count)
			// extraneous else's and continue's to make flow clearer.
			if item.lock {
				if locks[item.key] {
					item.response <- false
					continue
				} else {
					locks[item.key] = true
					item.response <- true
					continue
				}
			} else {
				delete(locks, item.key)
				item.response <- true
				continue
			}
		}
	}()
	return locker
}

type getSession func() (*concurrency.Session, error)

type v3Locker struct {
	getSession  getSession
	lockTimeout time.Duration
	lLocker     localLocker
}

func (v3l *v3Locker) lock(key string) (ruleLock, error) {
	return v3l.lockWithTimeout(key, v3l.lockTimeout)
}

var errLockedLocally = errors.New("locked locally")

// Timeout in this case means how long the client will wait to determine
// whether the lock can be obtained. This call will return immediately once
// another client is known to hold the lock. There is no waiting for the lock
// to be released.
func (v3l *v3Locker) lockWithTimeout(key string, timeout time.Duration) (ruleLock, error) {
	fmt.Println("***lockWithTimeout called")
	if ok := v3l.lLocker.toggle(key, true); !ok {
		return nil, errLockedLocally
	}
	s, err := v3l.getSession()
	if err != nil {
		return nil, err
	}
	m := concurrency.NewMutex(s, key)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err = m.TryLock(ctx)
	if err != nil {
		return nil, err
	}
	return &v3Lock{
		mutex:  m,
		locker: v3l,
		key:    key,
	}, nil
}

type v3Lock struct {
	mutex  *concurrency.Mutex
	locker *v3Locker
	key    string
}

func (v3l *v3Lock) unlock() error {
	v3l.locker.lLocker.toggle(v3l.key, false)
	if v3l.mutex != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		return v3l.mutex.Unlock(ctx)
	}
	return errors.New("nil mutex")
}
