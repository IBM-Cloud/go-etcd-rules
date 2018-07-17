package rules

import (
	"fmt"
	"strings"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/uber-go/zap"
	"golang.org/x/net/context"
)

type stopable interface {
	stop()
	isStopped() bool
}

// BaseEngine provides common method for etcd v2 and v3 rules engine instances.
type BaseEngine interface {
	Run()
	Stop()
	IsStopped() bool

	// Shutdown gracefully stops the rules engine and waits for termination to
	// complete. If the provided context expires before the shutdown is complete,
	// then the context's error is returned.
	Shutdown(ctx context.Context) error
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

type v3Engine struct {
	baseEngine
	configV3    clientv3.Config
	keyProc     v3KeyProcessor
	workChannel chan v3RuleWork
	kvWrapper   WrapKV
	cl          *clientv3.Client
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

// NewV3Engine creates a new V3Engine instance.
func NewV3Engine(configV3 clientv3.Config, logger zap.Logger, options ...EngineOption) V3Engine {
	cl, err := clientv3.New(configV3)
	if err != nil {
		logger.Fatal("Failed to connect to etcd", zap.Error(err))
	}
	return NewV3EngineWithClient(cl, configV3, logger, options...)
}

// NewV3EngineWithClient creates a new V3Engine instance with the provided etcd v3 client instance.
func NewV3EngineWithClient(cl *clientv3.Client, configV3 clientv3.Config, logger zap.Logger, options ...EngineOption) V3Engine {
	eng := newV3Engine(configV3, true, logger, cl, options...)
	return &eng
}

func newV3Engine(configV3 clientv3.Config, useV3 bool, logger zap.Logger, cl *clientv3.Client, options ...EngineOption) v3Engine {
	opts := makeEngineOptions(options...)
	ruleMgr := newRuleManager(opts.constraints, opts.enhancedRuleFilter)
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
		cl:          cl,
	}
	return eng
}

func (e *v3Engine) SetKVWrapper(kvWrapper WrapKV) {
	e.kvWrapper = kvWrapper
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

var shutdownPollInterval = 500 * time.Millisecond

func (e *baseEngine) Shutdown(ctx context.Context) error {
	e.logger.Info("Shutting down engine")
	go e.stop()

	ticker := time.NewTicker(shutdownPollInterval)
	defer ticker.Stop()
	for {
		if e.IsStopped() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
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
		rules, _ := rule.Expand(e.options.keyExpansion)
		for _, expRule := range rules {
			e.addRule(expRule, lockPattern, callback, options...)
		}
	} else {
		e.addRule(rule, lockPattern, callback, options...)
	}
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
		kv:             e.kvWrapper(e.cl),
		lease:          e.cl,
		engine:         e,
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

func (e *v3Engine) Run() {
	prefixSlice := []string{}
	prefixes := e.ruleMgr.prefixes
	// This is a map; used to ensure there are no duplicates
	for prefix := range prefixes {
		prefixSlice = append(prefixSlice, prefix)
		logger := e.logger.With(zap.String("prefix", prefix))
		w, err := newV3Watcher(e.cl, prefix, logger, e.baseEngine.keyProc, e.options.watchTimeout, e.kvWrapper)
		if err != nil {
			e.logger.Fatal("Failed to initialize watcher", zap.String("prefix", prefix))
		}
		e.watchers = append(e.watchers, &w)
		go w.run()
	}
	logger := e.logger
	c, err := newIntCrawler(e.cl,
		e.options.syncInterval,
		e.baseEngine.keyProc,
		logger,
		e.options.crawlMutex,
		e.options.crawlerTTL,
		prefixSlice,
		e.kvWrapper,
		e.options.syncDelay)
	if err != nil {
		e.logger.Fatal("Failed to initialize crawler", zap.Error(err))
	}
	e.crawlers = append(e.crawlers, c)
	go c.run()

	e.logger.Info("Starting workers", zap.Int("count", e.options.concurrency))
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

type v3CallbackWrapper struct {
	ttlPathPattern string
	callback       V3RuleTaskCallback
	ttl            int
	kv             clientv3.KV
	lease          clientv3.Lease
	engine         *v3Engine
}

func (cbw *v3CallbackWrapper) doRule(task *V3RuleTask) {
	logger := task.Logger
	cbw.callback(task)
	path := task.Attr.Format(cbw.ttlPathPattern)
	logger.Debug("Setting polling TTL", zap.String("path", path))
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Duration(5)*time.Second)
	defer cancelFunc()
	resp, leaseErr := cbw.lease.Grant(ctx, int64(cbw.ttl))
	if leaseErr != nil {
		logger.Error("Error obtaining lease", zap.Error(leaseErr), zap.String("path", path))
		return
	}
	ctx1, cancelFunc1 := context.WithTimeout(context.Background(), time.Duration(5)*time.Second)
	defer cancelFunc1()
	_, setErr := cbw.kv.Put(
		ctx1,
		path,
		"",
		clientv3.WithLease(resp.ID),
	)
	if setErr != nil {
		logger.Error("Error setting polling TTL", zap.Error(setErr), zap.String("path", path))
	}
}
