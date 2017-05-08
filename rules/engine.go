package rules

import (
	"fmt"
	"strings"
	"time"

	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/clientv3"
	"github.com/uber-go/zap"
	"golang.org/x/net/context"
)

type stopable interface {
	stop()
	isStopped() bool
}

type BaseEngine interface {
	Run()
	Stop()
	IsStopped() bool
}

type baseEngine struct {
	cCloser      channelCloser
	keyProc      setableKeyProcessor
	logger       zap.Logger
	options      engineOptions
	ruleLockTTLs map[int]int
	ruleMgr      ruleManager
	stopped      uint32
	crawlers     []stopable
	watchers     []stopable
	workers      []stopable
}

type channelCloser func()

type engine struct {
	baseEngine
	config         client.Config
	keyProc        keyProcessor
	workChannel    chan ruleWork
	keysAPIWrapper WrapKeysAPI
}

type v3Engine struct {
	baseEngine
	configV3    clientv3.Config
	keyProc     v3KeyProcessor
	workChannel chan v3RuleWork
	kvWrapper   WrapKV
}

// Engine defines the interactions with a rule engine instance.
type Engine interface {
	BaseEngine
	SetKeysAPIWrapper(WrapKeysAPI)
	AddRule(rule DynamicRule,
		lockPattern string,
		callback RuleTaskCallback,
		options ...RuleOption)
	AddPolling(namespacePattern string,
		preconditions DynamicRule,
		ttl int,
		callback RuleTaskCallback) error
}

// V3Engine defines the interactions with a rule engine instance communicating with etcd v3.
type V3Engine interface {
	BaseEngine
	SetKVWrapper(WrapKV)
	AddRule(rule DynamicRule,
		lockPattern string,
		callback V3RuleTaskCallback,
		options ...RuleOption)
	AddPolling(namespacePattern string,
		preconditions DynamicRule,
		ttl int,
		callback V3RuleTaskCallback) error
}

// NewEngine creates a new Engine instance.
func NewEngine(config client.Config, logger zap.Logger, options ...EngineOption) Engine {
	eng := newEngine(config, clientv3.Config{}, false, logger, options...)
	return &eng
}

// NewV3Engine creates a new V3Engine instance.
func NewV3Engine(configV3 clientv3.Config, logger zap.Logger, options ...EngineOption) V3Engine {
	eng := newV3Engine(client.Config{}, configV3, true, logger, options...)
	return &eng
}

func newEngine(config client.Config, configV3 clientv3.Config, useV3 bool, logger zap.Logger, options ...EngineOption) engine {
	opts := makeEngineOptions(options...)
	ruleMgr := newRuleManager(map[string]constraint{})
	channel := make(chan ruleWork)
	keyProc := newKeyProcessor(channel, config, &ruleMgr)
	eng := engine{
		baseEngine: baseEngine{
			cCloser: func() {
				close(channel)
			},
			keyProc:      &keyProc,
			logger:       logger,
			options:      opts,
			ruleLockTTLs: map[int]int{},
			ruleMgr:      ruleMgr,
		},
		config:         config,
		keyProc:        keyProc,
		workChannel:    channel,
		keysAPIWrapper: defaultWrapKeysAPI,
	}
	return eng
}

func newV3Engine(config client.Config, configV3 clientv3.Config, useV3 bool, logger zap.Logger, options ...EngineOption) v3Engine {
	opts := makeEngineOptions(options...)
	ruleMgr := newRuleManager(opts.constraints)
	channel := make(chan v3RuleWork)
	keyProc := newV3KeyProcessor(channel, &configV3, &ruleMgr)
	eng := v3Engine{
		baseEngine: baseEngine{
			cCloser: func() {
				close(channel)
			},
			keyProc:      &keyProc,
			logger:       logger,
			options:      opts,
			ruleLockTTLs: map[int]int{},
			ruleMgr:      ruleMgr,
		},
		configV3:    configV3,
		keyProc:     keyProc,
		workChannel: channel,
		kvWrapper:   defaultWrapKV,
	}
	return eng
}

