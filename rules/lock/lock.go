package lock

import (
	"errors"
	"time"

	v3 "go.etcd.io/etcd/client/v3"
	"golang.org/x/net/context"

	"github.com/IBM-Cloud/go-etcd-rules/concurrency"
)

type RuleLocker interface {
	Lock(string, ...Option) (RuleLock, error)
}

type RuleLock interface {
	Unlock() error
}

// NewV3Locker creates a locker backed by etcd V3.
func NewV3Locker(cl *v3.Client, lockTimeout int) RuleLocker {
	return &v3Locker{
		cl:          cl,
		lockTimeout: lockTimeout,
	}
}

type v3Locker struct {
	cl          *v3.Client
	lockTimeout int
}

func (v3l *v3Locker) Lock(key string, options ...Option) (RuleLock, error) {
	return v3l.lockWithTimeout(key, v3l.lockTimeout)
}
func (v3l *v3Locker) lockWithTimeout(key string, timeout int) (RuleLock, error) {
	// TODO once we switch to a shared session, we can get rid of the TTL option
	// and go to the default (60 seconds). This is the TTL for the lease that
	// is associated with the session and the lease is renewed before it expires
	// while the session is active (not closed). It is not the TTL of any locks;
	// those persist until Unlock is called or the process dies and the session
	// lease is allowed to expire.
	s, err := concurrency.NewSession(v3l.cl, concurrency.WithTTL(30))
	if err != nil {
		return nil, err
	}
	m := concurrency.NewMutex(s, key)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
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
