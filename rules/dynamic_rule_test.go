package rules

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	expansionMap        = map[string][]string{"a": {"first", "second"}, "b": {"third", "fourth"}, "c": {"x", "y"}}
	expansionPatterns1  = []string{"/first/third/a/attr1", "/first/fourth/b/attr1", "/second/third/c/attr1", "/second/fourth/d/attr1"}
	expansionPatterns2  = []string{"/first/third/a/attr2", "/first/fourth/b/attr2", "/second/third/c/attr2", "/second/fourth/d/attr2"}
	expansionAttributes = []map[string]string{{"a": "first", "b": "third"}, {"a": "first", "b": "fourth"}, {"a": "second", "b": "third"}, {"a": "second", "b": "fourth"}}
)

type dummyRuleTrueFactory struct {
}

func (drtf *dummyRuleTrueFactory) newRule(keys []string) staticRule {
	return &dummyRule{
		satisfiableResponse: true,
		satisfiedResponse:   true,
	}
}

func (a attributeInstance) String() string {
	value := "<nil>"
	if a.value != nil {
		value = *a.value
	}
	return fmt.Sprintf("key: %s value: %s", a.key, value)
}

func TestEqualsLiteralRule(t *testing.T) {
	r, err := NewEqualsLiteralRule("/:region/actual/clusters/:clusterid/workers/:workerid", nil)
	assert.NoError(t, err)
	rule, _, _ := r.makeStaticRule("/us-south/actual/clusters/armada-9b93c18d/workers/worker3", nil)
	assert.True(t, rule.satisfiable("/us-south/actual/clusters/armada-9b93c18d/workers/worker3", nil))
	_, _, ok := r.makeStaticRule("/us-south/desired/clusters/armada-9b93c18d/workers/worker3", nil)
	assert.False(t, ok)
	_, err = NewEqualsLiteralRule("/:region/actual/clusters/:clusterid/[workers/:workerid", nil)
	assert.Error(t, err)
	assert.Equal(t, "/:region/actual/clusters/:clusterid/workers/:workerid", r.getPatterns()[0])
	r, err = NewEqualsLiteralRule("/:a/:b/:var/attr1", nil)
	assert.NoError(t, err)
	expanded, exp := r.expand(expansionMap)
	assert.True(t, exp)
	assert.Equal(t, 4, len(expanded))

	staticRuleOks := []bool{false, false, false, false}

	for i, pattern := range expansionPatterns1 {
		for _, expandedRule := range expanded {
			_, attr, ok := expandedRule.makeStaticRule(pattern, nil)
			staticRuleOks[i] = staticRuleOks[i] || ok
			if ok {
				for key, value := range expansionAttributes[i] {
					attrValue := attr.GetAttribute(key)
					assert.NotNil(t, attrValue)
					if attrValue != nil {
						assert.Equal(t, value, *attrValue)
					}
				}
			}
		}
	}
	for i, staticRuleOk := range staticRuleOks {
		assert.True(t, staticRuleOk, "%s pattern did not match", expansionPatterns1[i])
	}
	val := "val"
	simple, _ := NewEqualsLiteralRule("/testpolling/:value", &val)
	prefixes := simple.getPrefixes()
	assert.Equal(t, len(prefixes), 1)
	assert.Equal(t, "/testpolling/", prefixes[0])
}

