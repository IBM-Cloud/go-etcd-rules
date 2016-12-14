package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRuleOptions(t *testing.T) {
	opts := makeRuleOptions()
	var defaultLockTimeout uint64
	assert.Equal(t, defaultLockTimeout, opts.lockTimeout)
	opts = makeRuleOptions(RuleLockTimeout(300))
	var threeHundred uint64 = 300
	assert.Equal(t, threeHundred, opts.lockTimeout)
}
