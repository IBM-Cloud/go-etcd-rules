package rules

import (
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/clientv3/concurrency"
	"golang.org/x/net/context"
)

type ruleLocker interface {
	lock(string, int) (ruleLock, error)
}

type ruleLock interface {
	unlock()
}

func newV3Locker(cl *clientv3.Client, lockTimeout int) ruleLocker {
	return &v3Locker{
		cl:          cl,
		lockTimeout: lockTimeout,
	}
}

type v3Locker struct {
	cl          *clientv3.Client
	lockTimeout int
}

func (v3l *v3Locker) lock(key string, ttl int) (ruleLock, error) {
	return v3l.lockWithTimeout(key, ttl, v3l.lockTimeout)
}
func (v3l *v3Locker) lockWithTimeout(key string, ttl int, timeout int) (ruleLock, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(ttl)*time.Second)
	defer cancel()
	s, err := concurrency.NewSession(v3l.cl, concurrency.WithTTL(ttl), concurrency.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	m := concurrency.NewMutex(s, key)
	ctx, cancel = context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
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

func (v3l *v3Lock) unlock() {
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