func (e *engine) SetKeysAPIWrapper(keysAPIWrapper WrapKeysAPI) {
	e.keysAPIWrapper = keysAPIWrapper
}

func (e *v3Engine) SetKVWrapper(kvWrapper WrapKV) {
	e.kvWrapper = kvWrapper
}

func (e *engine) AddRule(rule DynamicRule,
	lockPattern string,
	callback RuleTaskCallback,
	options ...RuleOption) {
	e.addRuleWithIface(rule, lockPattern, callback, options...)
}

func (e *v3Engine) AddRule(rule DynamicRule,
	lockPattern string,
	callback V3RuleTaskCallback,
	options ...RuleOption) {
	e.addRuleWithIface(rule, lockPattern, callback, options...)
}

func (e *baseEngine) Stop() {
	e.logger.Info("Stopping engine")
	go e.stop()
}

func (e *baseEngine) stop() {
	e.logger.Debug("Stopping crawlers")
	stopStopables(e.crawlers)
	e.logger.Debug("Stopping watchers")
	stopStopables(e.watchers)
	e.logger.Debug("Stopping workers")
	for _, worker := range e.workers {
		worker.stop()
	}
	e.logger.Debug("Closing work channel")
	e.cCloser()
	// Ensure workers are stopped; the "stop" method is called again, but
	// that is idempotent.  The workers' "stop" method must be called before
	// the channel is closed in order to avoid nil pointer dereference panics.
	stopStopables(e.workers)
	atomicSet(&e.stopped, true)
	e.logger.Info("Engine stopped")
}

func stopStopables(stopables []stopable) {
	for _, s := range stopables {
		s.stop()
	}
	allStopped := false
	for !allStopped {
		allStopped = true
		for _, s := range stopables {
			allStopped = allStopped && s.isStopped()
		}
	}
}

func (e *baseEngine) IsStopped() bool {
	return is(&e.stopped)
}

func (e *baseEngine) addRuleWithIface(rule DynamicRule, lockPattern string, callback interface{}, options ...RuleOption) {
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

func (e *v3Engine) AddPolling(namespacePattern string, preconditions DynamicRule, ttl int, callback V3RuleTaskCallback) error {
	if !strings.HasSuffix(namespacePattern, "/") {
		namespacePattern = namespacePattern + "/"
	}
	ttlPathPattern := namespacePattern + "ttl"
	ttlRule, err := NewEqualsLiteralRule(ttlPathPattern, nil)
	if err != nil {
		return err
	}
	rule := NewAndRule(preconditions, ttlRule)
	cbw := v3CallbackWrapper{
		callback:       callback,
		ttl:            ttl,
		ttlPathPattern: ttlPathPattern,
	}
	e.AddRule(rule, "/rule_locks"+namespacePattern+"lock", cbw.doRule)
	return nil
}

func (e *baseEngine) addRule(rule DynamicRule,
	lockPattern string,
	callback interface{},
	options ...RuleOption) {
	ruleIndex := e.ruleMgr.addRule(rule)
	opts := makeRuleOptions(options...)
	ttl := e.options.lockTimeout
	if opts.lockTimeout > 0 {
		ttl = opts.lockTimeout
	}
	contextProvider := opts.contextProvider
	if contextProvider == nil {
		contextProvider = e.options.contextProvider
	}
	e.ruleLockTTLs[ruleIndex] = ttl
	e.keyProc.setCallback(ruleIndex, callback)
	e.keyProc.setLockKeyPattern(ruleIndex, lockPattern)
	e.keyProc.setContextProvider(ruleIndex, contextProvider)
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
			e.baseEngine.keyProc,
			e.keysAPIWrapper,
		)
		if err1 != nil {
			e.logger.Fatal("Failed to initialize crawler", zap.String("prefix", prefix), zap.Error(err1))
		}
		e.crawlers = append(e.crawlers, c)
		go c.run()
		w, err := newWatcher(e.config, prefix, logger, e.baseEngine.keyProc, e.options.watchTimeout, e.keysAPIWrapper)
		if err != nil {
			e.logger.Fatal("Failed to initialize watcher", zap.String("prefix", prefix))
		}
		e.watchers = append(e.watchers, &w)
		go w.run()
	}

	for i := 0; i < e.options.concurrency; i++ {
		id := fmt.Sprintf("worker%d", i)
		w, err := newWorker(id, e)
		if err != nil {
			e.logger.Fatal("Failed to start worker", zap.String("worker", id), zap.Error(err))
		}
		e.workers = append(e.workers, &w)
		go w.run()
	}

}

