package rules

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type testKeyProcessor struct {
	apis          []readAPI
	keys          []string
	loggers       []*zap.Logger
	values        []*string
	ruleIDs       map[int]string
	timesEvalFunc func(ruleID string)
}

func newTestKeyProcessor() testKeyProcessor {
	return testKeyProcessor{
		keys:    []string{},
		ruleIDs: make(map[int]string),
	}
}
func (tkp *testKeyProcessor) processKey(key string,
	value *string,
	api readAPI,
	logger *zap.Logger,
	metadata map[string]string,
	timesEvaluated func(rulesID string)) {
	tkp.keys = append(tkp.keys, key)
	tkp.values = append(tkp.values, value)
	tkp.apis = append(tkp.apis, api)
	tkp.loggers = append(tkp.loggers, logger)
	if timesEvaluated != nil {
		timesEvaluated(key)
	}
}

func (tkp *testKeyProcessor) setTimesEvalFunc(timeEvalFunc func(ruleID string)) {
	tkp.timesEvalFunc = timeEvalFunc
}

func getTestLogger() *zap.Logger {
	logger, _ := zap.NewDevelopment()
	return logger
}

func TestV3KeyProcessor(t *testing.T) {
	value := "value"
	rule, err := NewEqualsLiteralRule("/test/:key", &value)
	assert.NoError(t, err)
	rm := newRuleManager(map[string]constraint{}, false)
	rm.addRule(rule)
	api := newMapReadAPI()
	api.put("/test/key", value)
	callbacks := map[int]V3RuleTaskCallback{0: v3DummyCallback}
	contextProviders := map[int]ContextProvider{0: defaultContextProvider}
	lockKeyPatterns := map[int]string{0: "/test/lock/:key"}
	ruleIDs := map[int]string{0: "testKey"}
	channel := make(chan v3RuleWork)
	kpChannel := make(chan *keyTask)
	kp := v3KeyProcessor{
		baseKeyProcessor: baseKeyProcessor{
			contextProviders: contextProviders,
			rm:               &rm,
			lockKeyPatterns:  lockKeyPatterns,
			ruleIDs:          ruleIDs,
		},
		callbacks: callbacks,
		channel:   channel,
		kpChannel: kpChannel,
	}
	logger := getTestLogger()
	go kp.keyWorker(logger)
	go kp.processKey("/test/key", &value, api, logger, map[string]string{}, nil)
	work := <-channel
	assert.Equal(t, "/test/lock/key", work.lockKey)
}

func TestNewV3KeyProcessor(t *testing.T) {
	value := "value"
	rule, err := NewEqualsLiteralRule("/test/:key", &value)
	assert.NoError(t, err)
	rm := newRuleManager(map[string]constraint{}, false)
	rm.addRule(rule)
	api := newMapReadAPI()
	api.put("/test/key", value)

	channel := make(chan v3RuleWork)
	kpChannel := make(chan *keyTask, 1000)
	logger := getTestLogger()
	kp := newV3KeyProcessor(channel, &rm, kpChannel, 1, 1, logger)
	kp.setCallback(0, V3RuleTaskCallback(v3DummyCallback))
	kp.setContextProvider(0, defaultContextProvider)
	kp.setRuleID(0, "testKey")
	kp.setLockKeyPattern(0, "/test/lock/:key")
	go kp.processKey("/test/key", &value, api, logger, map[string]string{}, nil)
	time.Sleep(time.Second)
	work := <-channel
	assert.Equal(t, "/test/lock/key", work.lockKey)
}
