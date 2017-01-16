package rules

import (
	"testing"

	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/clientv3"
	"github.com/stretchr/testify/assert"
	"github.com/uber-go/zap"
)

func TestKeyProcessor(t *testing.T) {
	value := "value"
	rule, err := NewEqualsLiteralRule("/test/:key", &value)
	assert.NoError(t, err)
	rm := newRuleManager()
	rm.addRule(rule)
	api := newMapReadAPI()
	api.put("/test/key", value)
	callbacks := map[int]RuleTaskCallback{0: dummyCallback}
	lockKeyPatterns := map[int]string{0: "/test/lock/:key"}
	channel := make(chan ruleWork)
	kp := keyProcessor{
		baseKeyProcessor: baseKeyProcessor{
			rm:              &rm,
			lockKeyPatterns: lockKeyPatterns,
		},
		callbacks: callbacks,
		channel:   channel,
		config:    client.Config{},
	}
	logger := getTestLogger()
	go kp.processKey("/test/key", &value, api, logger)
	work := <-channel
	assert.Equal(t, "/test/lock/key", work.lockKey)
}

type testKeyProcessor struct {
	apis    []readAPI
	keys    []string
	loggers []zap.Logger
	values  []*string
}

func (tkp *testKeyProcessor) processKey(key string,
	value *string,
	api readAPI,
	logger zap.Logger) {
	tkp.keys = append(tkp.keys, key)
	tkp.values = append(tkp.values, value)
	tkp.apis = append(tkp.apis, api)
	tkp.loggers = append(tkp.loggers, logger)
}

func getTestLogger() zap.Logger {
	return zap.New(zap.NewTextEncoder())
}

func TestV3KeyProcessor(t *testing.T) {
	value := "value"
	rule, err := NewEqualsLiteralRule("/test/:key", &value)
	assert.NoError(t, err)
	rm := newRuleManager()
	rm.addRule(rule)
	api := newMapReadAPI()
	api.put("/test/key", value)
	callbacks := map[int]V3RuleTaskCallback{0: v3DummyCallback}
	lockKeyPatterns := map[int]string{0: "/test/lock/:key"}
	channel := make(chan v3RuleWork)
	kp := v3KeyProcessor{
		baseKeyProcessor: baseKeyProcessor{
			rm:              &rm,
			lockKeyPatterns: lockKeyPatterns,
		},
		callbacks: callbacks,
		channel:   channel,
		config:    &clientv3.Config{},
	}
	logger := getTestLogger()
	go kp.processKey("/test/key", &value, api, logger)
	work := <-channel
	assert.Equal(t, "/test/lock/key", work.lockKey)
}
