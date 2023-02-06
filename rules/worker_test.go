package rules

import (
	"testing"

	"github.com/IBM-Cloud/go-etcd-rules/rules/lock"
	"github.com/stretchr/testify/assert"
	v3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

func TestWorkerSingleRun(t *testing.T) {
	conf := v3.Config{
		Endpoints: []string{""},
	}
	lgr, err := zap.NewDevelopment()
	assert.NoError(t, err)
	metrics := NewMockMetricsCollector()
	metrics.SetLogger(lgr)
	cl, err := v3.New(conf)
	assert.NoError(t, err)
	e := newV3Engine(getTestLogger(), cl, EngineLockTimeout(300))
	channel := e.workChannel
	lockChannel := make(chan bool)
	locker := lock.MockLocker{
		Channel: lockChannel,
	}
	api := mapReadAPI{}
	w := v3Worker{
		baseWorker: baseWorker{
			api:      &api,
			locker:   &locker,
			workerID: "testworker",
			metrics:  &metrics,
		},
		engine: &e,
	}
	attrMap := map[string]string{}
	attr := mapAttributes{
		values: attrMap,
	}
	ctx, cancel := context.WithCancel(context.Background())
	ctx = SetMethod(ctx, "workerTest")
	task := V3RuleTask{
		Attr:     &attr,
		Logger:   getTestLogger(),
		Metadata: map[string]string{},
		Context:  ctx,
		cancel:   cancel,
	}
	cbChannel := make(chan bool)
	callback := testCallback{
		called: cbChannel,
	}

	// Test case: happy path
	rule := dummyRule{
		satisfiedResponse: true,
		attr:              &attr,
	}
	rw := v3RuleWork{
		rule:             &rule,
		ruleTask:         task,
		ruleTaskCallback: callback.callback,
		lockKey:          "key",
		metricsInfo:      newMetricsInfo(ctx, "/test/item"),
	}
	expectedMethodNames := []string{"workerTest"}

	go w.singleRun()
	channel <- rw
	assert.True(t, <-cbChannel)
	assert.True(t, <-lockChannel)
	assert.Equal(t, expectedMethodNames, metrics.WorkerQueueWaitTimeMethod)
	assert.NotEmpty(t, metrics.WorkerQueueWaitTime)

	// Test case: rule is satisfied but there is an error obtaining the lock
	errorMsg := "Some error"
	locker.ErrorMsg = &errorMsg

	callChannel := make(chan bool)

	go channelWriteAfterCall(callChannel, w.singleRun)
	channel <- rw
	assert.True(t, <-callChannel)
	assert.Equal(t, 0, len(cbChannel))
	assert.Equal(t, 0, len(lockChannel))
	// not expecting these to change from the first run at all because the rule doesn't make it
	// all the way through
	assert.Equal(t, expectedMethodNames, metrics.WorkerQueueWaitTimeMethod)
	assert.NotEmpty(t, metrics.WorkerQueueWaitTime)

	// Test case: the rule is immediately not satisfied
	rule = dummyRule{
		satisfiedResponse: false,
	}
	expectedSatisfiedMetricsPatterns := []string{"/test/item"}
	expectedSatisfiedMetricsPhase := []string{"worker.doWorkBeforeLock"}
	go channelWriteAfterCall(callChannel, w.singleRun)
	channel <- rw
	assert.True(t, <-callChannel)
	assert.Equal(t, 0, len(cbChannel))
	assert.Equal(t, 0, len(lockChannel))

	assert.Equal(t, expectedSatisfiedMetricsPatterns, metrics.IncSatisfiedThenNotPattern)
	assert.Equal(t, expectedSatisfiedMetricsPhase, metrics.IncIncSatisfiedThenNotPhase)

	// not expecting these to change from the first run at all because the rule doesn't make it
	// all the way through
	assert.Equal(t, expectedMethodNames, metrics.WorkerQueueWaitTimeMethod)
	assert.NotEmpty(t, metrics.WorkerQueueWaitTime)
}
