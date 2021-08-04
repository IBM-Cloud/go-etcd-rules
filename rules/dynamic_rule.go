package rules

import (
	"errors"
	"fmt"
	"strings"
)

// DynamicRule defines rules that have dynamic key paths so that classes of keys can be
// referenced in rules.
type DynamicRule interface {
	makeStaticRule(key string, value *string) (staticRule, Attributes, bool)
	staticRuleFromAttributes(attr Attributes) (staticRule, bool)
	getPatterns() []string
	getPrefixes() []string
	getPrefixesWithConstraints(constraints map[string]constraint) []string
	Expand(map[string][]string) ([]DynamicRule, bool)
	evaluate(map[string]bool) bool
	getLeafRepresentations() []string
	getLeafRepresentationPatternMap() map[string][]string
}

func newDynamicRule(factory ruleFactory, patterns []string, rep string, attributes ...attributeInstance) (DynamicRule, error) {
	matchers := make([]keyMatcher, len(patterns))
	prefixes := make([]string, len(patterns))
	for i, v := range patterns {
		matcher, err := newRegexKeyMatcher(v)
		if err != nil {
			return nil, err
		}
		matchers[i] = matcher
		prefixes[i] = matcher.getPrefix()
	}
	pattern := dynamicRule{
		factory:    factory,
		matchers:   matchers,
		patterns:   patterns,
		prefixes:   prefixes,
		attributes: attributes,
		rep:        rep,
	}
	return &pattern, nil
}

type dynamicRule struct {
	factory    ruleFactory
	matchers   []keyMatcher
	patterns   []string
	prefixes   []string
	attributes []attributeInstance
	rep        string
}

func getEssentialRepresentations(rule DynamicRule) []string {
	// Provides the string representations of the leaf rules that must
	// always be true in order for the overall rule to evaluate to true
	mustAlwaysBeTrue := map[string]bool{}
	reps := rule.getLeafRepresentations()
	for _, rep := range reps {
		mustAlwaysBeTrue[rep] = true
	}
	parent := map[string]bool{}
	proc := func(values map[string]bool) {
		result := rule.evaluate(values)
		if result {
			for _, rep := range reps {
				if !values[rep] {
					mustAlwaysBeTrue[rep] = false
				}
			}
		}
	}
	for _, val := range []bool{false, true} {
		processBooleanMap(reps, parent, val, proc)
	}
	essentialReps := []string{}
	for _, rep := range reps {
		if mustAlwaysBeTrue[rep] {
			essentialReps = append(essentialReps, rep)
		}
	}
	return essentialReps
}

func processBooleanMap(keys []string, parent map[string]bool, value bool, proc func(map[string]bool)) {
	if len(keys) == 0 {
		return
	}
	child := map[string]bool{}
	for k, v := range parent {
		child[k] = v
	}
	child[keys[0]] = value
	if len(keys) == 1 {
		proc(child)
		return
	}
	for _, val := range []bool{false, true} {
		processBooleanMap(keys[1:], child, val, proc)
	}

}

func (krp *dynamicRule) getLeafRepresentationPatternMap() map[string][]string {
	return map[string][]string{
		krp.rep: krp.patterns,
	}
}

func (krp *dynamicRule) getLeafRepresentations() []string {
	return []string{krp.rep}
}

func (krp *dynamicRule) evaluate(values map[string]bool) bool {
	return values[krp.rep]
}

func (krp *dynamicRule) String() string {
	return krp.rep
}

