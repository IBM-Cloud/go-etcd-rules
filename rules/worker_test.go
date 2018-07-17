package rules

import (
	"testing"

	"github.com/coreos/etcd/clientv3"
	"github.com/stretchr/testify/assert"
	"github.com/uber-go/zap"
	"golang.org/x/net/context"
)

func TestWorkerSingleRun(t *testing.T) {
	conf := clientv3.Config{
		Endpoints: []string{""},
	}
	cl, err := clientv3.New(conf)
	assert.NoError(t, err)
	e := newV3Engine(conf, getTestLogger(), cl, EngineLockTimeout(300))
	channel := e.workChannel
	lockChannel := make(chan bool)
	locker := testLocker{
		channel: lockChannel,
	}
	api := mapReadAPI{}
	w := v3Worker{
		baseWorker: baseWorker{
			api:      &api,
			locker:   &locker,
			workerID: "testworker",
		},
		engine: &e,
	}
	attrMap := map[string]string{}
	attr := mapAttributes{
		values: attrMap,
	}
	ctx, cancel := context.WithCancel(context.Background())
	task := V3RuleTask{
		Attr:     &attr,
		Conf:     &clientv3.Config{},
		Logger:   zap.New(zap.NewTextEncoder()),
		Metadata: map[string]string{},
		Context:  ctx,
		cancel:   cancel,
	}
	cbChannel := make(chan bool)
	callback := testCallback{
		called: cbChannel,
	}
	rule := dummyRule{
		satisfiedResponse: true,
	}
	rw := v3RuleWork{
		rule:             &rule,
		ruleTask:         task,
		ruleTaskCallback: callback.callback,
		lockKey:          "key",
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