func (e *v3Engine) Run() {
	prefixes := e.ruleMgr.prefixes
	// This is a map; used to ensure there are no duplicates
	for prefix := range prefixes {
		e.logger.Debug("Adding crawler", zap.String("prefix", prefix))
		logger := e.logger.With(zap.String("prefix", prefix))
		c, err1 := newV3Crawler(
			e.configV3,
			e.options.syncInterval,
			e.baseEngine.keyProc,
			logger,
			e.options.crawlMutex,
			e.options.crawlerTTL,
			prefix,
			e.kvWrapper,
		)
		if err1 != nil {
			e.logger.Fatal("Failed to initialize crawler", zap.String("prefix", prefix), zap.Error(err1))
		}
		e.crawlers = append(e.crawlers, c)
		go c.run()
		w, err := newV3Watcher(e.configV3, prefix, logger, e.baseEngine.keyProc, e.options.watchTimeout, e.kvWrapper)
		if err != nil {
			e.logger.Fatal("Failed to initialize watcher", zap.String("prefix", prefix))
		}
		e.watchers = append(e.watchers, &w)
		go w.run()
	}

	for i := 0; i < e.options.concurrency; i++ {
		id := fmt.Sprintf("worker%d", i)
		w, err := newV3Worker(id, e)
		if err != nil {
			e.logger.Fatal("Failed to start worker", zap.String("worker", id), zap.Error(err))
		}
		e.workers = append(e.workers, &w)
		go w.run()
	}

}

func (e *baseEngine) getLockTTLForRule(index int) int {
	return e.ruleLockTTLs[index]
}

type callbackWrapper struct {
	ttlPathPattern string
	callback       RuleTaskCallback
	ttl            int
}

type v3CallbackWrapper struct {
	ttlPathPattern string
	callback       V3RuleTaskCallback
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

func (cbw *v3CallbackWrapper) doRule(task *V3RuleTask) {
	logger := task.Logger
	cbw.callback(task)
	c, err := clientv3.New(*task.Conf)
	if err != nil {
		logger.Error("Error obtaining client", zap.Error(err))
		return
	}
	kv := clientv3.NewKV(c)
	path := task.Attr.Format(cbw.ttlPathPattern)
	logger.Debug("Setting polling TTL", zap.String("path", path))
	lease := clientv3.NewLease(c)
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Duration(5)*time.Second)
	defer cancelFunc()
	resp, leaseErr := lease.Grant(ctx, int64(cbw.ttl))
	if leaseErr != nil {
		logger.Error("Error obtaining lease", zap.Error(leaseErr), zap.String("path", path))
		return
	}
	ctx1, cancelFunc1 := context.WithTimeout(context.Background(), time.Duration(5)*time.Second)
	defer cancelFunc1()
	_, setErr := kv.Put(
		ctx1,
		path,
		"",
		clientv3.WithLease(resp.ID),
	)
	if setErr != nil {
		logger.Error("Error setting polling TTL", zap.Error(setErr), zap.String("path", path))
	}
}
