package rules

type ruleFactory interface {
	// The actual keys derived from patterns
	newRule(keys []string, attr Attributes) staticRule
}

type staticRule interface {
	keyMatch(key string) bool
	satisfiable(key string, value *string) bool
	satisfied(api readAPI) (bool, error)
	getAttributes() Attributes
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

type equalsLiteralRule struct {
	baseRule
	key   string
	value *string
}

type EqualsLiteralRuleFactory struct {
	value *string
}

func newEqualsLiteralRuleFactory(value *string) ruleFactory {
	factory := EqualsLiteralRuleFactory{
		value: value,
	}
	return &factory
}

func (elrf *EqualsLiteralRuleFactory) newRule(keys []string, attr Attributes) staticRule {
	br := baseRule{
		attr: attr,
	}
	r := equalsLiteralRule{
		baseRule: br,
		key:      keys[0],
		value:    elrf.value,
	}
	return &r
}

func (elr *equalsLiteralRule) satisfiable(key string, value *string) bool {
	if key != elr.key {
		return false
	}
	if value == nil {
		if elr.value == nil {
			return true
		}
		return false
	}
	if elr.value == nil {
		return false
	}
	return *value == *elr.value
}

func (elr *equalsLiteralRule) satisfied(api readAPI) (bool, error) {
	value, err := api.get(elr.key)
	if err != nil {
		return false, err
	}
	return elr.satisfiable(elr.key, value), nil
}

func (elr *equalsLiteralRule) keyMatch(key string) bool {
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

func (crs *compoundStaticRule) keyMatch(key string) bool {
	for _, rule := range crs.nestedRules {
		if rule.keyMatch(key) {
			return true
		}
	}
	return false
}

type andStaticRule struct {
	compoundStaticRule
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

func (nsr *notStaticRule) getAttributes() Attributes {
	return nsr.nested.getAttributes()
}

func (nsr *notStaticRule) keyMatch(key string) bool {
	return nsr.nested.keyMatch(key)
}

func (nsr *notStaticRule) satisfiable(key string, value *string) bool {
	return nsr.nested.keyMatch(key) && !nsr.nested.satisfiable(key, value)
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

func (er *equalsRule) satisfiable(key string, value *string) bool {
	return er.keyMatch(key)
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
	if err1 != nil {
		return false, err1
	}
	for _, key := range er.keys {
		value, err2 := api.get(key)
		if err2 != nil {
			return false, err2
		}
		if value == nil {
			if ref != nil {
				return false, nil
			}
		} else {
			if ref == nil {
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
