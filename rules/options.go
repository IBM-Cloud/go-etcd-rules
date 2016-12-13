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

type EngineOption interface {
	apply(*engineOptions)
}

type engineOptionFunction func(*engineOptions)

func (f engineOptionFunction) apply(o *engineOptions) {
	f(o)
}

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

type RuleOption interface {
	apply(*ruleOptions)
}

type ruleOptionFunction func(*ruleOptions)

func (f ruleOptionFunction) apply(o *ruleOptions) {
	f(o)
}

func RuleLockTimeout(lockTimeout uint64) RuleOption {
	return ruleOptionFunction(func(o *ruleOptions) {
		o.lockTimeout = lockTimeout
	})
}
