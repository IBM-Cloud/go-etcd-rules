package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRuleManager(t *testing.T) {
	rm := newRuleManager()
	rule1, err1 := NewEqualsLiteralRule("/this/is/:a/rule", nil)
	assert.NoError(t, err1)
	rm.addRule(rule1)
	rule2, err2 := NewEqualsLiteralRule("/that/is/:a/nother", nil)
	assert.NoError(t, err2)
	rm.addRule(rule2)
	rule3, err3 := NewEqualsLiteralRule("/this/is/:a", nil)
	assert.NoError(t, err3)
	rm.addRule(rule3)
	rules := rm.getStaticRules("/this/is/a/rule", nil)
	assert.Equal(t, 1, len(rules))
	for r, index := range rules {
		assert.Equal(t, 0, index)
		assert.True(t, r.satisfiable("/this/is/a/rule", nil))
	}
	rules = rm.getStaticRules("/nothing", nil)
	assert.Equal(t, 0, len(rules))
}
