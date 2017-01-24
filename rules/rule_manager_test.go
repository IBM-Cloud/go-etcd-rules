package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRuleManager(t *testing.T) {
	rm := newRuleManager(map[string]constraint{})
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

func TestReducePrefixes(t *testing.T) {
	prefixes := map[string]string{"/servers/internal/states": "", "/servers/internal": "", "/servers": ""}
	prefixes = reducePrefixes(prefixes)
	assert.Equal(t, 1, len(prefixes))
	assert.Equal(t, "", prefixes["/servers"])

}

func TestSortPrefixesByLength(t *testing.T) {
	prefixes := map[string]string{"/servers/internal": "", "/servers/internal/states": "", "/servers": ""}
	sorted := sortPrefixesByLength(prefixes)
	assert.Equal(t, "/servers/internal/states", sorted[2])
	assert.Equal(t, "/servers/internal", sorted[1])
	assert.Equal(t, "/servers", sorted[0])
}