func TestAndRule(t *testing.T) {
	api := newMapReadAPI()

	deployed := "deployed"
	api.put("/us-south/desired/clusters/armada-9b93c18d/workers/worker3/state", "deployed")
	workerDesiredStateDeployed, _ := NewEqualsLiteralRule("/:region/desired/clusters/:clusterid/workers/:workerid/state", &deployed)
	workerPathMissing, _ := NewEqualsLiteralRule("/:region/actual/clusters/:clusterid/workers/:workerid", nil)
	a1 := NewAndRule(workerDesiredStateDeployed, workerPathMissing)
	s1, attr, ok := a1.makeStaticRule("/us-south/desired/clusters/armada-9b93c18d/workers/worker3/state", &deployed)
	assert.True(t, ok)
	var sat bool
	sat, _ = s1.satisfied(api)
	assert.True(t, sat)
	s3, _ := a1.staticRuleFromAttributes(attr)
	sat, _ = s3.satisfied(api)
	assert.True(t, sat)
	assert.Equal(t, "/:region/desired/clusters/:clusterid/workers/:workerid/state", a1.getPatterns()[0])
	assert.Equal(t, "/:region/actual/clusters/:clusterid/workers/:workerid", a1.getPatterns()[1])
	assert.Equal(t, 2, len(a1.getPatterns()))
	assert.Equal(t, "/", a1.getPrefixes()[0])
	assert.Equal(t, "/", a1.getPrefixes()[1])
	assert.Equal(t, 2, len(a1.getPrefixes()))

	api.put("/us-south/actual/clusters/armada-9b93c18d/workers/worker3", "<dir>")
	sat, _ = s1.satisfied(api)
	assert.False(t, sat)
	_, _, ok = a1.makeStaticRule("/us-south/desired/clusters/armada-9b93c18d/workers/worker3/state", nil)
	assert.True(t, ok)

	e1, _ := NewEqualsLiteralRule("/:a/:b/:var/attr1", nil)
	e2, _ := NewEqualsLiteralRule("/:a/:b/:var/attr2", nil)

	eAnd := NewAndRule([]DynamicRule{e1, e2}...)
	expanded, exp := eAnd.expand(expansionMap)
	assert.True(t, exp)
	assert.Equal(t, 4, len(expanded))

	staticRuleOks := []bool{false, false, false, false}

	for i, pattern := range expansionPatterns1 {
		for _, expandedRule := range expanded {
			_, attr1, ok := expandedRule.makeStaticRule(pattern, nil)
			staticRuleOks[i] = staticRuleOks[i] || ok
			if ok {
				for key, value := range expansionAttributes[i] {
					attrValue := attr1.GetAttribute(key)
					assert.NotNil(t, attrValue)
					if attrValue != nil {
						assert.Equal(t, value, *attrValue)
					}
				}
				assert.Equal(t, pattern, attr1.Format("/:a/:b/:var/attr1"))
			}
		}
	}
	for i, staticRuleOk := range staticRuleOks {
		assert.True(t, staticRuleOk, "%s pattern did not match", expansionPatterns1[i])
	}

}

func TestOrRule(t *testing.T) {
	api := newMapReadAPI()

	deployed := "deployed"
	api.put("/us-south/desired/clusters/armada-9b93c18d/workers/worker3/state", "deployed")
	workerDesiredStateDeployed, _ := NewEqualsLiteralRule("/:region/desired/clusters/:clusterid/workers/:workerid/state", &deployed)
	workerPathMissing, _ := NewEqualsLiteralRule("/:region/actual/clusters/:clusterid/workers/:workerid", nil)
	o1 := NewOrRule(workerDesiredStateDeployed, workerPathMissing)
	s1, attr, ok := o1.makeStaticRule("/us-south/desired/clusters/armada-9b93c18d/workers/worker3/state", &deployed)
	assert.True(t, ok)
	var sat bool
	sat, _ = s1.satisfied(api)
	assert.True(t, sat)
	s3, _ := o1.staticRuleFromAttributes(attr)
	sat, _ = s3.satisfied(api)
	assert.True(t, sat)

	api.put("/us-south/actual/clusters/armada-9b93c18d/workers/worker3", "<dir>")
	sat, _ = s1.satisfied(api)
	assert.True(t, sat)
	_, _, ok = o1.makeStaticRule("/us-south/desired/clusters/armada-9b93c18d/workers/worker3/state", nil)
	assert.True(t, ok)
	o2 := NewOrRule(workerPathMissing)
	s2, _, _ := o2.makeStaticRule("/us-south/actual/clusters/armada-9b93c18d/workers/worker3", nil)
	sat, _ = s2.satisfied(api)
	assert.False(t, sat)
}

