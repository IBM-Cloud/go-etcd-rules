package rules

import (
	"errors"
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
	return &v3Locker{
		cl:          cl,
		getSession:  getSessn,
		lockTimeout: lockTimeout,
	}
}

type getSession func() (*concurrency.Session, error)

type v3Locker struct {
	cl          *clientv3.Client
	getSession  getSession
	lockTimeout time.Duration
}

func (v3l *v3Locker) lock(key string) (ruleLock, error) {
	return v3l.lockWithTimeout(key, v3l.lockTimeout)
}
func (v3l *v3Locker) lockWithTimeout(key string, timeout time.Duration) (ruleLock, error) {
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
		mutex: m,
	}, nil
}

type v3Lock struct {
	mutex *concurrency.Mutex
}

func (v3l *v3Lock) unlock() error {
	if v3l.mutex != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		return v3l.mutex.Unlock(ctx)
	}
	return errors.New("nil mutex")
}
