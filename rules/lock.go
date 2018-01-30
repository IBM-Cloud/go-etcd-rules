package rules

import (
	"time"

	"github.com/IBM-Cloud/go-etcd-lock/lock"
	"github.com/coreos/etcd/client"
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

type lockLock struct {
	lockInst lock.Lock
}

func (ll *lockLock) unlock() {
	err := ll.lockInst.Release()
	if err != nil {
	}
}

type lockLocker struct {
	locker lock.Locker
}

func (ll *lockLocker) lock(key string, ttl int) (ruleLock, error) {
	lockInst, err := ll.locker.Acquire(key, uint64(ttl))
	return &lockLock{lockInst}, err
}

func newLockLocker(cl client.Client) ruleLocker {
	return &lockLocker{
		locker: lock.NewEtcdLocker(cl, false),
	}
}

func newV3Locker(cl *clientv3.Client) ruleLocker {
	return &v3Locker{
		cl: cl,
	}
}

type v3Locker struct {
	cl *clientv3.Client
}

func (v3l *v3Locker) lock(key string, ttl int) (ruleLock, error) {
	return v3l.lockWithTimeout(key, ttl, 5)
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
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(5)*time.Second)
		defer cancel()
		err := v3l.mutex.Unlock(ctx)
		if err == nil && v3l.session != nil {
			v3l.session.Close()
		}
	}
}
