package rules

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type errorReadAPI struct {
}

func (era *errorReadAPI) get(key string) (*string, error) {
	return nil, errAPI
}

func (era *errorReadAPI) getCachedAPI(keys []string) (readAPI, error) {
	return era, nil
}

var errorAPI = errorReadAPI{}
var errAPI = errors.New("API Error")

type dummyRule struct {
	satisfiableResponse, satisfiedResponse bool
	key                                    string
	attr                                   Attributes
	err                                    error
	qSatisfiableResponse                   quadState
	expectedKey                            *string
	expectedValue                          **string
}

func (dr *dummyRule) getAttributes() Attributes {
	return dr.attr
}

func (dr *dummyRule) satisfiable(key string, value *string) bool {
	return dr.satisfiableResponse
}

func (dr *dummyRule) qSatisfiable(key string, value *string) quadState {
	if dr.expectedKey != nil && *dr.expectedKey != key {
		panic("Key did not match")
	}
	if dr.expectedValue != nil {
		eVal := *dr.expectedValue
		if eVal == nil {
			if value != nil {
				panic("Value did not match")
			}
		} else {
			if value == nil || *value != *eVal {
				panic("Value did not match")
			}
		}
	}
	return dr.qSatisfiableResponse
}

func (dr *dummyRule) String() string {
	return fmt.Sprintf("qSatisfiable: %d", dr.qSatisfiableResponse)
}

func (dr *dummyRule) satisfied(api readAPI) (bool, error) {
	return dr.satisfiedResponse, dr.err
}

func (dr *dummyRule) keyMatch(key string) bool {
	return dr.key == key
}