func (krp *dynamicRule) Expand(valueMap map[string][]string) ([]DynamicRule, bool) {
	params := map[string]string{}
	for _, pattern := range krp.patterns {
		fieldsToParms, _ := parsePath(pattern)
		for parm := range fieldsToParms {
			params[parm] = ""
		}
	}
	expanded := false
	out := []DynamicRule{}
	// Iterate through all parameters in the rule's key patterns
	for param := range params {
		// See if the parameter is in the provided value map
		values, ok := valueMap[param]
		if ok {
			expanded = true
			// Iterate through all values for the parameter in the value map
			for _, value := range values {
				newPatterns := []string{}
				attrs := mapAttributes{values: map[string]string{param: value}}
				for _, pattern := range krp.patterns {
					newPatterns = append(newPatterns, FormatWithAttributes(pattern, &attrs))
				}
				// Create a new rule instance for each value in the value map
				newAttributes := []attributeInstance{}
				newAttributes = append(newAttributes, krp.attributes...)
				// Need a new variable because the value memory location is reused during
				// each loop iteration
				valref := value
				newAttributes = append(newAttributes, attributeInstance{key: param, value: &valref})
				rep := strings.Replace(krp.rep, "/:"+param+"/", "/"+value+"/", -1)
				rule, err := newDynamicRule(krp.factory, newPatterns, rep, newAttributes...)
				// Expand the new rule instance
				if err == nil {
					exp, _ := rule.Expand(valueMap)
					out = append(out, exp...)
				}
			}
			break
		}
	}
	if !expanded {
		out = append(out, krp)
	}
	return out, expanded
}

func (krp *dynamicRule) getPatterns() []string {
	return krp.patterns
}

func (krp *dynamicRule) getPrefixes() []string {
	return krp.prefixes
}

func (krp *dynamicRule) getPrefixesWithConstraints(constraints map[string]constraint) []string {
	prefixes := []string{}
	for _, km := range krp.matchers {
		matchPrefixes := km.getPrefixesWithConstraints(constraints)
		prefixes = append(prefixes, matchPrefixes...)
	}
	return prefixes
}

type attributeInstance struct {
	key   string
	value *string
}

type nestingAttributes struct {
	nested Attributes
	attrs  []attributeInstance
}

func (na *nestingAttributes) GetAttribute(key string) *string {
	for _, attribute := range na.attrs {
		if attribute.key == key {
			return attribute.value
		}
	}
	return na.nested.GetAttribute(key)
}

func (na *nestingAttributes) Format(pattern string) string {
	return FormatWithAttributes(pattern, na)
}

func (krp *dynamicRule) makeStaticRule(key string, value *string) (staticRule, Attributes, bool) {
	var match keyMatch
	anyMatch := false
	for _, matcher := range krp.matchers {
		m, ok := matcher.match(key)
		if ok {
			match = m
			anyMatch = true
			break
		}
	}
	if anyMatch {
		keys := make([]string, len(krp.matchers))
		for i, matcher := range krp.matchers {
			keys[i] = match.Format(matcher.getPattern())
		}
		attr := nestingAttributes{attrs: krp.attributes, nested: match}
		sr := krp.factory.newRule(keys, &attr)
		return sr, &attr, true
	}
	return nil, nil, false
}

func (krp *dynamicRule) staticRuleFromAttributes(attr Attributes) (staticRule, bool) {
	keys := make([]string, len(krp.matchers))
	for i, matcher := range krp.matchers {
		key, ok := formatPath(matcher.getPattern(), attr)
		if !ok {
			return nil, false
		}
		keys[i] = key
	}
	sr := krp.factory.newRule(keys, attr)
	return sr, true
}

// NewEqualsLiteralRule creates a rule that compares the provided string value with the
// value of a node whose key matches the provided key pattern. A nil value indicates that
// there is no node with the given key.
func NewEqualsLiteralRule(pattern string, value *string) (DynamicRule, error) {
	valString := "<nil>"
	if value != nil {
		valString = `"` + *value + `"`
	}
	return NewCompareLiteralRule(pattern, newEqualsComparator(value), fmt.Sprintf("%s %s %s", "%s", "=", valString))
}

// NewCompareLiteralRule creates a rule that allows arbitrary comparisons to be performed
// against values in etcd.
// When comparator returns true for a given string pointer value, the rule is satisfied.
// The string template value is used to render the output of the String() method, with a single
// string placeholder that is the etcd key or key pattern.  An example:
// %s = "value"
// This can help with debugging rules.
func NewCompareLiteralRule(pattern string, comparator func(*string) bool, renderTemplate string) (DynamicRule, error) {
	if comparator == nil {
		return nil, errors.New("Comparator cannot be nil")
	}
	f := newCompareLiteralRuleFactory(comparator, renderTemplate)
	return newDynamicRule(f, []string{pattern}, fmt.Sprintf(renderTemplate, pattern))
}

