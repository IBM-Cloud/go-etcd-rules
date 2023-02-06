package rules

import (
	"fmt"
	"os"
	"strings"
	"time"

	v3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"golang.org/x/net/context"

	"github.com/IBM-Cloud/go-etcd-rules/concurrency"
	"github.com/IBM-Cloud/go-etcd-rules/rules/lock"
)

const (
	// WebhookURLEnv is the environment variable used to specify a callback
	// webhook that will get called every time a callback has finished executing.
	WebhookURLEnv = "RULES_ENGINE_CALLBACK_WEBHOOK"
)

type stoppable interface {
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
	keyProc          setableKeyProcessor
	metrics          AdvancedMetricsCollector
	logger           *zap.Logger
	options          engineOptions
	ruleLockTTLs     map[int]int
	ruleMgr          ruleManager
	stopped          uint32
	crawlers         []stoppable
	watchers         []stoppable
	workers          []stoppable
	locker           lock.RuleLocker
	callbackListener callbackListener
}

type v3Engine struct {
	baseEngine
	keyProc        v3KeyProcessor
	workChannel    chan v3RuleWork
	kvWrapper      WrapKV
	watcherWrapper WrapWatcher
	cl             *v3.Client
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
	SetWatcherWrapper(WrapWatcher)
}

// NewV3Engine creates a new V3Engine instance.
func NewV3Engine(configV3 v3.Config, logger *zap.Logger, options ...EngineOption) V3Engine {
	cl, err := v3.New(configV3)
	if err != nil {
		logger.Fatal("Failed to connect to etcd", zap.Error(err))
	}
	return NewV3EngineWithClient(cl, logger, options...)
}

// NewV3EngineWithClient creates a new V3Engine instance with the provided etcd v3 client instance.
func NewV3EngineWithClient(cl *v3.Client, logger *zap.Logger, options ...EngineOption) V3Engine {
	eng := newV3Engine(logger, cl, options...)
	return &eng
}

func newV3Engine(logger *zap.Logger, cl *v3.Client, options ...EngineOption) v3Engine {
	opts := makeEngineOptions(options...)
	ruleMgr := newRuleManager(opts.constraints, opts.enhancedRuleFilter)
	channel := make(chan v3RuleWork, opts.ruleWorkBuffer)
	kpChannel := make(chan *keyTask, opts.keyProcBuffer)
	keyProc := newV3KeyProcessor(channel, &ruleMgr, kpChannel, opts.keyProcConcurrency, logger)

	baseMetrics := opts.metrics()
	metrics, ok := baseMetrics.(AdvancedMetricsCollector)
	if !ok {
		metrics = advancedMetricsCollectorAdaptor{
			MetricsCollector: baseMetrics,
		}
	}
	var cbListener callbackListener
	// Should be used for system testing only
	if cblURL, ok := os.LookupEnv(WebhookURLEnv); ok {
		cbListener = httpCallbackListener{
			hookURL: cblURL,
			logger:  logger,
		}
	}
	var baseEtcdLocker lock.RuleLocker
	if opts.useSharedLockSession {
		sessionManager := concurrency.NewSessionManager(cl, logger)
		baseEtcdLocker = lock.NewSessionLocker(sessionManager.GetSession, opts.lockAcquisitionTimeout, false, opts.useTryLock)
	} else {
		baseEtcdLocker = lock.NewV3Locker(cl, opts.lockAcquisitionTimeout, opts.useTryLock)
	}
	metricsEtcdLocker := lock.WithMetrics(baseEtcdLocker, "etcd")
	var baseLocker lock.RuleLocker
	if opts.useSharedLockSession {
		baseMapLocker := lock.NewMapLocker()
		metricsMapLocker := lock.WithMetrics(baseMapLocker, "map")
		baseLocker = lock.NewNestedLocker(metricsMapLocker, metricsEtcdLocker)
	} else {
		baseLocker = metricsEtcdLocker
	}
	var finalLocker lock.RuleLocker
	if opts.lockCoolOff == 0 {
		finalLocker = baseLocker
	} else {
		coolOffLocker := lock.NewCoolOffLocker(opts.lockCoolOff)
		metricsCoolOffLocker := lock.WithMetrics(coolOffLocker, "cooloff")
		finalLocker = lock.NewNestedLocker(metricsCoolOffLocker, baseLocker)
	}
	eng := v3Engine{
		baseEngine: baseEngine{
			keyProc:          &keyProc,
			metrics:          metrics,
			logger:           logger,
			options:          opts,
			ruleLockTTLs:     map[int]int{},
			ruleMgr:          ruleMgr,
			locker:           finalLocker,
			callbackListener: cbListener,
		},
		keyProc:        keyProc,
		workChannel:    channel,
		kvWrapper:      defaultWrapKV,
		watcherWrapper: defaultWrapWatcher,
		cl:             cl,
	}
	return eng
}

func (e *v3Engine) SetKVWrapper(kvWrapper WrapKV) {
	e.kvWrapper = kvWrapper
}

func (e *v3Engine) SetWatcherWrapper(watcherWrapper WrapWatcher) {
	e.watcherWrapper = watcherWrapper
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
	stopstoppables(e.crawlers)
	e.logger.Debug("Stopping watchers")
	stopstoppables(e.watchers)
	e.logger.Debug("Stopping workers")
	for _, worker := range e.workers {
		worker.stop()
	}
	// Ensure workers are stopped; the "stop" method is called again, but
	// that is idempotent.  The workers' "stop" method must be called before
	// the channel is closed in order to avoid nil pointer dereference panics.
	e.logger.Debug("Stopping workers")
	stopstoppables(e.workers)
	atomicSet(&e.stopped, true)
	e.logger.Info("Engine stopped")
}

func stopstoppables(stoppables []stoppable) {
	for _, s := range stoppables {
		s.stop()
	}
	allStopped := false
	for !allStopped {
		allStopped = true
		for _, s := range stoppables {
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
	e.AddRule(rule, "/rule_locks"+namespacePattern+"lock", cbw.doRule, RuleID(namespacePattern))
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
	if opts.ruleID == defaultRuleID {
		panic("Rule ID option missing")
	}
	ruleID := opts.ruleID
	e.ruleLockTTLs[ruleIndex] = ttl
	e.keyProc.setCallback(ruleIndex, callback)
	e.keyProc.setLockKeyPattern(ruleIndex, lockPattern)
	e.keyProc.setContextProvider(ruleIndex, contextProvider)
	e.keyProc.setRuleID(ruleIndex, ruleID)
}

func (e *v3Engine) Run() {
	prefixSlice := []string{}
	prefixes := e.ruleMgr.prefixes
	// This is a map; used to ensure there are no duplicates
	for prefix := range prefixes {
		prefixSlice = append(prefixSlice, prefix)
		logger := e.logger.With(zap.String("prefix", prefix))
		w, err := newV3Watcher(e.cl, prefix, logger, e.baseEngine.keyProc, e.options.watchTimeout, e.kvWrapper, e.metrics, e.watcherWrapper, e.options.watchDelay)
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
		e.metrics,
		logger,
		e.options.crawlMutex,
		e.options.lockAcquisitionTimeout,
		prefixSlice,
		e.kvWrapper,
		e.options.syncDelay,
		e.locker,
	)
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
	kv             v3.KV
	lease          v3.Lease
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
		v3.WithLease(resp.ID),
	)
	if setErr != nil {
		logger.Error("Error setting polling TTL", zap.Error(setErr), zap.String("path", path))
	}
}
