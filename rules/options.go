package rules

import (
	"time"

	"golang.org/x/net/context"
)

type ContextProvider func() (context.Context, context.CancelFunc)

func defaultContextProvider() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Minute*5)
}

type engineOptions struct {
	concurrency, crawlerTTL, syncGetTimeout, syncInterval, watchTimeout int
	syncDelay                                                           int
	constraints                                                         map[string]constraint
	contextProvider                                                     ContextProvider
	keyExpansion                                                        map[string][]string
	lockTimeout                                                         int
	crawlMutex                                                          *string
	ruleWorkBuffer                                                      int
}

func makeEngineOptions(options ...EngineOption) engineOptions {
	opts := engineOptions{
		concurrency:     5,
		constraints:     map[string]constraint{},
		contextProvider: defaultContextProvider,
		syncDelay:       1,
		lockTimeout:     30,
		syncInterval:    300,
		syncGetTimeout:  0,
		watchTimeout:    0,
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
func EngineLockTimeout(lockTimeout int) EngineOption {
	return engineOptionFunction(func(o *engineOptions) {
		o.lockTimeout = lockTimeout
	})
}

// EngineConcurrency controls the number of concurrent workers
// processing rule tasks.
func EngineConcurrency(workers int) EngineOption {
	return engineOptionFunction(func(o *engineOptions) {
		o.concurrency = workers
	})
}

// EngineWatchTimeout controls the timeout of a watch operation in seconds.
func EngineWatchTimeout(watchTimeout int) EngineOption {
	return engineOptionFunction(func(o *engineOptions) {
		o.watchTimeout = watchTimeout
	})
}

// KeyExpansion enables attributes in rules to be fixed at run time
// while allowing the rule declarations to continue to use the
// attribute placeholders.  For instance, an application may
// use a root directory "/:geo" to hold data for a given geography.
// Passing map[string][]string{"geo":{"na"}} into the KeyExpansion
// option will cause all rules with the "/:geo/" prefix to be rendered
// as "/na/..." but all paths rendered with attributes from realized
// rules will still correctly resolve ":geo" to "na".  This allows
// the placeholder values to be set as application configuration
// settings while minimizing the scope of the watchers.
func KeyExpansion(keyExpansion map[string][]string) EngineOption {
	return engineOptionFunction(func(o *engineOptions) {
		// Combine existing pairings with additional pairings, with
		// collisions resolved by having later values overwrite
		// earlier ones, i.e. "last one wins"
		if o.keyExpansion != nil {
			for k, v := range keyExpansion {
				o.keyExpansion[k] = v
			}
			return
		}
		o.keyExpansion = keyExpansion
	})
}

// KeyConstraint enables multiple query prefixes to be generated for a specific
// attribute as a way to limit the scope of a query for a prefix query.
func KeyConstraint(attribute string, prefix string, chars [][]rune) EngineOption {
	return engineOptionFunction(func(o *engineOptions) {
		o.constraints[attribute] = constraint{
			chars:  chars,
			prefix: prefix,
		}
	})
}

// EngineSyncInterval enables the interval between sync or crawler runs to be configured.
// The interval is in seconds.
func EngineSyncInterval(interval int) EngineOption {
	return engineOptionFunction(func(o *engineOptions) {
		o.syncInterval = interval
	})
}

// EngineSyncDelay enables the throttling of the crawlers by introducing a delay (in ms)
// between queries to keep the crawlers from overwhelming etcd.
func EngineSyncDelay(delay int) EngineOption {
	return engineOptionFunction(func(o *engineOptions) {
		o.syncDelay = delay
	})
}

// EngineContextProvider sets a custom provider for generating context instances for use
// by callbacks.
func EngineContextProvider(cp ContextProvider) EngineOption {
	return engineOptionFunction(func(o *engineOptions) {
		o.contextProvider = cp
	})
}

// EngineCrawlMutex sets an application identifier mutex and a TTL value for the mutex
// to limit the number of instances of an application performing a crawl at any given
// time to one.  mutexTTL refers to how long the mutex is in effect; if set too short,
// multiple instances of an application may end up crawling simultaneously.  Note that this
// functionality is only implemented in etcd v3 and that a mutex in etcd v3 is held
// only while the app instance that created it is still active. This means that setting
// a high value, such as 3600 seconds, does not expose one to the risk of no crawls
// occuring for a maximum of one hour if an application instance terminates at the
// beginning of a crawler run.
func EngineCrawlMutex(mutex string, mutexTTL int) EngineOption {
	return engineOptionFunction(func(o *engineOptions) {
		o.crawlMutex = &mutex
		o.crawlerTTL = mutexTTL
	})
}

// EngineRuleWorkBuffer sets the limit on the number of ruleWork in the channel
// without a receiving worker.
func EngineRuleWorkBuffer(buffer int) EngineOption {
	return engineOptionFunction(func(o *engineOptions) {
		o.ruleWorkBuffer = buffer
	})
}

type ruleOptions struct {
	lockTimeout     int
	contextProvider ContextProvider
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
func RuleLockTimeout(lockTimeout int) RuleOption {
	return ruleOptionFunction(func(o *ruleOptions) {
		o.lockTimeout = lockTimeout
	})
}

// RuleContextProvider sets a custom provider for generating context instances for use
// by a specific callback.
func RuleContextProvider(cp ContextProvider) RuleOption {
	return ruleOptionFunction(func(o *ruleOptions) {
		o.contextProvider = cp
	})
}