type compoundDynamicRule struct {
	nestedDynamicRules []DynamicRule
	patterns           []string
	prefixes           []string
}

func (cdr *compoundDynamicRule) getLeafRepresentationPatternMap() map[string][]string {
	out := map[string][]string{}
	for _, rule := range cdr.nestedDynamicRules {
		for k, v := range rule.getLeafRepresentationPatternMap() {
			var patterns []string
			var ok bool
			if patterns, ok = out[k]; !ok {
				patterns = []string{}
			}
			out[k] = append(patterns, v...)
		}
	}
	for k, v := range out {
		out[k] = removeDuplicates(v)
	}
	return out
}

func (cdr *compoundDynamicRule) getLeafRepresentations() []string {
	reps := map[string]bool{}
	for _, rule := range cdr.nestedDynamicRules {
		for _, rep := range rule.getLeafRepresentations() {
			reps[rep] = true
		}
	}
	result := []string{}
	for rep := range reps {
		result = append(result, rep)
	}
	return result
}

func (cdr *compoundDynamicRule) makeStaticRule(key string, value *string) (*compoundStaticRule, Attributes, bool) {
	anySatisfiable := false
	var validAttr Attributes
	for _, nestedRule := range cdr.nestedDynamicRules {
		rule, attr, ok := nestedRule.makeStaticRule(key, value)
		if !ok {
			continue
		}
		anySatisfiable = rule.satisfiable(key, value)
		if anySatisfiable {
			validAttr = attr
			break
		}
	}
	if !anySatisfiable {
		return nil, nil, false
	}
	rule, ok := cdr.staticRuleFromAttributes(validAttr)
	if !ok {
		return nil, nil, false
	}
	return rule, validAttr, true
}

func (cdr *compoundDynamicRule) staticRuleFromAttributes(validAttr Attributes) (*compoundStaticRule, bool) {

	staticRules := make([]staticRule, len(cdr.nestedDynamicRules))
	for i, nestedRule := range cdr.nestedDynamicRules {
		rule, ok := nestedRule.staticRuleFromAttributes(validAttr)
		if !ok {
			return nil, false
		}
		staticRules[i] = rule
	}
	out := compoundStaticRule{
		nestedRules: staticRules,
	}
	return &out, true
}

func newCompoundDynamicRule(rules []DynamicRule) compoundDynamicRule {
	var patterns []string
	var prefixes []string
	for _, rule := range rules {
		patterns = append(patterns, rule.getPatterns()...)
		prefixes = append(prefixes, rule.getPrefixes()...)
	}
	patterns = removeDuplicates(patterns)
	prefixes = removeDuplicates(prefixes)
	cdr := compoundDynamicRule{
		nestedDynamicRules: rules,
		patterns:           patterns,
		prefixes:           prefixes,
	}
	return cdr
}

func removeDuplicates(in []string) []string {
	unique := map[string]bool{}
	for _, value := range in {
		unique[value] = true
	}
	out := []string{}
	for value := range unique {
		out = append(out, value)
	}
	return out
}

func (cdr *compoundDynamicRule) getPatterns() []string {
	return cdr.patterns
}

func getCrawlerPatterns(rule DynamicRule) []string {
	patterns := map[string]bool{}
	for _, pattern := range rule.getPatterns() {
		patterns[pattern] = true
	}
	// Get all the representations of leaf rules that must evaluate
	// to true for the overall rule to evaluate to true
	eReps := getEssentialRepresentations(rule)
	mappings := rule.getLeafRepresentationPatternMap()
	for _, rep := range eReps {
		if strings.HasSuffix(rep, "<nil>") {
			// Remove the pattern (there will only be one) associated
			// with the leaf rule, so it isn't crawled
			for _, pattern := range mappings[rep] {
				delete(patterns, pattern)
			}
		}
	}
	out := []string{}
	for k := range patterns {
		out = append(out, k)
	}
	return out
}

func (cdr *compoundDynamicRule) getPrefixes() []string {
	return cdr.prefixes
}

type newCompoundDynamicRuleFunc func(...DynamicRule) DynamicRule

func (cdr *compoundDynamicRule) getPrefixesWithConstraints(constraints map[string]constraint) []string {
	prefixes := []string{}
	for _, nested := range cdr.nestedDynamicRules {
		prefixes = append(prefixes, nested.getPrefixesWithConstraints(constraints)...)
	}
	return prefixes
}