func (dr *dummyRule) getKeys() []string {
	return []string{dr.key}
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

func TestCompareLiteralEquals(t *testing.T) {
	ruleValue := "val1"
	factory := newCompareLiteralRuleFactory(newEqualsComparator(&ruleValue), "%s = val1")
	rule := factory.newRule([]string{"/prefix/mykey"}, getTestAttributes())
	queryValue := "val1"
	result := rule.satisfiable("/prefix/mykey", &queryValue)
	assert.True(t, result)
	verifyTestAttributes(t, rule)
	assert.Equal(t, "/prefix/mykey = val1", rule.String())
	factory = newCompareLiteralRuleFactory(newEqualsComparator(nil), "%s = <nil>")
	rule = factory.newRule([]string{"/prefix/mykey"}, getTestAttributes())
	assert.Equal(t, "/prefix/mykey = <nil>", rule.String())
}

func TestCompareLiteralError(t *testing.T) {
	ruleValue := "val1"
	factory := newCompareLiteralRuleFactory(newEqualsComparator(&ruleValue), "%s = val1")
	rule := factory.newRule([]string{"/prefix/mykey"}, getTestAttributes())
	_, err := rule.satisfied(&errorAPI)
	assert.Equal(t, errAPI, err)
}

func TestCompareLiteralEqualsNil(t *testing.T) {
	rule := compareLiteralRule{
		key:            "/prefix/mykey",
		comparator:     newEqualsComparator(nil),
		stringTemplate: "%s = <nil>",
	}
	result := rule.satisfiable("/prefix/mykey", nil)
	assert.True(t, result)
	assert.Equal(t, "/prefix/mykey = <nil>", rule.String())
}

func TestCompareLiteralKeyMismatch(t *testing.T) {
	ruleValue := "val1"
	queryValue := "val1"
	rule := compareLiteralRule{
		key:        "/prefix/mykey1",
		comparator: newEqualsComparator(&ruleValue),
	}
	result := rule.satisfiable("/prefix/mykey2", &queryValue)
	assert.False(t, result)
	assert.False(t, rule.keyMatch("val2"))
}

func TestCompareLiteralOnlyRuleNil(t *testing.T) {
	queryValue := "val1"
	rule := compareLiteralRule{
		key:        "/prefix/mykey",
		comparator: newEqualsComparator(nil),
	}
	result := rule.satisfiable("/prefix/mykey", &queryValue)
	assert.True(t, result)
}

func TestCompareLiteralOnlyQueryNil(t *testing.T) {
	ruleValue := "val1"
	rule := compareLiteralRule{
		key:        "/prefix/mykey",
		comparator: newEqualsComparator(&ruleValue),
	}
	result := rule.satisfiable("/prefix/mykey", nil)
	assert.True(t, result)
}

func Test_compareLiteralRule_getKeys(t *testing.T) {
	elr := &compareLiteralRule{
		key: "abc",
	}
	assert.Equal(t, []string{"abc"}, elr.getKeys())
}

func TestCompareLiteralFactory(t *testing.T) {
	value := "val1"
	factory := compareLiteralRuleFactory{
		comparator: newEqualsComparator(&value),
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

func Test_compoundStaticRule_getKeys(t *testing.T) {
	csr := &compoundStaticRule{
		nestedRules: []staticRule{
			&compareLiteralRule{
				key: "abc",
			},
			&compareLiteralRule{
				key: "def",
			},
		},
	}
	assert.Equal(t, []string{"abc", "def"}, csr.getKeys())
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

func TestNotEquals(t *testing.T) {
	nested := equalsRule{
		keys: []string{"key1", "key2"},
	}
	rule := notStaticRule{
		nested: &nested,
	}
	assert.True(t, rule.satisfiable("key1", sTP("whatever")))
	assert.False(t, rule.satisfiable("key3", nil))
	api := newMapReadAPI()
	// Both values nil
	{
		sat, err := rule.satisfied(api)
		assert.NoError(t, err)
		assert.False(t, sat)
	}
	api.put("key1", "value")
	// One value not nil, the other nil
	{
		sat, err := rule.satisfied(api)
		assert.NoError(t, err)
		assert.True(t, sat)
	}
	api.put("key2", "other_value")
	// Both values not nil and different
	{
		sat, err := rule.satisfied(api)
		assert.NoError(t, err)
		assert.True(t, sat)
	}
}

func Test_notStaticRule_getKeys(t *testing.T) {
	elr := &compareLiteralRule{
		key: "abc",
	}
	nsr := &notStaticRule{
		nested: elr,
	}
	assert.Equal(t, []string{"abc"}, nsr.getKeys())
}

func sTP(str string) *string {
	return &str
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

	// Both nil
	sat, _ = rule.satisfied(api)
	assert.True(t, sat)
	api.put("key2", anything)
	// One nil, the other not
	sat, _ = rule.satisfied(api)
	assert.False(t, sat)
	// Both not nil and same value
	api.put("key1", anything)
	sat, _ = rule.satisfied(api)
	assert.True(t, sat)
	// Both not nil and different values
	api.put("key1", "anyting else")
	sat, _ = rule.satisfied(api)
	assert.False(t, sat)

	api = newMapReadAPI()
	api.put("key1", anything)
	// One nil, the other not
	sat, _ = rule.satisfied(api)
	assert.False(t, sat)
	_, err := rule.satisfied(&errorAPI)
	assert.Error(t, err)
	api.put("key2", errorValue)
	_, err = rule.satisfied(api)
	assert.Error(t, err)

}

func Test_equalsRule_getKeys(t *testing.T) {
	er := &equalsRule{
		keys: []string{"abc", "def"},
	}
	assert.Equal(t, []string{"abc", "def"}, er.getKeys())
}

type srtc struct {
	name   string
	rule   func() staticRule
	key    string
	value  *string
	qState quadState
}

func TestEqualsLiteralQSatisfiable(t *testing.T) {
	rules := map[string]staticRule{}
	key1 := "key1"
	key2 := "key2"
	value1 := "value1"
	value2 := "value2"
	testCases := []*srtc{}
	tInfo := []struct {
		name       string
		value      *string
		inputKey   string
		inputValue *string
		result     quadState
	}{
		{
			"equalsLiteralSameKeyValueToSameValue",
			&value1,
			key1,
			&value1,
			qTrue,
		},
		{
			"equalsLiteralSameKeyValueToDifferentValue",
			&value2,
			key1,
			&value1,
			qFalse,
		},
		{
			"equalsLiteralSameKeyNilToNil",
			nil,
			key1,
			nil,
			qTrue,
		},
		{
			"equalsLiteralSameKeyNilToValue",
			nil,
			key1,
			&value1,
			qFalse,
		},
		{
			"equalsLiteralSameKeyValueToNil",
			&value1,
			key1,
			nil,
			qFalse,
		},
		// Keys are different
		{
			"equalsLiteralDiffKeyValueToSameValue",
			&value1,
			key2,
			&value1,
			qNone,
		},
		{
			"equalsLiteralDiffKeyValueToDifferentValue",
			&value2,
			key2,
			&value1,
			qNone,
		},
		{
			"equalsLiteralDiffKeyNilToNil",
			nil,
			key2,
			nil,
			qNone,
		},
		{
			"equalsLiteralDiffKeyNilToValue",
			nil,
			key2,
			&value1,
			qNone,
		},
		{
			"equalsLiteralDiffKeyValueToNil",
			&value1,
			key2,
			nil,
			qNone,
		},
	}
	for _, i := range tInfo {
		// Assign local variables because pointer variables in the iterated items
		// get re-used in loops.
		value := i.value
		inputValue := i.inputValue
		testCases = append(testCases, &srtc{
			name: i.name,
			rule: func() staticRule {
				return &compareLiteralRule{
					key:        key1,
					comparator: newEqualsComparator(value),
				}
			},
			key:    i.inputKey,
			value:  inputValue,
			qState: i.result,
		})
	}
	testQSatisfiable(t, testCases, rules)
}

func TestEqualsQSatisfiable(t *testing.T) {
	key1 := "key1"
	key2 := "key2"
	key3 := "key3"
	value := "value"
	rules := map[string]staticRule{}
	tInfo := []struct {
		name     string
		keys     []string
		inputKey string
		qState   quadState
	}{
		{
			"equalsKey1AndKey2ToKey1",
			[]string{key1, key2},
			key1,
			qMaybe,
		},
		{
			"equalsKey1AndKey2ToKey2",
			[]string{key1, key2},
			key2,
			qMaybe,
		},
		{
			"equalsKey1AndKey2ToKey3",
			[]string{key1, key2},
			key3,
			qNone,
		},
	}
	testCases := []*srtc{}
	for _, inputVal := range []*string{nil, &value} {
		localInputVal := inputVal
		for _, i := range tInfo {
			valType := "Nil"
			if localInputVal != nil {
				valType = "Not" + valType
			}
			testCases = append(testCases,
				&srtc{
					name:   i.name + valType + "Value",
					rule:   func() staticRule { return &equalsRule{keys: i.keys} },
					key:    i.inputKey,
					value:  localInputVal,
					qState: i.qState,
				},
			)
		}
	}
	testQSatisfiable(t, testCases, rules)
}

func TestCompoundQSatisfiable(t *testing.T) {
	value := "value"
	valuePtr := &value
	key1 := "key1"
	rules := map[string]staticRule{}
	testCases := []*srtc{
		{
			name: "dummyTrue",
			rule: func() staticRule {
				return &dummyRule{
					qSatisfiableResponse: qTrue,
					expectedKey:          &key1,
					expectedValue:        &valuePtr,
				}
			},
			value:  &value,
			qState: qTrue,
		},
		{
			name: "dummyFalse",
			rule: func() staticRule {
				return &dummyRule{
					qSatisfiableResponse: qFalse,
					expectedKey:          &key1,
					expectedValue:        &valuePtr,
				}
			},
			value:  &value,
			qState: qFalse,
		},
		{
			name: "dummyMaybe",
			rule: func() staticRule {
				return &dummyRule{
					qSatisfiableResponse: qMaybe,
					expectedKey:          &key1,
					expectedValue:        &valuePtr,
				}
			},

			qState: qMaybe,
		},
		{
			name: "dummyNone",
			rule: func() staticRule {
				return &dummyRule{
					qSatisfiableResponse: qNone,
					expectedKey:          &key1,
					expectedValue:        &valuePtr,
				}
			},
			qState: qNone,
		},
		{
			name:   "TrueAndTrue",
			rule:   func() staticRule { return asrfn(rules["dummyTrue"], rules["dummyTrue"]) },
			qState: qTrue,
		},
		{
			name:   "TrueAndFalse",
			rule:   func() staticRule { return asrfn(rules["dummyTrue"], rules["dummyFalse"]) },
			qState: qFalse,
		},
		{
			name:   "TrueAndMaybe",
			rule:   func() staticRule { return asrfn(rules["dummyTrue"], rules["dummyMaybe"]) },
			qState: qTrue,
		},
		{
			name:   "TrueAndNone",
			rule:   func() staticRule { return asrfn(rules["dummyTrue"], rules["dummyMaybe"]) },
			qState: qTrue,
		},
		{
			name:   "FalseAndFalse",
			rule:   func() staticRule { return asrfn(rules["dummyFalse"], rules["dummyFalse"]) },
			qState: qFalse,
		},
		{
			name:   "FalseAndMaybe",
			rule:   func() staticRule { return asrfn(rules["dummyFalse"], rules["dummyMaybe"]) },
			qState: qFalse,
		},
		{
			name:   "FalseAndNone",
			rule:   func() staticRule { return asrfn(rules["dummyFalse"], rules["dummyNone"]) },
			qState: qFalse,
		},
		{
			name:   "MaybeAndMaybe",
			rule:   func() staticRule { return asrfn(rules["dummyMaybe"], rules["dummyMaybe"]) },
			qState: qMaybe,
		},
		{
			name:   "MaybeAndNone",
			rule:   func() staticRule { return asrfn(rules["dummyMaybe"], rules["dummyNone"]) },
			qState: qMaybe,
		},
		{
			name:   "TrueOrTrue",
			rule:   func() staticRule { return osrfn(rules["dummyTrue"], rules["dummyTrue"]) },
			qState: qTrue,
		},
		{
			name:   "TrueOrFalse",
			rule:   func() staticRule { return osrfn(rules["dummyTrue"], rules["dummyFalse"]) },
			qState: qTrue,
		},
		{
			name:   "TrueOrMaybe",
			rule:   func() staticRule { return osrfn(rules["dummyTrue"], rules["dummyMaybe"]) },
			qState: qTrue,
		},
		{
			name:   "TrueOrNone",
			rule:   func() staticRule { return osrfn(rules["dummyTrue"], rules["dummyMaybe"]) },
			qState: qTrue,
		},
		{
			name:   "FalseOrFalse",
			rule:   func() staticRule { return osrfn(rules["dummyFalse"], rules["dummyFalse"]) },
			qState: qFalse,
		},
		{
			name:   "FalseOrMaybe",
			rule:   func() staticRule { return osrfn(rules["dummyFalse"], rules["dummyMaybe"]) },
			qState: qMaybe,
		},
		{
			name:   "FalseOrNone",
			rule:   func() staticRule { return osrfn(rules["dummyFalse"], rules["dummyNone"]) },
			qState: qFalse,
		},
		{
			name:   "MaybeOrMaybe",
			rule:   func() staticRule { return osrfn(rules["dummyMaybe"], rules["dummyMaybe"]) },
			qState: qMaybe,
		},
		{
			name:   "MaybeOrNone",
			rule:   func() staticRule { return osrfn(rules["dummyMaybe"], rules["dummyNone"]) },
			qState: qMaybe,
		},
		{
			name:   "NotTrue",
			rule:   func() staticRule { return &notStaticRule{nested: rules["dummyTrue"]} },
			qState: qFalse,
		},
		{
			name:   "NotFalse",
			rule:   func() staticRule { return &notStaticRule{nested: rules["dummyFalse"]} },
			qState: qTrue,
		},
		{
			name:   "NotMaybe",
			rule:   func() staticRule { return &notStaticRule{nested: rules["dummyMaybe"]} },
			qState: qMaybe,
		},
		{
			name:   "NotNone",
			rule:   func() staticRule { return &notStaticRule{nested: rules["dummyNone"]} },
			qState: qNone,
		},
		{
			name:   "Not(TrueOrFalse) <=> NotTrueAndNotFalse",
			rule:   func() staticRule { return &notStaticRule{nested: rules["TrueOrFalse"]} },
			qState: qFalse,
		},
		{
			name: "NotTrueAndNotFalse <=> Not(TrueOrFalse)",
			rule: func() staticRule {
				return asrfn(&notStaticRule{nested: rules["dummyTrue"]}, &notStaticRule{nested: rules["dummyFalse"]})
			},
			qState: qFalse,
		},
		{
			name:   "Not(FalseOrFalse) <=> NotFalseAndNotFalse",
			rule:   func() staticRule { return &notStaticRule{nested: rules["FalseOrFalse"]} },
			qState: qTrue,
		},
		{
			name: "NotFalseAndNotFalse <=> Not(FalseOrFalse)",
			rule: func() staticRule {
				return asrfn(&notStaticRule{nested: rules["dummyFalse"]}, &notStaticRule{nested: rules["dummyFalse"]})
			},
			qState: qTrue,
		},
		{
			name:   "Not(TrueAndFalse) <=> NotTrueOrNotFalse",
			rule:   func() staticRule { return &notStaticRule{nested: rules["TrueAndFalse"]} },
			qState: qTrue,
		},
		{
			name: "NotTruOrNotFalse <=> Not(TrueAndFalse)",
			rule: func() staticRule {
				return osrfn(&notStaticRule{nested: rules["dummyTrue"]}, &notStaticRule{nested: rules["dummyFalse"]})
			},
			qState: qTrue,
		},
		{
			name:   "Not(TrueAndTrue) <=> NotTrueOrNotTrue",
			rule:   func() staticRule { return &notStaticRule{nested: rules["TrueAndTrue"]} },
			qState: qFalse,
		},
		{
			name: "NotTrueOrNotTrue <=> Not(TrueOrTrue)",
			rule: func() staticRule {
				return asrfn(&notStaticRule{nested: rules["dummyTrue"]}, &notStaticRule{nested: rules["dummyTrue"]})
			},
			qState: qFalse,
		},
	}
	for _, testCase := range testCases {
		testCase.key = key1
		testCase.value = &value
	}
	testQSatisfiable(t, testCases, rules)
}

func asrfn(rules ...staticRule) staticRule {
	return &andStaticRule{compoundStaticRule: compoundStaticRule{nestedRules: rules}}
}

func osrfn(rules ...staticRule) staticRule {
	return &orStaticRule{compoundStaticRule: compoundStaticRule{nestedRules: rules}}
}

func testQSatisfiable(t *testing.T, testCases []*srtc, rules map[string]staticRule) {
	for _, testCase := range testCases {
		rule := testCase.rule()
		val := "<nil>"
		if testCase.value != nil {
			val = *testCase.value
		}
		rules[testCase.name] = rule
		qState := evaluateQSatisfiable(t, testCase.name, rule, testCase.key, testCase.value) //rule.qSatisfiable(testCase.key, testCase.value)
		assert.Equal(t, testCase.qState, qState, "%s: %s (%s, %s)", testCase.name, rule, testCase.key, val)
	}
}

func evaluateQSatisfiable(t *testing.T, name string, rule staticRule, key string, value *string) quadState {
	defer func() {
		r := recover()
		if r != nil {
			t.Fatalf("Panic on %s: %s", name, r)
		}
	}()
	return rule.qSatisfiable(key, value)
}
