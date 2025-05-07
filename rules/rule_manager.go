package rules

import (
	"sort"
	"strings"
)

type ruleManager struct {
	constraints        map[string]constraint
	currentIndex       int
	rulesBySlashCount  map[int]map[DynamicRule]int
	prefixes           map[string]ruleMgrRuleOptions
	rules              []DynamicRule
	enhancedRuleFilter bool
}

type ruleMgrRuleOptions struct {
	crawlerOnly bool
	priority    uint
}

func newRuleManager(constraints map[string]constraint, enhancedRuleFilter bool) ruleManager {
	rm := ruleManager{
		rulesBySlashCount:  map[int]map[DynamicRule]int{},
		prefixes:           map[string]ruleMgrRuleOptions{},
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

		_, currentPriority := rm.prefixes[prefix]
		// if value does not exist in map yet
		if !currentPriority {
			rm.prefixes[prefix] = ruleMgrRuleOptions{crawlerOnly: opts.crawlerOnly, priority: opts.priority}
		} else {
			// ensure that no high priority is overwritten
			if rm.prefixes[prefix].priority < opts.priority {
				rm.prefixes[prefix] = ruleMgrRuleOptions{crawlerOnly: rm.prefixes[prefix].crawlerOnly, priority: opts.priority}
			}
			// only update crawlerOnly value if new option is false
			if !opts.crawlerOnly {
				rm.prefixes[prefix] = ruleMgrRuleOptions{crawlerOnly: false, priority: rm.prefixes[prefix].priority}
			}
		}

	}
	rm.prefixes = reducePrefixes(rm.prefixes)
	lastIndex := rm.currentIndex
	rm.currentIndex = rm.currentIndex + 1
	return lastIndex
}

func (rm *ruleManager) getPrioritizedPrefixes() []string {
	out := []string{}
	for prefix := range rm.prefixes {
		out = append(out, prefix)
	}
	// sort slice by highest priority value
	sort.SliceStable(out, func(i, j int) bool {
		return rm.prefixes[out[i]].priority > rm.prefixes[out[j]].priority
	})
	return out
}

func (rm *ruleManager) getWatcherPrefixes() []string {
	out := []string{}
	for prefix, ruleOpt := range rm.prefixes {
		if !ruleOpt.crawlerOnly {
			out = append(out, prefix)
		}
	}
	return out
}

// Removes any path prefixes that have other path prefixes as
// string prefixes
func reducePrefixes(prefixes map[string]ruleMgrRuleOptions) map[string]ruleMgrRuleOptions {
	out := map[string]ruleMgrRuleOptions{}
	sorted := sortPrefixesByLength(prefixes)
	for _, prefix := range sorted {
		add := true
		optionsToAdd := prefixes[prefix]
		for addedPrefix, addedOptions := range out {
			if strings.HasPrefix(prefix, addedPrefix) {
				add = false
				optsToUpdate := out[addedPrefix]
				// update the addedPrefix to be the
				// highest priority of any
				// overlapping prefixes
				if addedOptions.priority < optionsToAdd.priority {
					optsToUpdate.priority = optionsToAdd.priority
					out[addedPrefix] = optsToUpdate
				}
				// if any rule associated with the prefix
				// is not crawler only, set crawlerOnly option
				// to be false
				if !optionsToAdd.crawlerOnly {
					optsToUpdate.crawlerOnly = false
					out[addedPrefix] = optsToUpdate
				}
			}
		}
		if add {
			out[prefix] = optionsToAdd
		}
	}
	return out
}

// Sorts prefixes shortest to longest
func sortPrefixesByLength(prefixes map[string]ruleMgrRuleOptions) []string {
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