func (cdr *compoundDynamicRule) expand(valueMap map[string][]string,
	constructor newCompoundDynamicRuleFunc,
	dr DynamicRule) ([]DynamicRule, bool) {
	expanded := false
	out := []DynamicRule{}
	for key, values := range valueMap {
		keyExpansion := false
		newRules := []DynamicRule{}
		for _, value := range values {
			attr := map[string][]string{key: {value}}
			newNested := []DynamicRule{}
			for _, nested := range cdr.nestedDynamicRules {
				expandedRule, gotExpanded := nested.Expand(attr)
				if gotExpanded {
					keyExpansion = true
				}
				newNested = append(newNested, expandedRule[0])
			}
			newRules = append(newRules, constructor(newNested...))
		}
		if keyExpansion {
			expanded = true
			for _, rule := range newRules {
				expandedRules, _ := rule.Expand(valueMap)
				out = append(out, expandedRules...)
			}
			break
		}
	}
	if !expanded {
		out = append(out, dr)
	}
	return out, expanded
}

type andDynamicRule struct {
	compoundDynamicRule
}

func (adr *andDynamicRule) evaluate(values map[string]bool) bool {
	result := true
	for _, rule := range adr.compoundDynamicRule.nestedDynamicRules {
		result = result && rule.evaluate(values)
	}
	return result
}

func (adr *andDynamicRule) String() string {
	return adr.delimitedString("AND")
}

func (odr *orDynamicRule) String() string {
	return odr.delimitedString("OR")
}

func (cdr *compoundDynamicRule) delimitedString(del string) string {
	out := strings.Builder{}
	out.WriteByte('(')
	for idx, rule := range cdr.nestedDynamicRules {
		if idx > 0 {
			out.WriteByte(' ')
			out.WriteString(del)
			out.WriteByte(' ')
		}
		out.WriteString(fmt.Sprintf("%s", rule))
	}
	out.WriteByte(')')
	return out.String()
}

func (adr *andDynamicRule) makeStaticRule(key string, value *string) (staticRule, Attributes, bool) {
	cdr, attr, ok := adr.compoundDynamicRule.makeStaticRule(key, value)
	if !ok {
		return nil, nil, false
	}
	return &andStaticRule{
		compoundStaticRule: *cdr,
	}, attr, ok
}

func (adr *andDynamicRule) staticRuleFromAttributes(attr Attributes) (staticRule, bool) {
	cdr, ok := adr.compoundDynamicRule.staticRuleFromAttributes(attr)
	if !ok {
		return nil, false
	}
	return &andStaticRule{
		compoundStaticRule: *cdr,
	}, true
}

func (adr *andDynamicRule) Expand(valueMap map[string][]string) ([]DynamicRule, bool) {
	return adr.compoundDynamicRule.expand(valueMap, NewAndRule, adr)
}

// NewAndRule allows two or more dynamic rules to be combined into a single rule
// such that every nested rule must be satisfied in order for the overall rule to be
// satisfied.
func NewAndRule(rules ...DynamicRule) DynamicRule {
	cdr := newCompoundDynamicRule(rules)
	rule := andDynamicRule{
		compoundDynamicRule: cdr,
	}
	return &rule
}

type orDynamicRule struct {
	compoundDynamicRule
}

func (odr *orDynamicRule) evaluate(values map[string]bool) bool {
	result := false
	for _, rule := range odr.compoundDynamicRule.nestedDynamicRules {
		result = result || rule.evaluate(values)
	}
	return result
}

func (odr *orDynamicRule) Expand(valueMap map[string][]string) ([]DynamicRule, bool) {
	return odr.compoundDynamicRule.expand(valueMap, NewOrRule, odr)
}

func (odr *orDynamicRule) makeStaticRule(key string, value *string) (staticRule, Attributes, bool) {
	cdr, attr, ok := odr.compoundDynamicRule.makeStaticRule(key, value)
	if !ok {
		return nil, nil, false
	}
	return &orStaticRule{
		compoundStaticRule: *cdr,
	}, attr, ok
}

