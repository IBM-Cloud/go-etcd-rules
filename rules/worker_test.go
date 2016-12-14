package rules

import (
	"testing"

	"github.com/coreos/etcd/client"
	"github.com/stretchr/testify/assert"
	"github.com/uber-go/zap"
)

func TestWorkerSingleRun(t *testing.T) {
	assert.True(t, true)
	e := newEngine(client.Config{}, getTestLogger(), EngineLockTimeout(300))
	channel := e.workChannel
	lockChannel := make(chan bool)
	locker := testLocker{
		channel: lockChannel,
	}
	api := mapReadAPI{}
	w := worker{
		engine:   &e,
		locker:   &locker,
		api:      &api,
		workerID: "testworker",
	}
	attrMap := map[string]string{}
	attr := mapAttributes{
		attr: attrMap,
	}
	conf := client.Config{}
	task := RuleTask{
		Attr:   &attr,
		Conf:   conf,
		Logger: zap.New(zap.NewTextEncoder()),
	}
	cbChannel := make(chan bool)
	callback := testCallback{
		called: cbChannel,
	}
	rule := dummyRule{
		satisfiedResponse: true,
	}
	rw := ruleWork{
		ruleTask:         task,
		ruleTaskCallback: callback.callback,
		lockKey:          "key",
		rule:             &rule,
	}
	go w.singleRun()
	channel <- rw
	assert.True(t, <-cbChannel)
	assert.True(t, <-lockChannel)

	errorMsg := "Some error"
	locker.errorMsg = &errorMsg

	callChannel := make(chan bool)

	go channelWriteAfterCall(callChannel, w.singleRun)
	channel <- rw
	assert.True(t, <-callChannel)
	assert.Equal(t, 0, len(cbChannel))
	assert.Equal(t, 0, len(lockChannel))

	rule = dummyRule{
		satisfiedResponse: false,
	}
	go channelWriteAfterCall(callChannel, w.singleRun)
	channel <- rw
	assert.True(t, <-callChannel)
	assert.Equal(t, 0, len(cbChannel))
	assert.Equal(t, 0, len(lockChannel))
}
