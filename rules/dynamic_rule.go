package rules

import ()

// DynamicRule defines rules that have dynamic key paths so that classes of keys can be
// referenced in rules.
type DynamicRule interface {
	makeStaticRule(key string, value *string) (staticRule, Attributes, bool)
	staticRuleFromAttributes(attr Attributes) staticRule
	getPatterns() []string
	getPrefixes() []string
	getPrefixesWithConstraints(constraints map[string]constraint) []string
	expand(map[string][]string) ([]DynamicRule, bool)
}

func newDynamicRule(factory ruleFactory, patterns []string, attributes ...attributeInstance) (DynamicRule, error) {
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
	}
	return &pattern, nil
}

type dynamicRule struct {
	factory    ruleFactory
	matchers   []keyMatcher
	patterns   []string
	prefixes   []string
	attributes []attributeInstance
}

func (krp *dynamicRule) expand(valueMap map[string][]string) ([]DynamicRule, bool) {
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
					newPatterns = append(newPatterns, formatWithAttributes(pattern, &attrs))
				}
				// Create a new rule instance for each value in the value map
				newAttributes := []attributeInstance{}
				newAttributes = append(newAttributes, krp.attributes...)
				// Need a new variable because the value memory location is reused during
				// each loop iteration
				valref := value
				newAttributes = append(newAttributes, attributeInstance{key: param, value: &valref})
				rule, err := newDynamicRule(krp.factory, newPatterns, newAttributes...)
				// Expand the new rule instance
				if err == nil {
					exp, _ := rule.expand(valueMap)
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
	return formatWithAttributes(pattern, na)
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

func (krp *dynamicRule) staticRuleFromAttributes(attr Attributes) staticRule {
	keys := make([]string, len(krp.matchers))
	for i, matcher := range krp.matchers {
		keys[i] = formatWithAttributes(matcher.getPattern(), attr)
	}
	sr := krp.factory.newRule(keys, attr)
	return sr
}

// NewEqualsLiteralRule creates a rule that compares the provided string value with the
// value of a node whose key matches the provided key pattern. A nil value indicates that
// there is no node with the given key.
func NewEqualsLiteralRule(pattern string, value *string) (DynamicRule, error) {
	f := newEqualsLiteralRuleFactory(value)
	return newDynamicRule(f, []string{pattern})
}

type compoundDynamicRule struct {
	nestedDynamicRules []DynamicRule
	patterns           []string
	prefixes           []string
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
	rule := cdr.staticRuleFromAttributes(validAttr)
	return rule, validAttr, true
}

func (cdr *compoundDynamicRule) staticRuleFromAttributes(validAttr Attributes) *compoundStaticRule {

	staticRules := make([]staticRule, len(cdr.nestedDynamicRules))
	for i, nestedRule := range cdr.nestedDynamicRules {
		rule := nestedRule.staticRuleFromAttributes(validAttr)
		staticRules[i] = rule
	}
	out := compoundStaticRule{
		nestedRules: staticRules,
	}
	return &out
}

func newCompoundDynamicRule(rules []DynamicRule) compoundDynamicRule {
	var patterns []string
	var prefixes []string
	for _, rule := range rules {
		patterns = append(patterns, rule.getPatterns()...)
		prefixes = append(prefixes, rule.getPrefixes()...)
	}
	cdr := compoundDynamicRule{
		nestedDynamicRules: rules,
		patterns:           patterns,
		prefixes:           prefixes,
	}
	return cdr
}

func (cdr *compoundDynamicRule) getPatterns() []string {
	return cdr.patterns
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
				expandedRule, gotExpanded := nested.expand(attr)
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
				expandedRules, _ := rule.expand(valueMap)
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

func (adr *andDynamicRule) makeStaticRule(key string, value *string) (staticRule, Attributes, bool) {
	cdr, attr, ok := adr.compoundDynamicRule.makeStaticRule(key, value)
	if !ok {
		return nil, nil, false
	}
	return &andStaticRule{
		compoundStaticRule: *cdr,
	}, attr, ok
}

func (adr *andDynamicRule) staticRuleFromAttributes(attr Attributes) staticRule {
	cdr := adr.compoundDynamicRule.staticRuleFromAttributes(attr)
	return &andStaticRule{
		compoundStaticRule: *cdr,
	}
}

func (adr *andDynamicRule) expand(valueMap map[string][]string) ([]DynamicRule, bool) {
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

func (odr *orDynamicRule) expand(valueMap map[string][]string) ([]DynamicRule, bool) {
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

func (odr *orDynamicRule) staticRuleFromAttributes(attr Attributes) staticRule {
	cdr := odr.compoundDynamicRule.staticRuleFromAttributes(attr)
	return &orStaticRule{
		compoundStaticRule: *cdr,
	}
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

func (ndr *notDynamicRule) expand(valueMap map[string][]string) ([]DynamicRule, bool) {
	return ndr.compoundDynamicRule.expand(valueMap, newNotRule, ndr)
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
	nsr := notStaticRule{
		nested: ns,
	}
	return &nsr, attr, ok
}

func (ndr *notDynamicRule) staticRuleFromAttributes(attr Attributes) staticRule {
	nsr := notStaticRule{
		nested: ndr.nestedDynamicRules[0].staticRuleFromAttributes(attr),
	}
	return &nsr
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
	return newDynamicRule(f, pattern)
}