func (odr *orDynamicRule) staticRuleFromAttributes(attr Attributes) (staticRule, bool) {
	cdr, ok := odr.compoundDynamicRule.staticRuleFromAttributes(attr)
	if !ok {
		return nil, false
	}
	return &orStaticRule{
		compoundStaticRule: *cdr,
	}, true
}

// NewOrRule allows two or more dynamic rules to be combined into a single rule
// such that at least one nested rule must be satisfied in order for the overall rule to be
// satisfied.
func NewOrRule(rules ...DynamicRule) DynamicRule {
	cdr := newCompoundDynamicRule(rules)
	rule := orDynamicRule{
		compoundDynamicRule: cdr,
	}
	return &rule
}

type notDynamicRule struct {
	compoundDynamicRule
}

func (ndr *notDynamicRule) Expand(valueMap map[string][]string) ([]DynamicRule, bool) {
	return ndr.compoundDynamicRule.expand(valueMap, newNotRule, ndr)
}

func (ndr *notDynamicRule) String() string {
	return "NOT (" + fmt.Sprintf("%s", ndr.nestedDynamicRules[0]) + ")"
}

func (ndr *notDynamicRule) evaluate(values map[string]bool) bool {
	return !ndr.nestedDynamicRules[0].evaluate(values)
}

func newNotRule(rules ...DynamicRule) DynamicRule {
	cdr := newCompoundDynamicRule(rules)
	rule := notDynamicRule{
		compoundDynamicRule: cdr,
	}
	return &rule
}

func (ndr *notDynamicRule) makeStaticRule(key string, value *string) (staticRule, Attributes, bool) {
	ns, attr, ok := ndr.nestedDynamicRules[0].makeStaticRule(key, value)
	if !ok {
		return nil, nil, false
	}
	nsr := notStaticRule{
		nested: ns,
	}
	return &nsr, attr, ok
}

func (ndr *notDynamicRule) staticRuleFromAttributes(attr Attributes) (staticRule, bool) {
	nested, ok := ndr.nestedDynamicRules[0].staticRuleFromAttributes(attr)
	if !ok {
		return nil, false
	}
	nsr := notStaticRule{
		nested: nested,
	}
	return &nsr, true
}

func (ndr *notDynamicRule) getPatterns() []string {
	return ndr.nestedDynamicRules[0].getPatterns()
}

func (ndr *notDynamicRule) getPrefixes() []string {
	return ndr.nestedDynamicRules[0].getPrefixes()
}

// NewNotRule allows a rule to be negated such that if the
// nested rule's key matches but the rule is otherwise not
// satisfied, the not rule is satisfied. This is to enable
// capabilities such as checking whether a given key is
// set, i.e. its value is not nil.
func NewNotRule(nestedRule DynamicRule) DynamicRule {
	r := []DynamicRule{nestedRule}
	cdr := newCompoundDynamicRule(r)

	ndr := notDynamicRule{
		compoundDynamicRule: cdr,
	}
	return &ndr
}

// NewEqualsRule enables the comparison of two or more node
// values with the specified key patterns.
func NewEqualsRule(pattern []string) (DynamicRule, error) {
	f := newEqualsRuleFactory()
	rep := strings.Join(pattern, " = ")
	return newDynamicRule(f, pattern, rep)
}

// FormatRuleString creates an indented, more readable version of a rule string
func FormatRuleString(in string) string {
	out := ""
	indent := 0
	for _, char := range in {
		if char == ')' {
			indent--
			out = out + "\n" + strings.Repeat(" ", indent*4)
		}
		out = out + string(char)
		if char == '(' {
			indent++
			out = out + "\n" + strings.Repeat(" ", indent*4)
		}
	}
	return out
}

// RuleSatisfied returns true if the rule was satisfied and false if it was not.  An error is
// returned if the trigger key did not contain the required path variables to evaluate the rule.
func RuleSatisfied(rule DynamicRule, triggerKey string, triggerValue *string, kvs map[string]string) (bool, error) {
	sRule, _, ok := rule.makeStaticRule(triggerKey, triggerValue)
	if !ok {
		return false, errors.New("Rule could not be triggered")
	}
	return sRule.satisfied(&mapReadAPI{values: kvs})
}

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
