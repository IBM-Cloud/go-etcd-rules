package rules

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mapReadAPI struct {
	values map[string]string
}

func newMapReadAPI() *mapReadAPI {
	values := make(map[string]string)
	return &mapReadAPI{
		values: values,
	}
}

var errorValue = "<ERROR>"

func (mra *mapReadAPI) get(key string) (*string, error) {
	value, ok := mra.values[key]
	if !ok {
		return nil, nil
	}
	if value == errorValue {
		return nil, errors.New("Error")
	}
	return &value, nil
}

func (mra *mapReadAPI) put(key string, value string) {
	mra.values[key] = value
}

type errorReadAPI struct {
}

func (era *errorReadAPI) get(key string) (*string, error) {
	return nil, errAPI
}

var errorAPI = errorReadAPI{}
var errAPI = errors.New("API Error")

type dummyRule struct {
	baseRule
	satisfiableResponse, satisfiedResponse bool
	key                                    string
	attr                                   Attributes
	err                                    error
}

func (dr *dummyRule) getAttributes() Attributes {
	return dr.attr
}

func (dr *dummyRule) satisfiable(key string, value *string) bool {
	return dr.satisfiableResponse
}

func (dr *dummyRule) satisfied(api readAPI) (bool, error) {
	return dr.satisfiedResponse, dr.err
}

func (dr *dummyRule) keyMatch(key string) bool {
	return dr.key == key
}

func getTestAttributes() Attributes {
	attributeMap := map[string]string{
		"testkey": "testvalue",
	}
	attr := mapAttributes{
		values: attributeMap,
	}
	return &attr
}

func verifyTestAttributes(t *testing.T, rule staticRule) {
	attr := rule.getAttributes()
	assert.Equal(t, "testvalue", *attr.GetAttribute("testkey"))
}

func TestEqualsLiteralEquals(t *testing.T) {
	ruleValue := "val1"
	factory := newEqualsLiteralRuleFactory(&ruleValue)
	rule := factory.newRule([]string{"/prefix/mykey"}, getTestAttributes())
	queryValue := "val1"
	result := rule.satisfiable("/prefix/mykey", &queryValue)
	assert.True(t, result)
	verifyTestAttributes(t, rule)
}

func TestEqualsLiteralError(t *testing.T) {
	ruleValue := "val1"
	factory := newEqualsLiteralRuleFactory(&ruleValue)
	rule := factory.newRule([]string{"/prefix/mykey"}, getTestAttributes())
	_, err := rule.satisfied(&errorAPI)
	assert.Equal(t, errAPI, err)
}

func TestEqualsLiteralEqualsNil(t *testing.T) {
	rule := equalsLiteralRule{
		key:   "/prefix/mykey",
		value: nil,
	}
	result := rule.satisfiable("/prefix/mykey", nil)
	assert.True(t, result)
}

func TestEqualsLiteralKeyMismatch(t *testing.T) {
	ruleValue := "val1"
	queryValue := "val1"
	rule := equalsLiteralRule{
		key:   "/prefix/mykey1",
		value: &ruleValue,
	}
	result := rule.satisfiable("/prefix/mykey2", &queryValue)
	assert.False(t, result)
	assert.False(t, rule.keyMatch("val2"))
}

func TestEqualsLiteralOnlyRuleNil(t *testing.T) {
	queryValue := "val1"
	rule := equalsLiteralRule{
		key:   "/prefix/mykey",
		value: nil,
	}
	result := rule.satisfiable("/prefix/mykey", &queryValue)
	assert.False(t, result)
}

func TestEqualsLiteralOnlyQueryNil(t *testing.T) {
	ruleValue := "val1"
	rule := equalsLiteralRule{
		key:   "/prefix/mykey",
		value: &ruleValue,
	}
	result := rule.satisfiable("/prefix/mykey", nil)
	assert.False(t, result)
}

