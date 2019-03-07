package rules

import (
	"strings"
)

type prefixMap struct {
	prefixes     map[string]string
	currentIndex int
}

func NewPrefixMap() prefixMap {
	return prefixMap{
		prefixes:     map[string]string{},
		currentIndex: 0,
	}
}

func (p *prefixMap) Set(prefix, value string) {
	p.prefixes[prefix] = value
}

type ruleManager struct {
	constraints        map[string]constraint
	currentIndex       int
	rulesBySlashCount  map[int]map[DynamicRule]int
	watcherPrefixes    prefixMap
	crawlerPrefixes    prefixMap
	rules              []DynamicRule
	enhancedRuleFilter bool
}

func newRuleManager(constraints map[string]constraint, enhancedRuleFilter bool) ruleManager {
	rm := ruleManager{
		rulesBySlashCount:  map[int]map[DynamicRule]int{},
		watcherPrefixes:    NewPrefixMap(),
		crawlerPrefixes:    NewPrefixMap(),
		constraints:        constraints,
		currentIndex:       0,
		rules:              []DynamicRule{},
		enhancedRuleFilter: enhancedRuleFilter,
	}
	return rm
}

func (rm *ruleManager) getStaticRules(key string, value *string) map[staticRule]int {
	slashCount := strings.Count(key, "/")
	out := make(map[staticRule]int)
	rules, ok := rm.rulesBySlashCount[slashCount]
	if ok {
		for rule, index := range rules {
			sRule, _, inScope := rule.makeStaticRule(key, value)
			if inScope {
				if rm.enhancedRuleFilter {
					qSat := sRule.qSatisfiable(key, value)
					if qSat == qTrue || qSat == qMaybe {
						out[sRule] = index
					}
				} else {
					if sRule.satisfiable(key, value) {
						out[sRule] = index
					}
				}
			}
		}
	}
	return out
}

func (rm *ruleManager) addRule(rule DynamicRule, watcherOnly bool) int {
	p := &rm.watcherPrefixes
	if !watcherOnly {
		p = &rm.crawlerPrefixes
	}
	rm.rules = append(rm.rules, rule)
	for _, pattern := range rule.getPatterns() {
		slashCount := strings.Count(pattern, "/")
		rules, ok := rm.rulesBySlashCount[slashCount]
		if !ok {
			rules = map[DynamicRule]int{}
			rm.rulesBySlashCount[slashCount] = rules
		}
		rules[rule] = rm.currentIndex
	}
	for _, prefix := range rule.getPrefixesWithConstraints(rm.constraints) {
		p.Set(prefix, "")
	}
	p.prefixes = reducePrefixes(p.prefixes)
	lastIndex := p.currentIndex
	p.currentIndex = p.currentIndex + 1
	return lastIndex
}

// Removes any path prefixes that have other path prefixes as
// string prefixes
func reducePrefixes(prefixes map[string]string) map[string]string {
	out := map[string]string{}
	sorted := sortPrefixesByLength(prefixes)
	for _, prefix := range sorted {
		add := true
		for addedPrefix := range out {
			if strings.HasPrefix(prefix, addedPrefix) {
				add = false
			}
		}
		if add {
			out[prefix] = ""
		}
	}
	return out
}

// Sorts prefixes shortest to longest
func sortPrefixesByLength(prefixes map[string]string) []string {
	out := []string{}
	for prefix := range prefixes {
		out = append(out, prefix)
	}
	for i := 1; i < len(out); i++ {
		x := out[i]
		j := i - 1
		for j >= 0 && len(out[j]) > len(x) {
			out[j+1] = out[j]
			j = j - 1
		}
		out[j+1] = x
	}
	return out
}

func getCrawlGuidesForRules(rules []DynamicRule) []string {
	return combineRuleData(rules, getCrawlerPatterns)
}

func getRulePatterns(rules []DynamicRule) []string {
	return combineRuleData(rules, func(rule DynamicRule) []string { return rule.getPatterns() })
}

func combineRuleData(rules []DynamicRule, source func(DynamicRule) []string) []string {
	crawlGuides := []string{}
	for _, rule := range rules {
		crawlGuides = append(crawlGuides, source(rule)...)
	}
	crawlGuides = removeDuplicates(crawlGuides)
	return crawlGuides
}
