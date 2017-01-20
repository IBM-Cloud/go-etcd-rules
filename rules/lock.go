package rules

import (
	"time"

	"github.com/IBM-Bluemix/go-etcd-lock/lock"
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
	s, err1 := concurrency.NewSession(v3l.cl, concurrency.WithTTL(ttl))
	if err1 != nil {
		return nil, err1
	}
	m := concurrency.NewMutex(s, key)
	ctx, canfunc := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer canfunc()
	err2 := m.Lock(ctx)
	if err2 != nil {
		return nil, err2
	}
	return &v3Lock{m}, nil
}

type v3Lock struct {
	mutex *concurrency.Mutex
}

func (v3l *v3Lock) unlock() {
	ctx, canfunc := context.WithTimeout(context.Background(), time.Duration(5)*time.Second)
	err := v3l.mutex.Unlock(ctx)
	if err != nil {
	}
	canfunc()
}