func TestNotRule(t *testing.T) {
	workerPathMissing, _ := NewEqualsLiteralRule("/:region/actual/clusters/:clusterid/workers/:workerid", nil)
	test := NewNotRule(workerPathMissing)
	value := "value"
	notRule, attr, ok := test.makeStaticRule("/us-south/actual/clusters/armada-9b93c18d/workers/worker3", &value)
	assert.True(t, ok)
	assert.True(t, notRule.satisfiable("/us-south/actual/clusters/armada-9b93c18d/workers/worker3", &value))
	api := newMapReadAPI()
	var sat bool
	sat, _ = notRule.satisfied(api)
	assert.False(t, sat)
	api.put("/us-south/actual/clusters/armada-9b93c18d/workers/worker3", "<dir>")
	sat, _ = notRule.satisfied(api)
	assert.True(t, sat)

	notRule, _ = test.staticRuleFromAttributes(attr)
	sat, _ = notRule.satisfied(api)
	assert.True(t, sat)
	assert.Equal(t, "/:region/actual/clusters/:clusterid/workers/:workerid", test.getPatterns()[0])
	assert.Equal(t, "/", test.getPrefixes()[0])
}

func TestEqualsRule(t *testing.T) {
	test, err := NewEqualsRule([]string{
		"/:region/desired/clusters/:clusterid/workers/:workerid/state",
		"/:region/actual/clusters/:clusterid/workers/:workerid/state",
	})
	assert.NoError(t, err)
	api := newMapReadAPI()
	actual, _, ok := test.makeStaticRule("/us-south/actual/clusters/armada-9b93c18d/workers/worker3/state", nil)
	assert.True(t, ok)
	var sat bool
	sat, _ = actual.satisfied(api)
	assert.True(t, sat)
	api.put("/us-south/desired/clusters/armada-9b93c18d/workers/worker3/state", "deployed")
	sat, _ = actual.satisfied(api)
	assert.False(t, sat)
	api.put("/us-south/actual/clusters/armada-9b93c18d/workers/worker3/state", "deployed")
	sat, _ = actual.satisfied(api)
	assert.True(t, sat)
}

func TestRuleCombinations(t *testing.T) {
	nilRule, err := NewEqualsLiteralRule("/:id/prop1", nil)
	if !assert.NoError(t, err) {
		return
	}
	andRule := NewAndRule(nilRule)
	notRule := NewNotRule(andRule)
	api := newMapReadAPI()
	api.put("/id/prop1", "value")
	value := "value"
	stat, _, ok := notRule.makeStaticRule("/id/prop1", &value)
	if !assert.True(t, ok) {
		return
	}
	sat, err := stat.satisfied(api)
	if !assert.NoError(t, err) {
		return
	}
	assert.True(t, sat)
}

func TestRulePrint(t *testing.T) {
	rules := []DynamicRule{}
	testCases := []struct {
		get    func() DynamicRule
		getErr func() (DynamicRule, error)
		expect string
	}{
		{
			nil,
			func() (DynamicRule, error) { return NewEqualsLiteralRule("/:region/test", nil) },
			"/:region/test = <nil>",
		},
		{
			nil,
			func() (DynamicRule, error) { return NewEqualsLiteralRule("/:region/test2", sTP("value")) },
			"/:region/test2 = \"value\"",
		},
		{
			func() DynamicRule { return NewAndRule(rules[0], rules[1]) },
			nil,
			"(/:region/test = <nil> AND /:region/test2 = \"value\")",
		},
		{
			func() DynamicRule { return NewOrRule(rules[0], rules[1]) },
			nil,
			"(/:region/test = <nil> OR /:region/test2 = \"value\")",
		},
		{
			func() DynamicRule { return NewOrRule(rules[2], rules[3]) },
			nil,
			"((/:region/test = <nil> AND /:region/test2 = \"value\") OR (/:region/test = <nil> OR /:region/test2 = \"value\"))",
		},
		{
			func() DynamicRule { return NewNotRule(rules[4]) },
			nil,
			"NOT (((/:region/test = <nil> AND /:region/test2 = \"value\") OR (/:region/test = <nil> OR /:region/test2 = \"value\")))",
		},
		{
			nil,
			func() (DynamicRule, error) { return NewEqualsRule([]string{"/:region/test", "/:region/test2"}) },
			"/:region/test = /:region/test2",
		},
	}
	for idx, testCase := range testCases {
		var dr DynamicRule
		if testCase.get != nil {
			dr = testCase.get()
		}
		if testCase.getErr != nil {
			var err error
			dr, err = testCase.getErr()
			assert.NoError(t, err, "index %d", idx)
		}
		rules = append(rules, dr)
		assert.Equal(t, testCase.expect, fmt.Sprintf("%s", dr), "index %d", idx)
	}
}
func TestFormatRuleString(t *testing.T) {
	assert.Equal(
		t,
		"(\n    (\n        /:region/test = <nil> AND /:region/test2 = \"value\"\n    ) OR (\n        /:region/test = <nil> OR /:region/test2 = \"value\"\n    )\n)",
		FormatRuleString("((/:region/test = <nil> AND /:region/test2 = \"value\") OR (/:region/test = <nil> OR /:region/test2 = \"value\"))"),
	)
}