//type mapAttributes struct {
//	attr map[string]string
//}
//
//func (ma *mapAttributes) GetAttribute(key string) *string {
//	value, _ := ma.attr[key]
//	return &value
//}
//
//func (ma *mapAttributes) Format(s string) string {
//	return formatWithAttributes(s, ma)
//}

func TestEqualsLiteralFactory(t *testing.T) {
	value := "val1"
	factory := equalsLiteralRuleFactory{
		value: &value,
	}
	attr := mapAttributes{
		values: make(map[string]string),
	}
	rule := factory.newRule([]string{"/prefix/mykey"}, &attr)
	assert.True(t, rule.satisfiable("/prefix/mykey", &value))
}

func TestCompoundStaticRuleSatisfiable(t *testing.T) {
	trueRule := dummyRule{
		satisfiableResponse: true,
		satisfiedResponse:   false,
	}
	falseRule := dummyRule{
		satisfiableResponse: false,
		satisfiedResponse:   false,
	}

	csr := compoundStaticRule{
		nestedRules: []staticRule{&trueRule, &falseRule},
	}

	assert.True(t, csr.satisfiable("test", nil))
}

func TestCompoundStaticRuleNotSatisfiable(t *testing.T) {
	falseRule1 := dummyRule{
		satisfiableResponse: false,
		satisfiedResponse:   false,
	}
	falseRule2 := dummyRule{
		satisfiableResponse: false,
		satisfiedResponse:   false,
	}

	csr := compoundStaticRule{
		nestedRules: []staticRule{&falseRule1, &falseRule2},
	}

	assert.False(t, csr.satisfiable("test", nil))
}

func TestCompoundStaticRuleKeyMatch(t *testing.T) {
	keyRule1 := dummyRule{
		key: "key1",
	}
	keyRule2 := dummyRule{
		key: "key2",
	}

	csr := compoundStaticRule{
		nestedRules: []staticRule{&keyRule1, &keyRule2},
	}
	assert.True(t, csr.keyMatch("key1"))
	assert.True(t, csr.keyMatch("key2"))
	assert.False(t, csr.keyMatch("key3"))
}

func TestAndStaticRuleSatisfied(t *testing.T) {
	trueRule1 := dummyRule{
		satisfiableResponse: true,
		satisfiedResponse:   true,
		attr:                getTestAttributes(),
	}
	trueRule2 := dummyRule{
		satisfiableResponse: true,
		satisfiedResponse:   true,
		attr:                getTestAttributes(),
	}

	csr := compoundStaticRule{
		nestedRules: []staticRule{&trueRule1, &trueRule2},
	}

	and := andStaticRule{
		compoundStaticRule: csr,
	}
	sat, _ := and.satisfied(newMapReadAPI())
	assert.True(t, sat)
	verifyTestAttributes(t, &and)
}

func TestAndStaticRuleSatisfiedError(t *testing.T) {
	trueRule1 := dummyRule{
		satisfiableResponse: true,
		satisfiedResponse:   true,
		attr:                getTestAttributes(),
		err:                 errors.New("some error"),
	}
	trueRule2 := dummyRule{
		satisfiableResponse: true,
		satisfiedResponse:   true,
		attr:                getTestAttributes(),
	}

	csr := compoundStaticRule{
		nestedRules: []staticRule{&trueRule1, &trueRule2},
	}

	and := andStaticRule{
		compoundStaticRule: csr,
	}
	_, err := and.satisfied(newMapReadAPI())
	assert.Error(t, err)
}

func TestAndStaticRuleNotSatisfied(t *testing.T) {
	trueRule := dummyRule{
		satisfiableResponse: true,
		satisfiedResponse:   true,
	}
	falseRule := dummyRule{
		satisfiableResponse: false,
		satisfiedResponse:   false,
	}

	csr := compoundStaticRule{
		nestedRules: []staticRule{&trueRule, &falseRule},
	}
	and := andStaticRule{
		compoundStaticRule: csr,
	}
	sat, _ := and.satisfied(newMapReadAPI())
	assert.False(t, sat)
}

