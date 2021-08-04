package rules

import (
	"fmt"
	"strings"
)

type quadState int

const (
	// The key/value pair makes a rule true or contributes to making it true
	qTrue quadState = iota
	// The key/value pair makes a rule false or matches a key involved in the
	// rule but its value does not have the potential to make it true
	qFalse
	// The key/value pair has the potential to make a rule true,
	// for instance when dealing with a rule that compares the values of two keys and
	// one of the keys is the key of the key/value pair
	qMaybe
	// The key/value pair has no impact on a rule or any part of it
	qNone
)

type ruleFactory interface {
	// The actual keys derived from patterns
	newRule(keys []string, attr Attributes) staticRule
}

type staticRule interface {
	keyMatch(key string) bool
	satisfiable(key string, value *string) bool
	// This method determines whether the given key/value pair contributes to making
	// the rule true; it does NOT determine whether the rule can be true given the
	// key/value pair.  For instance, if the rule consists of an OR expression whose
	// first part evaluates to qFalse and whose second part evaluates to qNone, the
	// overall rule should evaluate to qFalse, although the rule could still evaluate
	// to true if the data store contained values that would cause the second part to evaluate
	// to true.  The purpose of this method is to determine whether the store should be
	// queried for additional values, and in the example the given key/value pair does
	// not warrant further queries.  The evaluation of the rule should be triggered either
	// when the matching value is passed in for the same key or a different key/value pair
	// is passed in that would cause the second part of the rule to evaluate to qTrue or qMaybe.
	qSatisfiable(key string, value *string) quadState
	satisfied(api readAPI) (bool, error)
	getAttributes() Attributes
	String() string
}

type readAPI interface {
	get(string) (*string, error)
}

type baseRule struct {
	attr Attributes
}

func (br *baseRule) getAttributes() Attributes {
	return br.attr
}

type compareLiteralRule struct {
	baseRule
	key            string
	comparator     func(*string) bool
	stringTemplate string
}

type compareLiteralRuleFactory struct {
	comparator func(*string) bool
	// This is used to render the output of the String() method,
	// where the placeholder is for the etcd key.
	stringTemplate string
}

func newEqualsComparator(value *string) func(*string) bool {
	return func(comparedValue *string) bool {
		if value == nil {
			return comparedValue == nil
		}
		return comparedValue != nil && *comparedValue == *value
	}
}

// When comparator returns true for a given string pointer value, the rule is satisfied.
// The string template value is used to render the output of the String() method, where the
// placeholder is the etcd key.  An example:
// %s = "value"
// This can help with debugging rules.
func newCompareLiteralRuleFactory(comparator func(*string) bool, stringTemplate string) ruleFactory {
	factory := compareLiteralRuleFactory{
		comparator:     comparator,
		stringTemplate: stringTemplate,
	}
	return &factory
}

func (elrf *compareLiteralRuleFactory) newRule(keys []string, attr Attributes) staticRule {
	br := baseRule{
		attr: attr,
	}
	r := compareLiteralRule{
		baseRule:       br,
		key:            keys[0],
		comparator:     elrf.comparator,
		stringTemplate: elrf.stringTemplate,
	}
	return &r
}

func (elr *compareLiteralRule) String() string {
	return fmt.Sprintf(elr.stringTemplate, elr.key)
}

func (elr *compareLiteralRule) satisfiable(key string, value *string) bool {
	return key == elr.key
}

func (elr *compareLiteralRule) qSatisfiable(key string, value *string) quadState {
	if key != elr.key {
		return qNone
	}
	if elr.comparator(value) {
		return qTrue
	}
	return qFalse
}

func (elr *compareLiteralRule) satisfied(api readAPI) (bool, error) {
	value, err := api.get(elr.key)
	if err != nil {
		return false, err
	}
	return elr.comparator(value), nil
}

func (elr *compareLiteralRule) keyMatch(key string) bool {
	return elr.key == key
}

type compoundStaticRule struct {
	nestedRules []staticRule
}

func (csr *compoundStaticRule) getAttributes() Attributes {
	return csr.nestedRules[0].getAttributes()
}

func (csr *compoundStaticRule) satisfiable(key string, value *string) bool {
	anySatisfiable := false
	for _, rule := range csr.nestedRules {
		if rule.satisfiable(key, value) {
			anySatisfiable = true
			break
		}
	}
	return anySatisfiable
}

func (asr *andStaticRule) qSatisfiable(key string, value *string) quadState {
	anyTrue := false
	anyMaybe := false
	anyFalse := false
	for _, rule := range asr.nestedRules {
		nqs := rule.qSatisfiable(key, value)
		switch nqs {
		case qTrue:
			anyTrue = true
		case qFalse:
			anyFalse = true
		case qMaybe:
			anyMaybe = true
		}
	}
	if anyFalse {
		return qFalse
	}
	if anyTrue {
		return qTrue
	}
	if anyMaybe {
		return qMaybe
	}
	return qNone
}

