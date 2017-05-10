package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRuleOptions(t *testing.T) {
	opts := makeRuleOptions()
	var defaultLockTimeout int
	assert.Equal(t, defaultLockTimeout, opts.lockTimeout)
	opts = makeRuleOptions(RuleLockTimeout(300))
	var threeHundred = 300
	assert.Equal(t, threeHundred, opts.lockTimeout)
}

func TestEngineOptions(t *testing.T) {
	opts := makeEngineOptions(EngineSyncInterval(5))
	assert.Equal(t, 5, opts.syncInterval)
	assert.Equal(t, 1, opts.syncDelay)
	opts = makeEngineOptions(EngineConcurrency(10))
	assert.Equal(t, 10, opts.concurrency)
	keyExp1 := KeyExpansion(map[string][]string{"key1": []string{"val1"}, "key2": []string{"val2"}})
	keyExp2 := KeyExpansion(map[string][]string{"key2": []string{"val3"}, "key3": []string{"val4"}})
	opts = makeEngineOptions(keyExp1, keyExp2)
	assert.Equal(t, map[string][]string{"key1": []string{"val1"}, "key2": []string{"val3"}, "key3": []string{"val4"}}, opts.keyExpansion)
	opts = makeEngineOptions(EngineSyncDelay(10))
	assert.Equal(t, 10, opts.syncDelay)
}
