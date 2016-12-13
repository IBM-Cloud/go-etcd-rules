package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type dummyRuleTrueFactory struct {
}

func (drtf *dummyRuleTrueFactory) newRule(keys []string) staticRule {
	return &dummyRule{
		satisfiableResponse: true,
		satisfiedResponse:   true,
	}
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
	s3 := a1.staticRuleFromAttributes(attr)
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
	assert.False(t, ok)
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
	s3 := o1.staticRuleFromAttributes(attr)
	sat, _ = s3.satisfied(api)
	assert.True(t, sat)

	api.put("/us-south/actual/clusters/armada-9b93c18d/workers/worker3", "<dir>")
	sat, _ = s1.satisfied(api)
	assert.True(t, sat)
	_, _, ok = o1.makeStaticRule("/us-south/desired/clusters/armada-9b93c18d/workers/worker3/state", nil)
	assert.False(t, ok)
	o2 := NewOrRule(workerPathMissing)
	s2, _, _ := o2.makeStaticRule("/us-south/actual/clusters/armada-9b93c18d/workers/worker3", nil)
	sat, _ = s2.satisfied(api)
	assert.False(t, sat)
}

func TestNotRule(t *testing.T) {
	workerPathMissing, _ := NewEqualsLiteralRule("/:region/actual/clusters/:clusterid/workers/:workerid", nil)
	test := notDynamicRule{
		nestedRule: workerPathMissing,
	}
	value := "value"
	notRule, attr, ok := test.makeStaticRule("/us-south/actual/clusters/armada-9b93c18d/workers/worker3", &value)
	assert.True(t, ok)
	assert.True(t, notRule.satisfiable("/us-south/actual/clusters/armada-9b93c18d/workers/worker3", &value))
	assert.False(t, notRule.satisfiable("/us-south/actual/clusters/armada-9b93c18d/workers/worker3", nil))
	api := newMapReadAPI()
	var sat bool
	sat, _ = notRule.satisfied(api)
	assert.False(t, sat)
	api.put("/us-south/actual/clusters/armada-9b93c18d/workers/worker3", "<dir>")
	sat, _ = notRule.satisfied(api)
	assert.True(t, sat)

	notRule = test.staticRuleFromAttributes(attr)
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
