package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type testKeyProcessor struct {
	apis    []readAPI
	keys    []string
	loggers []*zap.Logger
	values  []*string
	// tracks the number of times a rule is processed in a single run
	rulesProcessedCount map[string]int
	ruleIDs             map[int]string
	timesEvalFunc       func(ruleID string)
}

func newTestKeyProcessor() testKeyProcessor {
	return testKeyProcessor{
		keys:          []string{},
		ruleIDs:       make(map[int]string, 0),
		timesEvalFunc: noOpTimesEval,
	}
}
func (tkp *testKeyProcessor) processKey(key string,
	value *string,
	api readAPI,
	logger *zap.Logger,
	metadata map[string]string) {
	tkp.keys = append(tkp.keys, key)
	tkp.values = append(tkp.values, value)
	tkp.apis = append(tkp.apis, api)
	tkp.loggers = append(tkp.loggers, logger)
	tkp.timesEvalFunc(key)
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
	kp := v3KeyProcessor{
		baseKeyProcessor: baseKeyProcessor{
			contextProviders: contextProviders,
			rm:               &rm,
			lockKeyPatterns:  lockKeyPatterns,
			ruleIDs:          ruleIDs,
			timesEvalFunc:    noOpTimesEval,
		},
		callbacks: callbacks,
		channel:   channel,
	}
	logger := getTestLogger()
	go kp.processKey("/test/key", &value, api, logger, map[string]string{})
	work := <-channel
	assert.Equal(t, "/test/lock/key", work.lockKey)
}
