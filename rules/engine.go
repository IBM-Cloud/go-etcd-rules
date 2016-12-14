package rules

import (
	"fmt"

	"github.com/coreos/etcd/client"
	"github.com/uber-go/zap"
)

type engine struct {
	config       client.Config
	keyProc      keyProcessor
	logger       zap.Logger
	options      engineOptions
	ruleLockTTLs map[int]uint64
	ruleMgr      ruleManager
	workChannel  chan ruleWork
}

// Engine defines the interactions with a rule engine instance.
type Engine interface {
	AddRule(rule DynamicRule,
		lockPattern string,
		callback RuleTaskCallback,
		options ...RuleOption)
	Run()
}

// NewEngine creates a new Engine instance.
func NewEngine(config client.Config, logger zap.Logger, options ...EngineOption) Engine {
	eng := newEngine(config, logger, options...)
	return &eng
}

func newEngine(config client.Config, logger zap.Logger, options ...EngineOption) engine {
	opts := makeEngineOptions(options...)
	ruleMgr := newRuleManager()
	channel := make(chan ruleWork)
	eng := engine{
		config:       config,
		keyProc:      newKeyProcessor(channel, config, &ruleMgr),
		logger:       logger,
		options:      opts,
		ruleLockTTLs: map[int]uint64{},
		ruleMgr:      ruleMgr,
		workChannel:  channel,
	}
	return eng
}

func (e *engine) AddRule(rule DynamicRule,
	lockPattern string,
	callback RuleTaskCallback,
	options ...RuleOption) {
	ruleIndex := e.ruleMgr.addRule(rule)
	opts := makeRuleOptions(options...)
	ttl := e.options.lockTimeout
	if opts.lockTimeout > 0 {
		ttl = opts.lockTimeout
	}
	e.ruleLockTTLs[ruleIndex] = ttl
	e.keyProc.callbacks[ruleIndex] = callback
	e.keyProc.lockKeyPatterns[ruleIndex] = lockPattern
}

func (e *engine) Run() {
	prefixes := e.ruleMgr.prefixes

	// This is a map; used to ensure there are no duplicates
	for prefix := range prefixes {
		logger := e.logger.With(zap.String("prefix", prefix))

		c, err1 := newCrawler(
			e.config,
			logger,
			prefix,
			e.options.syncInterval,
			&e.keyProc,
		)
		if err1 != nil {
			e.logger.Fatal("Failed to initialize crawler", zap.String("prefix", prefix), zap.Error(err1))
		}
		go c.run()

		w, err := newWatcher(e.config, prefix, logger, &e.keyProc)
		if err != nil {
			e.logger.Fatal("Failed to initialize watcher", zap.String("prefix", prefix))
		}
		go w.run()
	}

	for i := 0; i < e.options.concurrency; i++ {
		id := fmt.Sprintf("worker%d", i)
		w, err := newWorker(id, e, e.config)
		if err != nil {
			e.logger.Fatal("Failed to start worker", zap.String("worker", id))
		}
		go w.run()
	}

}

func (e *engine) getLockTTLForRule(index int) uint64 {
	return e.ruleLockTTLs[index]
}