func TestRuleSatisfied(t *testing.T) {
	rules := []DynamicRule{}
	testCases := []struct {
		get            func() DynamicRule
		getErr         func() (DynamicRule, error)
		key            string
		value          *string
		satisfied, err bool
		kvs            map[string]string
	}{
		{
			nil,
			func() (DynamicRule, error) {
				return NewEqualsLiteralRule("/emea/branch/parent/:parentid/child/:childid/attributes/:attr/value", sTP("home"))
			},
			"/emea/branch/parent/fef460923d2248bf99da87f8d4b1c363/child/child-home-fef460923d2248bf99da87f8d4b1c363-c1/attributes/location/value",
			sTP("home"),
			true,
			false,
			map[string]string{
				"/emea/branch/parent/fef460923d2248bf99da87f8d4b1c363/child/child-home-fef460923d2248bf99da87f8d4b1c363-c1/attributes/location/value": "home",
			},
		},
		{
			nil,
			func() (DynamicRule, error) {
				return NewEqualsLiteralRule("/updater/emea/child/:attr/enabled", sTP("true"))
			},
			"/updater/emea/child/reading/enabled",
			sTP("true"),
			true,
			false,
			map[string]string{
				"/updater/emea/child/reading/enabled": "true",
			},
		},
		// This rule is not satisfied, because the trigger key does not contain all the
		// attributes necessary to evaluate the rule
		{
			func() DynamicRule { return NewAndRule(rules[0], rules[1]) },
			nil,
			"/updater/emea/child/location/enabled",
			sTP("true"),
			false,
			true,
			map[string]string{
				"/emea/branch/parent/fef460923d2248bf99da87f8d4b1c363/child/child-home-fef460923d2248bf99da87f8d4b1c363-c1/attributes/location/value": "home",
				"/updater/emea/child/location/enabled":                                                                                                "true",
			},
		},
		{
			func() DynamicRule { return NewAndRule(rules[0], rules[1]) },
			nil,
			"/emea/branch/parent/fef460923d2248bf99da87f8d4b1c363/child/child-home-fef460923d2248bf99da87f8d4b1c363-c1/attributes/location/value",
			sTP("home"),
			true,
			false,
			map[string]string{
				"/emea/branch/parent/fef460923d2248bf99da87f8d4b1c363/child/child-home-fef460923d2248bf99da87f8d4b1c363-c1/attributes/location/value": "home",
				"/updater/emea/child/location/enabled":                                                                                                "true",
			},
		},
	}
	for idx, testCase := range testCases {
		var dr DynamicRule
		if testCase.get != nil {
			dr = testCase.get()
		}
		if testCase.getErr != nil {
			var err error
			dr, err = testCase.getErr()
			assert.NoError(t, err, "index %d", idx)
		}
		rules = append(rules, dr)
		satisfied, err := RuleSatisfied(dr, testCase.key, testCase.value, testCase.kvs)
		assert.Equal(t, testCase.satisfied, satisfied, "index %d", idx)
		if testCase.err {
			assert.Error(t, err, "index %d", idx)
		} else {
			assert.NoError(t, err, "index %d", idx)
		}
	}
}
