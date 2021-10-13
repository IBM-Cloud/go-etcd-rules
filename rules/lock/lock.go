package lock

import (
	"time"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/clientv3/concurrency"
	"golang.org/x/net/context"
)

type RuleLocker interface {
	Lock(string, ...LockOption) (RuleLock, error)
}

type RuleLock interface {
	Unlock()
}

type lockoptions struct {
	// TODO add options
}

type LockOption func(lo *lockoptions)

// NewV3Locker creates a locker backed by etcd V3.
func NewV3Locker(cl *clientv3.Client, lockTimeout int) RuleLocker {
	return &v3Locker{
		cl:          cl,
		lockTimeout: lockTimeout,
	}
}

type v3Locker struct {
	cl          *clientv3.Client
	lockTimeout int
}

func (v3l *v3Locker) Lock(key string, options ...LockOption) (RuleLock, error) {
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

func (v3l *v3Lock) Unlock() {
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
	}
}
