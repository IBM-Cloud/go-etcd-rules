package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

func TestRuleOptions(t *testing.T) {
	opts := makeRuleOptions()
	assert.Equal(t, defaultRuleID, opts.ruleID)
	var defaultLockTimeout int
	assert.Equal(t, defaultLockTimeout, opts.lockTimeout)
	opts = makeRuleOptions(RuleLockTimeout(300))
	var threeHundred = 300
	assert.Equal(t, threeHundred, opts.lockTimeout)
	opts = makeRuleOptions(RuleContextProvider(getTestContextProvider()))
	verifyTestContextProvider(t, opts.contextProvider)
	testRuleID := "super-awesome-rule-id"
	opts = makeRuleOptions(RuleID(testRuleID))
	assert.Equal(t, testRuleID, opts.ruleID)
}

func TestEngineOptions(t *testing.T) {
	var noOp noOpMetricsCollector
	opts := makeEngineOptions(EngineSyncInterval(5))
	assert.Equal(t, 5, opts.syncInterval)
	assert.Equal(t, 1, opts.syncDelay)
	assert.IsType(t, &noOp, opts.metrics())
	opts = makeEngineOptions(EngineConcurrency(10))
	assert.Equal(t, 10, opts.concurrency)
	keyExp1 := KeyExpansion(map[string][]string{"key1": {"val1"}, "key2": {"val2"}})
	keyExp2 := KeyExpansion(map[string][]string{"key2": {"val3"}, "key3": {"val4"}})
	opts = makeEngineOptions(keyExp1, keyExp2)
	assert.Equal(t, map[string][]string{"key1": {"val1"}, "key2": {"val3"}, "key3": {"val4"}}, opts.keyExpansion)
	opts = makeEngineOptions(EngineSyncDelay(10))
	assert.Equal(t, 10, opts.syncDelay)
	opts = makeEngineOptions(EngineWatchTimeout(3))
	assert.Equal(t, 3, opts.watchTimeout)
	opts = makeEngineOptions(KeyConstraint("clusterid", "/:clusterid/", [][]rune{{'a', 'b'}}))
	assert.Equal(t, constraint{chars: [][]rune{{'a', 'b'}}, prefix: "/:clusterid/"}, opts.constraints["clusterid"])
	cp := getTestContextProvider()
	opts = makeEngineOptions(EngineContextProvider(cp))
	verifyTestContextProvider(t, opts.contextProvider)
	opts = makeEngineOptions(EngineCrawlMutex("mutex", 23))
	if assert.NotNil(t, opts.crawlMutex) {
		assert.Equal(t, "mutex", *opts.crawlMutex)
	}
	assert.Equal(t, 23, opts.crawlerTTL)
	assert.Equal(t, 0, opts.ruleWorkBuffer)
	opts = makeEngineOptions(EngineRuleWorkBuffer(10))
	assert.Equal(t, 10, opts.ruleWorkBuffer)
	mm := NewMockMetricsCollector()
	mFunc := func() MetricsCollector {
		return &mm
	}
	opts = makeEngineOptions(EngineMetricsCollector(mFunc))
	assert.IsType(t, &mm, opts.metrics())
}

var contextKeyTest = contextKey("test")

func getTestContextProvider() ContextProvider {
	return func() (context.Context, context.CancelFunc) {
		ctx, cancel := context.WithCancel(context.Background())
		return context.WithValue(ctx, contextKeyTest, "value"), cancel
	}
}

func verifyTestContextProvider(t *testing.T, cp ContextProvider) {
	ctx, _ := cp()
	val := ctx.Value(contextKeyTest)
	if assert.NotNil(t, val) {
		text, ok := val.(string)
		if assert.True(t, ok) {
			assert.Equal(t, "value", text)
		}
	}
}
