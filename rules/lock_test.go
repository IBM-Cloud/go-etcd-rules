package rules

import (
	"testing"

	"github.com/IBM-Bluemix/go-etcd-lock/lock"
	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/clientv3"
	"github.com/stretchr/testify/assert"
)

func TestLockLockerConstructor(t *testing.T) {
	cfg := client.Config{
		Endpoints: []string{"http://127.0.0.1:2379"},
	}
	c, err := client.New(cfg)
	assert.NoError(t, err)
	newLockLocker(c)
}

func TestLockLocker(t *testing.T) {
	dll := dummyLockLocker{}
	llckr := lockLocker{
		locker: &dll,
	}
	llck, err := llckr.lock("test1", 200)
	assert.NoError(t, err)
	assert.Equal(t, "test1", dll.acquiredKey)
	assert.False(t, dll.lockInst.releaseCalled)
	llck.unlock()
	assert.True(t, dll.lockInst.releaseCalled)
}

type dummyLockLocker struct {
	acquiredKey string
	acquiredTTL uint64
	lockInst    dummyLockLock
}

func (dll *dummyLockLocker) Acquire(key string, ttl uint64) (lock.Lock, error) {
	dll.acquiredKey = key
	dll.acquiredTTL = ttl
	dll.lockInst = dummyLockLock{
		releaseCalled: false,
	}
	return &dll.lockInst, nil
}

func (dll *dummyLockLocker) WaitAcquire(key string, ttl uint64) (lock.Lock, error) {
	return nil, nil
}

func (dll *dummyLockLocker) Wait(key string) error {
	return nil
}

type dummyLockLock struct {
	releaseCalled bool
}

func (dll *dummyLockLock) Release() error {
	dll.releaseCalled = true
	return nil
}

func TestV3Locker(t *testing.T) {
	cfg, cl := initV3Etcd()
	c, err := clientv3.New(cfg)
	assert.NoError(t, err)
	newV3Locker(c)
	rlckr := v3Locker{
		cl: cl,
	}
	rlck, err1 := rlckr.lock("test", 10)
	assert.NoError(t, err1)
	_, err2 := rlckr.lockWithTimeout("test", 10, 1)
	assert.Error(t, err2)
	rlck.unlock()

}
