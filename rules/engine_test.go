package rules

import (
	"errors"
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

func channelWriteAfterCall(channel chan bool, f func()) {
	f()
	channel <- true
}

type testCallback struct {
	called chan bool
}

func (tc *testCallback) callback(task *V3RuleTask) {
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

func TestV3EngineConstructor(t *testing.T) {
	cfg, _ := initV3Etcd(t)
	eng := NewV3Engine(cfg, getTestLogger())
	value := "val"
	rule, _ := NewEqualsLiteralRule("/key", &value)
	eng.AddRule(rule, "/lock", v3DummyCallback)
	err := eng.AddPolling("/polling", rule, 30, v3DummyCallback)
	assert.NoError(t, err)
	eng.Run()
	eng = NewV3Engine(cfg, getTestLogger(), KeyExpansion(map[string][]string{"a:": {"b"}}))
	eng.AddRule(rule, "/lock", v3DummyCallback, RuleLockTimeout(30))
	err = eng.AddPolling("/polling", rule, 30, v3DummyCallback)
	assert.NoError(t, err)
	err = eng.AddPolling("/polling[", rule, 30, v3DummyCallback)
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

func TestV3CallbackWrapper(t *testing.T) {
	_, c := initV3Etcd(t)
	defer c.Close()
	task := V3RuleTask{
		Attr:   &mapAttributes{values: map[string]string{"a": "b"}},
		Logger: getTestLogger(),
	}
	cbw := v3CallbackWrapper{
		callback:       v3DummyCallback,
		ttl:            30,
		ttlPathPattern: "/:a/ttl",
		kv:             c,
		lease:          c,
	}
	cbw.doRule(&task)
	resp, err := c.Get(context.Background(), "/b/ttl")
	assert.NoError(t, err)
	if assert.Equal(t, 1, len(resp.Kvs)) {
		assert.Equal(t, "/b/ttl", string(resp.Kvs[0].Key))
		leaseID := resp.Kvs[0].Lease
		if assert.True(t, leaseID > 0) {
			ttlResp, err := c.TimeToLive(context.Background(), clientv3.LeaseID(leaseID))
			if assert.NoError(t, err) {
				assert.InDelta(t, ttlResp.TTL, 30, 5)
			}
		}
	}
}
