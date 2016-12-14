package rules

import ()

type engineOptions struct {
	concurrency, syncGetTimeout, syncInterval, watchTimeout int
	lockTimeout                                             uint64
}

func makeEngineOptions(options ...EngineOption) engineOptions {
	opts := engineOptions{
		concurrency:    5,
		lockTimeout:    30,
		syncInterval:   300,
		syncGetTimeout: 0,
		watchTimeout:   0,
	}
	for _, opt := range options {
		opt.apply(&opts)
	}
	return opts
}

// EngineOption instances control the overall behavior of an Engine
// instance.  Behavior for individual rules can be controlled via
// RuleOption instances.
type EngineOption interface {
	apply(*engineOptions)
}

type engineOptionFunction func(*engineOptions)

func (f engineOptionFunction) apply(o *engineOptions) {
	f(o)
}

// EngineLockTimeout controls the TTL of a lock in seconds.
func EngineLockTimeout(lockTimeout uint64) EngineOption {
	return engineOptionFunction(func(o *engineOptions) {
		o.lockTimeout = lockTimeout
	})
}

type ruleOptions struct {
	lockTimeout uint64
}

func makeRuleOptions(options ...RuleOption) ruleOptions {
	opts := ruleOptions{
		lockTimeout: 0,
	}
	for _, opt := range options {
		opt.apply(&opts)
	}
	return opts
}

// RuleOption instances control the behavior of individual rules.
type RuleOption interface {
	apply(*ruleOptions)
}

type ruleOptionFunction func(*ruleOptions)

func (f ruleOptionFunction) apply(o *ruleOptions) {
	f(o)
}

// RuleLockTimeout controls the TTL of the locks associated
// with the rule, in seconds.
func RuleLockTimeout(lockTimeout uint64) RuleOption {
	return ruleOptionFunction(func(o *ruleOptions) {
		o.lockTimeout = lockTimeout
	})
}
