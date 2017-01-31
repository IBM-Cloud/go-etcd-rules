package rules

import (
	"errors"
	"testing"
	"time"

	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/clientv3"
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

func (tlkr *testLocker) lock(key string, ttl int) (ruleLock, error) {
	if tlkr.errorMsg != nil {
		return nil, errors.New(*tlkr.errorMsg)
	}
	tLock := testLock{
		channel: tlkr.channel,
	}
	return &tLock, nil
}

type testLock struct {
	channel chan bool
}

func (tl *testLock) unlock() {
	tl.channel <- true
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
	eng.Stop()
	stopped := false
	for i := 0; i < 20; i++ {
		stopped = eng.IsStopped()
		if stopped {
			break
		}
		time.Sleep(time.Second)
	}
	assert.True(t, stopped)
}

func TestV3EngineConstructor(t *testing.T) {
	cfg, _ := initV3Etcd()
	eng := NewV3Engine(cfg, getTestLogger())
	value := "val"
	rule, _ := NewEqualsLiteralRule("/key", &value)
	eng.AddRule(rule, "/lock", v3DummyCallback)
	eng.AddPolling("/polling", rule, 30, v3DummyCallback)
	eng.Run()
	eng = NewV3Engine(cfg, getTestLogger(), KeyExpansion(map[string][]string{"a:": {"b"}}))
	eng.AddRule(rule, "/lock", v3DummyCallback, RuleLockTimeout(30))
	eng.AddPolling("/polling", rule, 30, v3DummyCallback)
	err := eng.AddPolling("/polling[", rule, 30, v3DummyCallback)
	assert.Error(t, err)
	eng.Run()
	eng.Stop()
	stopped := false
	for i := 0; i < 20; i++ {
		stopped = eng.IsStopped()
		if stopped {
			break
		}
		time.Sleep(time.Second)
	}
	assert.True(t, stopped)
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
	// Bad configuration resulting in error creating client
	cfg = client.Config{}
	task = RuleTask{
		Attr:   &mapAttributes{},
		Conf:   cfg,
		Logger: getTestLogger(),
	}
	cbw.doRule(&task)
	// Bad configuration resulting in HTTP error
	cfg = client.Config{
		Endpoints: []string{"http://500.0.0.1:0"},
	}
	task = RuleTask{
		Attr:   &mapAttributes{},
		Conf:   cfg,
		Logger: getTestLogger(),
	}
	cbw.doRule(&task)
}

func TestV3CallbackWrapper(t *testing.T) {
	cfg, _ := initV3Etcd()
	task := V3RuleTask{
		Attr:   &mapAttributes{values: map[string]string{"a": "b"}},
		Conf:   &cfg,
		Logger: getTestLogger(),
	}
	cbw := v3CallbackWrapper{
		callback:       v3DummyCallback,
		ttl:            30,
		ttlPathPattern: "/:a/ttl",
	}
	t.Log("Testing valid rule")
	cbw.doRule(&task)
	// Bad configuration resulting in error creating client
	cfg = clientv3.Config{}
	task = V3RuleTask{
		Attr:   &mapAttributes{},
		Conf:   &cfg,
		Logger: getTestLogger(),
	}
	t.Log("Testing invalid configuration--no endpoints")
	cbw.doRule(&task)
	// Bad configuration resulting in HTTP error
	cfg = clientv3.Config{
		Endpoints: []string{"xyz://500.0.0.1:0"},
	}
	task = V3RuleTask{
		Attr:   &mapAttributes{},
		Conf:   &cfg,
		Logger: getTestLogger(),
	}
	t.Log("Testing invalid configuration--invalid host")
	cbw.doRule(&task)
}
