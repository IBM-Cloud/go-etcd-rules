package lock

import (
	"errors"
	"sync"
)

func NewMapLocker() RuleLocker {
	ml := newMapLocker()
	return toggleLockerAdapter{
		toggle:    ml.toggle,
		close:     ml.close,
		errLocked: ErrLockedLocally,
	}
}

type mapLocker struct {
	once      *sync.Once
	stopCh    chan struct{}
	lockLocal chan mapLockItem
}

// ErrLockedLocally indicates that a local goroutine holds the lock
// and no attempt will be made to obtain the lock via etcd.
var ErrLockedLocally = errors.New("locked locally")

type mapLockItem struct {
	// The key to lock
	key string
	// When lock is true the request is to lock, otherwise it is to unlock
	lock bool
	// true is sent in the response channel if the operator was successful
	// unlocks are always successful.
	response chan<- bool
}

func (ml mapLocker) close() {
	ml.once.Do(func() {
		// This is thread safe because no goroutine is writing
		// to this channel.
		close(ml.stopCh)
	})
}

func (ml mapLocker) toggle(key string, lock bool) bool {
	resp := make(chan bool)
	item := mapLockItem{
		key:      key,
		response: resp,
		lock:     lock,
	}
	select {
	case <-ml.stopCh:
		// Return false if the locker is closed.
		return false
	case ml.lockLocal <- item:
	}
	out := <-resp
	return out
}

func newMapLocker() mapLocker {
	locker := mapLocker{
		stopCh:    make(chan struct{}),
		lockLocal: make(chan mapLockItem),
		once:      new(sync.Once),
	}
	// Thread safety is achieved by allowing only one goroutine to access
	// this map and having it read from a channel with multiple goroutines
	// writing to it.
	locks := make(map[string]bool)
	count := 0
	go func() {
		for item := range locker.lockLocal {
			count++
			// extraneous else's and continue's to make flow clearer.
			if item.lock {
				// Requesting a lock
				if locks[item.key] {
					// Lock already obtained
					item.response <- false
					continue
				} else {
					// Lock available
					locks[item.key] = true
					item.response <- true
					continue
				}
			} else {
				// Requesting an unlock
				delete(locks, item.key)
				item.response <- true
				continue
			}
		}
	}()
	return locker
}

type toggleLocker interface {
	toggle(key string, lock bool) bool
	close()
}
type toggleLockerAdapter struct {
	toggle    func(key string, lock bool) bool
	close     func()
	errLocked error
}

func (tla toggleLockerAdapter) Lock(key string, options ...Option) (RuleLock, error) {
	ok := tla.toggle(key, true)
	if !ok {
		return nil, tla.errLocked
	}
	return toggleLock{
		toggle: tla.toggle,
		key:    key,
	}, nil
}

type toggleLock struct {
	toggle func(key string, lock bool) bool
	key    string
}

func (tl toggleLock) Unlock() error {
	_ = tl.toggle(tl.key, false)
	return nil
}
