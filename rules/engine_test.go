package rules

import (
	"testing"
	"time"

	"github.com/IBM-Cloud/go-etcd-rules/rules/teststore"
	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
	v3 "go.etcd.io/etcd/client/v3"
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

func TestV3EngineConstructor(t *testing.T) {
	cfg, _ := teststore.InitV3Etcd(t)
	eng := NewV3Engine(cfg, getTestLogger())
	value := "val"
	rule, _ := NewEqualsLiteralRule("/key", &value)
	eng.AddRule(rule, "/lock", v3DummyCallback, RuleID("test"))
	assert.PanicsWithValue(t, "Rule ID option missing", func() { eng.AddRule(rule, "/lock", v3DummyCallback) })
	err := eng.AddPolling("/polling", rule, 30, v3DummyCallback)
	assert.NoError(t, err)
	assertEngineRunStop(t, eng)

	eng = NewV3Engine(cfg, getTestLogger(), KeyExpansion(map[string][]string{"a:": {"b"}}))
	eng.AddRule(rule, "/lock", v3DummyCallback, RuleLockTimeout(30), RuleID("test"))
	err = eng.AddPolling("/polling", rule, 30, v3DummyCallback)
	assert.NoError(t, err)
	err = eng.AddPolling("/polling[", rule, 30, v3DummyCallback)
	assert.Error(t, err)
	assertEngineRunStop(t, eng)
}

func assertEngineRunStop(t *testing.T, eng V3Engine) {
	eng.Run()
	eng.Stop()
	stopped := false
	for i := 0; i < 60; i++ {
		stopped = eng.IsStopped()
		if stopped {
			break
		}
		time.Sleep(time.Second)
	}
	assert.True(t, stopped)
}

func TestV3EngineWorkBuffer(t *testing.T) {
	// verifies the work channel buffer behavior based on the engine setting.  since we don't start the engine in this
	// test case, there are no workers to read from the work channel; therefore, only the channel buffering is tested.
	cfg, _ := teststore.InitV3Etcd(t)

	// unbuffered engine work channel blocks
	engI := NewV3Engine(cfg, getTestLogger())
	eng, ok := engI.(*v3Engine)
	require.True(t, ok)
	select {
	case eng.workChannel <- v3RuleWork{}:
		t.Fatal("unbuffered engine work channel should block")
	default:
		// work channel not ready to read
	}

	// buffered engine work channel can accept values
	bufSize := 3
	engI = NewV3Engine(cfg, getTestLogger(), EngineRuleWorkBuffer(bufSize))
	eng, ok = engI.(*v3Engine)
	require.True(t, ok)
	for i := 0; i < bufSize+1; i++ {
		select {
		case eng.workChannel <- v3RuleWork{}:
			if i == bufSize {
				t.Fatal("work channel buffer should be full but value accepted")
				continue
			}
			t.Log("accepted work")
		default:
			if i < bufSize {
				t.Fatal("work channel buffer not full and should accept values")
				continue
			}
			t.Log("work channel full. new work blocking.")
		}
	}
}

func TestV3CallbackWrapper(t *testing.T) {
	_, c := teststore.InitV3Etcd(t)
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
			ttlResp, err := c.TimeToLive(context.Background(), v3.LeaseID(leaseID))
			if assert.NoError(t, err) {
				assert.InDelta(t, ttlResp.TTL, 30, 5)
			}
		}
	}
}