func TestOrStaticRuleSatisfied(t *testing.T) {
	trueRule1 := dummyRule{
		satisfiableResponse: true,
		satisfiedResponse:   true,
		attr:                getTestAttributes(),
	}
	trueRule2 := dummyRule{
		satisfiableResponse: false,
		satisfiedResponse:   false,
		attr:                getTestAttributes(),
	}

	csr := compoundStaticRule{
		nestedRules: []staticRule{&trueRule1, &trueRule2},
	}

	or := orStaticRule{
		compoundStaticRule: csr,
	}
	sat, _ := or.satisfied(newMapReadAPI())
	assert.True(t, sat)
	verifyTestAttributes(t, &or)
}

func TestOrStaticRuleSatisfiedError(t *testing.T) {
	trueRule1 := dummyRule{
		satisfiableResponse: true,
		satisfiedResponse:   true,
		attr:                getTestAttributes(),
		err:                 errors.New("some error"),
	}
	trueRule2 := dummyRule{
		satisfiableResponse: true,
		satisfiedResponse:   true,
		attr:                getTestAttributes(),
	}

	csr := compoundStaticRule{
		nestedRules: []staticRule{&trueRule1, &trueRule2},
	}

	or := orStaticRule{
		compoundStaticRule: csr,
	}
	_, err := or.satisfied(newMapReadAPI())
	assert.Error(t, err)
}

func TestNotStaticRule(t *testing.T) {
	keyMatchNotSatisfied := dummyRule{
		satisfiedResponse:   false,
		satisfiableResponse: false,
		key:                 "key1",
		attr:                getTestAttributes(),
	}
	rule := notStaticRule{
		nested: &keyMatchNotSatisfied,
	}
	verifyTestAttributes(t, &rule)
	api := newMapReadAPI()
	assert.True(t, rule.satisfiable("key1", nil))
	assert.False(t, rule.satisfiable("key2", nil))
	var sat bool
	var err error
	sat, err = rule.satisfied(api)
	assert.True(t, sat)
	assert.True(t, rule.keyMatch("key1"))
	assert.False(t, rule.keyMatch("key2"))
	assert.NoError(t, err)
	keyMatchSatisfied := dummyRule{
		satisfiedResponse:   true,
		satisfiableResponse: true,
		key:                 "key1",
	}
	rule = notStaticRule{
		nested: &keyMatchSatisfied,
	}
	assert.False(t, rule.satisfiable("key1", nil))
	sat, err = rule.satisfied(api)
	assert.False(t, sat)
	assert.NoError(t, err)
	keyMatchError := dummyRule{
		err: errAPI,
	}
	rule.nested = &keyMatchError
	_, err = rule.satisfied(api)
	assert.Error(t, err)
}

func TestEquals(t *testing.T) {
	rule := equalsRule{
		keys: []string{},
	}
	anything := "anything"
	assert.True(t, rule.satisfiable(anything, nil))
	api := newMapReadAPI()
	var sat bool
	sat, _ = rule.satisfied(api)
	assert.True(t, sat)

	rule = equalsRule{
		keys: []string{"key1", "key2"},
	}
	assert.False(t, rule.satisfiable("key3", nil))
	assert.False(t, rule.satisfiable("key3", &anything))

	assert.True(t, rule.satisfiable("key1", nil))
	assert.True(t, rule.satisfiable("key2", nil))

	sat, _ = rule.satisfied(api)
	assert.True(t, sat)
	api.put("key2", anything)
	sat, _ = rule.satisfied(api)
	assert.False(t, sat)
	api.put("key1", anything)
	sat, _ = rule.satisfied(api)
	assert.True(t, sat)
	api = newMapReadAPI()
	api.put("key1", anything)
	sat, _ = rule.satisfied(api)
	assert.False(t, sat)
	_, err := rule.satisfied(&errorAPI)
	assert.Error(t, err)
	api.put("key2", errorValue)
	_, err = rule.satisfied(api)
	assert.Error(t, err)
}
