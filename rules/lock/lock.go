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

type GetSession func(context.Context) (*concurrency.Session, error)

// NewV3Locker creates a locker backed by etcd V3.
func NewV3Locker(cl *clientv3.Client, lockTimeout int) RuleLocker {
	// The TTL is for the lease associated with the session, in seconds. While the session is still open,
	// the lease's TTL will keep getting renewed to keep it from expiring, so all this really does is
	// set the amount of time it takes for the lease to expire if the lease stops being renewed due
	// to the application shutting down before a session could be properly closed.
	newSession := func(_ context.Context) (*concurrency.Session, error) {
		return concurrency.NewSession(cl, concurrency.WithTTL(30))
	}
	return NewSessionLocker(newSession, lockTimeout, true)
}

// NewSessionLocker creates a new locker with the provided session constructor. Note that
// if closeSession is false, it means that the session provided by getSession will not be
// closed but instead be reused. In that case the locker must be protected by another locker
// (for instance an in-memory locker) because locks within the same session are reentrant so
// two goroutines can obtain the same lock.
func NewSessionLocker(getSession GetSession, lockTimeout int, closeSession bool) RuleLocker {
	return &v3Locker{
		lockTimeout:  lockTimeout,
		newSession:   getSession,
		closeSession: closeSession,
	}
}

type v3Locker struct {
	lockTimeout  int
	newSession   GetSession
	closeSession bool
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
	err = m.TryLock(ctx)
	if err != nil {
		return nil, err
	}
	lock := &v3Lock{
		mutex: m,
	}
	if v3l.closeSession {
		lock.session = s
	}
	return lock, nil
}

type v3Lock struct {
	mutex   *concurrency.Mutex
	session *concurrency.Session
}

// ErrNilMutex indicates that the lock has a nil mutex
var ErrNilMutex = errors.New("mutex is nil")

func (v3l *v3Lock) Unlock() error {
	if v3l.mutex != nil {
		// This should be given every chance to complete, otherwise
		// a lock could prevent future interactions with a resource.
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		err := v3l.mutex.Unlock(ctx)
		// If the lock failed to be released, as least closing the session
		// will allow the lease it is associated with to expire.
		if v3l.session != nil {
			serr := v3l.session.Close()
			if err == nil {
				err = serr
			}
		}
		return err
	}
	return ErrNilMutex
}