func (csr *compoundStaticRule) keyMatch(key string) bool {
	for _, rule := range csr.nestedRules {
		if rule.keyMatch(key) {
			return true
		}
	}
	return false
}

type andStaticRule struct {
	compoundStaticRule
}

func (asr *andStaticRule) String() string {
	nestedRules := []string{}
	for _, rule := range asr.nestedRules {
		nestedRules = append(nestedRules, fmt.Sprint(rule))
	}
	return fmt.Sprintf("(%s)", strings.Join(nestedRules, " AND "))
}

func (asr *andStaticRule) satisfied(api readAPI) (bool, error) {
	for _, rule := range asr.nestedRules {
		satisfied, err := rule.satisfied(api)
		if err != nil {
			return false, err
		}
		if !satisfied {
			return false, nil
		}
	}
	return true, nil
}

type orStaticRule struct {
	compoundStaticRule
}

func (osr *orStaticRule) String() string {
	nestedRules := []string{}
	for _, rule := range osr.nestedRules {
		nestedRules = append(nestedRules, fmt.Sprint(rule))
	}
	return fmt.Sprintf("(%s)", strings.Join(nestedRules, " OR "))
}

func (osr *orStaticRule) qSatisfiable(key string, value *string) quadState {
	anyTrue := false
	anyMaybe := false
	anyFalse := false
	for _, rule := range osr.nestedRules {
		nqs := rule.qSatisfiable(key, value)
		switch nqs {
		case qTrue:
			anyTrue = true
		case qMaybe:
			anyMaybe = true
		case qFalse:
			anyFalse = true
		}

	}
	if anyTrue {
		return qTrue
	}
	if anyMaybe {
		return qMaybe
	}
	if anyFalse {
		return qFalse
	}
	return qNone
}

func (osr *orStaticRule) satisfied(api readAPI) (bool, error) {
	for _, rule := range osr.nestedRules {
		satisfied, err := rule.satisfied(api)
		if err != nil {
			return false, err
		}
		if satisfied {
			return true, nil
		}
	}
	return false, nil
}

type notStaticRule struct {
	nested staticRule
}

func (nsr *notStaticRule) String() string {
	return fmt.Sprintf("NOT (%s)", nsr.nested)
}

func (nsr *notStaticRule) getAttributes() Attributes {
	return nsr.nested.getAttributes()
}

func (nsr *notStaticRule) keyMatch(key string) bool {
	return nsr.nested.keyMatch(key)
}

func (nsr *notStaticRule) satisfiable(key string, value *string) bool {
	return nsr.nested.keyMatch(key)
}

func (nsr *notStaticRule) qSatisfiable(key string, value *string) quadState {
	nqs := nsr.nested.qSatisfiable(key, value)
	switch nqs {
	case qMaybe:
		return qMaybe
	case qTrue:
		return qFalse
	case qFalse:
		return qTrue
	}
	return qNone
}

func (nsr *notStaticRule) satisfied(api readAPI) (bool, error) {
	satisfied, err := nsr.nested.satisfied(api)
	if err != nil {
		return false, err
	}
	return !satisfied, nil
}

type equalsRule struct {
	baseRule
	keys []string
}

func (er *equalsRule) String() string {
	return strings.Join(er.keys, " = ")
}

func (er *equalsRule) satisfiable(key string, value *string) bool {
	return er.keyMatch(key)
}

func (er *equalsRule) qSatisfiable(key string, value *string) quadState {
	if er.keyMatch(key) {
		return qMaybe
	}
	return qNone
}

func (er *equalsRule) keyMatch(key string) bool {
	if len(er.keys) == 0 {
		return true
	}
	for _, ruleKey := range er.keys {
		if key == ruleKey {
			return true
		}
	}
	return false
}

func (er *equalsRule) satisfied(api readAPI) (bool, error) {
	if len(er.keys) == 0 {
		return true, nil
	}
	ref, err1 := api.get(er.keys[0])
	// Failed to get reference value?
	if err1 != nil {
		return false, err1
	}
	for index, key := range er.keys {
		if index == 0 {
			continue
		}
		// Failed to get next value?
		value, err2 := api.get(key)
		if err2 != nil {
			return false, err2
		}
		// Value is nil
		if value == nil {
			// Reference value isn't
			if ref != nil {
				return false, nil
			}
		} else {
			// Value is not nil but reference is
			if ref == nil {
				return false, nil
			}
			// Neither is nil
			if *ref != *value {
				return false, nil
			}
		}
	}
	return true, nil
}

type equalsRuleFactory struct{}

func (erf *equalsRuleFactory) newRule(keys []string, attr Attributes) staticRule {
	br := baseRule{
		attr: attr,
	}
	er := equalsRule{
		baseRule: br,
		keys:     keys,
	}
	return &er
}

func newEqualsRuleFactory() ruleFactory {
	erf := equalsRuleFactory{}
	return &erf
}
