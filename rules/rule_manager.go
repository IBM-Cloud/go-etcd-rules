package rules

import (
	"maps"
	"slices"
	"strings"
)

type ruleManager struct {
	constraints        map[string]constraint
	currentIndex       int
	rulesBySlashCount  map[int]map[DynamicRule]int
	prefixes           map[string]string
	watcherPrefixes    map[string]string
	rules              []DynamicRule
	enhancedRuleFilter bool
}

func newRuleManager(constraints map[string]constraint, enhancedRuleFilter bool) ruleManager {
	rm := ruleManager{
		rulesBySlashCount:  map[int]map[DynamicRule]int{},
		prefixes:           map[string]string{},
		watcherPrefixes:    map[string]string{},
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

func (rm *ruleManager) addRule(rule DynamicRule, opts ruleOptions) int {
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
		if !opts.crawlerOnly {
			rm.watcherPrefixes[prefix] = ""
		}

		// ensure that no high priority is overwritten
		if rm.prefixes[prefix] != "high" {
			if !opts.highPriority {
				rm.prefixes[prefix] = ""
			} else {
				// assign high priority if specified
				rm.prefixes[prefix] = "high"
			}
		}
	}
	rm.prefixes = reducePrefixes(rm.prefixes)
	rm.watcherPrefixes = reducePrefixes(rm.watcherPrefixes)
	lastIndex := rm.currentIndex
	rm.currentIndex = rm.currentIndex + 1
	return lastIndex
}

func (rm *ruleManager) getPrioritizedPrefixes() []string {
	high := []string{}
	low := []string{}
	for prefix, priority := range rm.prefixes {
		if priority == "high" {
			high = append(high, prefix)
		} else {
			low = append(low, prefix)
		}
	}
	return append(high, low...)
}

func (rm *ruleManager) getWatcherPrefixes() []string {
	return slices.Collect(maps.Keys(rm.watcherPrefixes))
}

// Removes any path prefixes that have other path prefixes as
// string prefixes
func reducePrefixes(prefixes map[string]string) map[string]string {
	out := map[string]string{}
	sorted := sortPrefixesByLength(prefixes)
	for _, prefix := range sorted {
		add := true
		priority := prefixes[prefix]
		for addedPrefix, priorityCheck := range out {
			if strings.HasPrefix(prefix, addedPrefix) {
				add = false
				// if any overlapping prefixes are marked
				// as high priority, keep priority high
				if priorityCheck == "high" {
					priority = "high"
				}
			}
		}
		if add {
			out[prefix] = priority
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

func combineRuleData(rules []DynamicRule, source func(DynamicRule) []string) []string {
	crawlGuides := []string{}
	for _, rule := range rules {
		crawlGuides = append(crawlGuides, source(rule)...)
	}
	crawlGuides = removeDuplicates(crawlGuides)
	return crawlGuides
}
