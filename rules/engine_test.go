package rules

import (
	"errors"
	"testing"

	"github.com/IBM-Bluemix/go-etcd-lock/lock"
	"github.com/coreos/etcd/client"
)

func channelWriteAfterCall(channel chan bool, f func()) {
	f()
	channel <- true
}

type testCallback struct {
	called chan bool
}

func (tc *testCallback) callback(task *RuleTask) {
	tc.called <- true
}

type testLocker struct {
	channel  chan bool
	errorMsg *string
}

func (tlkr *testLocker) Acquire(key string, ttl uint64) (lock.Lock, error) {
	if tlkr.errorMsg != nil {
		return nil, errors.New(*tlkr.errorMsg)
	}
	lock := testLock{
		channel: tlkr.channel,
	}
	return &lock, nil
}

func (tlkr *testLocker) Wait(key string) error {
	return nil
}

func (tlkr *testLocker) WaitAcquire(key string, ttl uint64) (lock.Lock, error) {
	err := tlkr.Wait(key)
	if err != nil {
		return nil, err
	}
	return tlkr.Acquire(key, ttl)
}

type testLock struct {
	channel chan bool
}

func (tl *testLock) Release() error {
	tl.channel <- true
	return nil
}

func TestEngineConstructor(t *testing.T) {
	eng := NewEngine(client.Config{
		// This does not need to be a real endpoint
		Endpoints: []string{"http://10.0.0.1:4001"},
	}, getTestLogger())
	value := "val"
	rule, _ := NewEqualsLiteralRule("/key", &value)
	eng.AddRule(rule, "/lock", dummyCallback)
	eng.Run()
}
