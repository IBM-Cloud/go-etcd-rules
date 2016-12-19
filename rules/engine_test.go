package rules

import (
	"errors"
	"testing"

	"github.com/IBM-Bluemix/go-etcd-lock/lock"
	"github.com/stretchr/testify/assert"
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
	cfg, _, _ := initEtcd()
	eng := NewEngine(cfg, getTestLogger())
	value := "val"
	rule, _ := NewEqualsLiteralRule("/key", &value)
	eng.AddRule(rule, "/lock", dummyCallback)
	eng.AddPolling("/polling", rule, 30, dummyCallback)
	eng.Run()
	eng = NewEngine(cfg, getTestLogger(), KeyExpansion(map[string][]string{"a:": {"b"}}))
	eng.AddRule(rule, "/lock", dummyCallback, RuleLockTimeout(30))
	eng.AddPolling("/polling", rule, 30, dummyCallback)
	err := eng.AddPolling("/polling[", rule, 30, dummyCallback)
	assert.Error(t, err)
	eng.Run()
}

func TestCallbackWrapper(t *testing.T) {
	cfg, _, _ := initEtcd()
	task := RuleTask{
		Attr:   &mapAttributes{values: map[string]string{"a": "b"}},
		Conf:   cfg,
		Logger: getTestLogger(),
	}
	cbw := callbackWrapper{
		callback:       dummyCallback,
		ttl:            30,
		ttlPathPattern: "/:a/ttl",
	}
	cbw.doRule(&task)
}
