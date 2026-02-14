package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var defaultOpts = ruleMgrRuleOptions{priority: 0, crawlerOnly: false}

func TestRuleManager(t *testing.T) {
	for _, erf := range []bool{true, false} {
		rm := newRuleManager(map[string]constraint{}, erf)
		rule1, err1 := NewEqualsLiteralRule("/this/is/:a/rule", nil)
		assert.NoError(t, err1)
		opts := makeRuleOptions()
		rm.addRule(rule1, opts)
		rule2, err2 := NewEqualsLiteralRule("/that/is/:a/nother", nil)
		assert.NoError(t, err2)
		rm.addRule(rule2, opts)
		rule3, err3 := NewEqualsLiteralRule("/this/is/:a", nil)
		assert.NoError(t, err3)
		rm.addRule(rule3, opts)
		rules, _ := rm.getStaticRules("/this/is/a/rule", nil)
		assert.Equal(t, 1, len(rules))
		for r, index := range rules {
			assert.Equal(t, 0, index)
			assert.True(t, r.satisfiable("/this/is/a/rule", nil))
		}
		rules, _ = rm.getStaticRules("/nothing", nil)
		assert.Equal(t, 0, len(rules))
	}
}

func TestReducePrefixes(t *testing.T) {
	prefixes := map[string]ruleMgrRuleOptions{"/servers/internal/states": defaultOpts, "/servers/internal": {priority: 10, crawlerOnly: true}, "/servers": {priority: 0, crawlerOnly: true}}
	prefixes = reducePrefixes(prefixes)
	assert.Equal(t, 1, len(prefixes))
	assert.Equal(t, ruleMgrRuleOptions{priority: 10, crawlerOnly: false}, prefixes["/servers"])
}

func TestSortPrefixesByLength(t *testing.T) {
	prefixes := map[string]ruleMgrRuleOptions{"/servers/internal": defaultOpts, "/servers/internal/states": defaultOpts, "/servers": defaultOpts}
	sorted := sortPrefixesByLength(prefixes)
	assert.Equal(t, "/servers/internal/states", sorted[2])
	assert.Equal(t, "/servers/internal", sorted[1])
	assert.Equal(t, "/servers", sorted[0])
}

func TestCombineRuleData(t *testing.T) {
	testCases := []struct {
		sourceData   [][]string
		expectedData []string
	}{
		{
			[][]string{{"/a/b/c", "/x/y/z"}, {"/a/b/c"}},
			[]string{"/a/b/c", "/x/y/z"},
		},
	}
	dummyRule := NewAndRule()
	for idx, testCase := range testCases {
		ruleIndex := 0
		source := func(_ DynamicRule) []string {
			out := testCase.sourceData[ruleIndex]
			ruleIndex++
			return out
		}
		rules := []DynamicRule{}
		for i := 0; i < len(testCase.sourceData); i++ {
			rules = append(rules, dummyRule)
		}
		compareUnorderedStringArrays(t, testCase.expectedData, combineRuleData(rules, source), "index %d", idx)
	}
}

func TestGetPrioritizedPrefixes(t *testing.T) {
	rm := newRuleManager(map[string]constraint{}, false)

	// Final priority - 300
	// shorter prefix rule, with lower priority than later overlapping rule
	rule1, err1 := NewEqualsLiteralRule("/this/is/:a/rule", nil)
	assert.NoError(t, err1)
	rm.addRule(rule1, makeRuleOptions(Priority(100)))

	// overlapping, longer prefix with a higher priority
	rule2, err2 := NewEqualsLiteralRule("/this/is/overlapping/:a", nil)
	assert.NoError(t, err2)
	rm.addRule(rule2, makeRuleOptions(Priority(300)))

	// Final priority - 200
	rule3, err3 := NewEqualsLiteralRule("/these/are/:a/ruleset", nil)
	assert.NoError(t, err3)
	rm.addRule(rule3, makeRuleOptions(Priority(200)))

	// same prefix as earlier, largest one should be considered
	rule4, err4 := NewEqualsLiteralRule("/these/are/:a", nil)
	assert.NoError(t, err4)
	rm.addRule(rule4, makeRuleOptions(Priority(50)))

	// Final priority - 0
	// no priority, should be last
	rule5, err5 := NewEqualsLiteralRule("/that/is/:a/nother", nil)
	assert.NoError(t, err5)
	rm.addRule(rule5, makeRuleOptions())

	// Final priority - 100
	// third tier priority rule
	rule6, err6 := NewEqualsLiteralRule("/this/one/is/:a", nil)
	assert.NoError(t, err6)
	rm.addRule(rule6, makeRuleOptions(Priority(100)))

	assert.Equal(t, []string{"/this/is/", "/these/are/", "/this/one/is/", "/that/is/"}, rm.getPrioritizedPrefixes())
}

func TestAddRuleCrawlerOnly(t *testing.T) {
	rm := newRuleManager(map[string]constraint{}, false)

	rule1, err1 := NewEqualsLiteralRule("/this/is/:a/rule", nil)
	assert.NoError(t, err1)
	rm.addRule(rule1, makeRuleOptions(Priority(100), CrawlerOnly()))

	// overlapping, longer prefix with a higher priority
	rule2, err2 := NewEqualsLiteralRule("/this/is/overlapping/:a", nil)
	assert.NoError(t, err2)
	rm.addRule(rule2, makeRuleOptions(Priority(300), CrawlerOnly()))

	assert.True(t, assert.ObjectsAreEqual(map[string]ruleMgrRuleOptions{"/this/is/": {priority: 300, crawlerOnly: true}}, rm.prefixes))
}

func TestGetStaticRules(t *testing.T) {
	rm := newRuleManager(map[string]constraint{}, false)

	rule1, err1 := NewEqualsLiteralRule("/this/is/:a/rule", nil)
	assert.NoError(t, err1)
	rm.addRule(rule1, makeRuleOptions(Priority(100)))

	rule2, err2 := NewEqualsLiteralRule("/this/is/overlapping/:a", nil)
	assert.NoError(t, err2)
	rm.addRule(rule2, makeRuleOptions(Priority(300)))

	rule3, err3 := NewEqualsLiteralRule("/these/are/:a/ruleset", nil)
	assert.NoError(t, err3)
	rm.addRule(rule3, makeRuleOptions(Priority(50)))

	rule4, err4 := NewEqualsLiteralRule("/these/are/:a/ruleset", nil)
	assert.NoError(t, err4)
	rm.addRule(rule4, makeRuleOptions(Priority(200)))

	rule5, err5 := NewEqualsLiteralRule("/that/is/:a/nother", nil)
	assert.NoError(t, err5)
	rm.addRule(rule5, makeRuleOptions())

	// The 200 priority rule, 3 rule added, should be first in
	// the priority slice
	index, priority := rm.getStaticRules("/these/are/a/ruleset", nil)
	assert.Equal(t, 3, index[priority[0]])
	assert.Equal(t, 2, index[priority[1]])
}
