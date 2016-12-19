package rules

import (
	"fmt"
	"strings"
	"time"

	"github.com/coreos/etcd/client"
	"github.com/uber-go/zap"
	"golang.org/x/net/context"
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
	AddPolling(namespacePattern string,
		preconditions DynamicRule,
		ttl int,
		callback RuleTaskCallback) error
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
	if len(e.options.keyExpansion) > 0 {
		rules, _ := rule.expand(e.options.keyExpansion)
		for _, expRule := range rules {
			e.addRule(expRule, lockPattern, callback, options...)
		}
	} else {
		e.addRule(rule, lockPattern, callback, options...)
	}
}

func (e *engine) AddPolling(namespacePattern string, preconditions DynamicRule, ttl int, callback RuleTaskCallback) error {
	if !strings.HasSuffix(namespacePattern, "/") {
		namespacePattern = namespacePattern + "/"
	}
	ttlPathPattern := namespacePattern + "ttl"
	ttlRule, err := NewEqualsLiteralRule(ttlPathPattern, nil)
	if err != nil {
		return err
	}
	rule := NewAndRule(preconditions, ttlRule)
	cbw := callbackWrapper{
		callback:       callback,
		ttl:            ttl,
		ttlPathPattern: ttlPathPattern,
	}
	e.AddRule(rule, namespacePattern+"lock", cbw.doRule)
	return nil
}

func (e *engine) addRule(rule DynamicRule,
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

		w, err := newWatcher(e.config, prefix, logger, &e.keyProc, e.options.watchTimeout)
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

type callbackWrapper struct {
	ttlPathPattern string
	callback       RuleTaskCallback
	ttl            int
}

func (cbw *callbackWrapper) doRule(task *RuleTask) {
	logger := task.Logger
	cbw.callback(task)
	c, err := client.New(task.Conf)
	if err != nil {
		logger.Error("Error obtaining client", zap.Error(err))
		return
	}
	kapi := client.NewKeysAPI(c)
	path := task.Attr.Format(cbw.ttlPathPattern)
	logger.Debug("Setting polling TTL", zap.String("path", path))
	_, setErr := kapi.Set(
		context.Background(),
		path,
		"",
		&client.SetOptions{TTL: time.Duration(cbw.ttl) * time.Second},
	)
	if setErr != nil {
		logger.Error("Error setting polling TTL", zap.Error(setErr), zap.String("path", path))
	}
}
