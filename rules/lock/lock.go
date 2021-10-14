package lock

import (
	"errors"
	"time"

	"go.etcd.io/etcd/clientv3"
	"golang.org/x/net/context"

	"github.com/IBM-Cloud/go-etcd-rules/concurrency"
)

type RuleLocker interface {
	Lock(string, ...Option) (RuleLock, error)
}

type RuleLock interface {
	Unlock() error
}

type NewSession func(context.Context) (*concurrency.Session, error)

// NewV3Locker creates a locker backed by etcd V3.
func NewV3Locker(cl *clientv3.Client, lockTimeout int) RuleLocker {
	// The TTL is for the lease associated with the session, in seconds. While the session is still open,
	// the lease's TTL will keep getting renewed to keep it from expiring, so all this really does is
	// set the amount of time it takes for the lease to expire if the lease stops being renewed due
	// to the application shutting down before a session could be properly closed.
	newSession := func(_ context.Context) (*concurrency.Session, error) {
		return concurrency.NewSession(cl, concurrency.WithTTL(30))
	}
	return NewSessionLocker(newSession, lockTimeout)
}

func NewSessionLocker(newSession NewSession, lockTimeout int) RuleLocker {
	return &v3Locker{
		lockTimeout: lockTimeout,
		newSession:  newSession,
	}
}

type v3Locker struct {
	lockTimeout int
	newSession  NewSession
}

func (v3l *v3Locker) Lock(key string, options ...Option) (RuleLock, error) {
	return v3l.lockWithTimeout(key, v3l.lockTimeout)
}
func (v3l *v3Locker) lockWithTimeout(key string, timeout int) (RuleLock, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	s, err := v3l.newSession(ctx)
	if err != nil {
		return nil, err
	}
	m := concurrency.NewMutex(s, key)
	err = m.Lock(ctx)
	if err != nil {
		return nil, err
	}
	return &v3Lock{
		mutex:   m,
		session: s,
	}, nil
}

type v3Lock struct {
	mutex   *concurrency.Mutex
	session *concurrency.Session
}

// ErrNilMutex indicates that the lock has a nil mutex
var ErrNilMutex = errors.New("mutex is nil")

func (v3l *v3Lock) Unlock() error {
	if v3l.mutex != nil {
		// TODO: Should the timeout for this be configurable too? Or use the same value as lock?
		//       It's a slightly different case in that here we want to make sure the unlock
		//       succeeds to free it for the use of others. In the lock case we want to give up
		//       early if someone already has the lock.
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(5)*time.Second)
		defer cancel()
		err := v3l.mutex.Unlock(ctx)
		if err == nil && v3l.session != nil {
			v3l.session.Close()
		}
		return err
	}
	return ErrNilMutex
}
