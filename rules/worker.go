package rules

import (
	"sync"
	"time"

	"github.com/IBM-Cloud/go-etcd-rules/metrics"
	"go.uber.org/zap"

	"github.com/IBM-Cloud/go-etcd-rules/rules/lock"
)

type callbackListener interface {
	callbackDone(ruleID string, attributes extendedAttributes)
}

type baseWorker struct {
	locker           lock.RuleLocker
	metrics          MetricsCollector
	api              readAPI
	workerID         string
	stopping         uint32
	stopped          uint32
	done             chan bool
	callbackListener callbackListener
}

type v3Worker struct {
	baseWorker
	engine *v3Engine
}

func newV3Worker(workerID string, engine *v3Engine) (v3Worker, error) {
	var api readAPI
	c := engine.cl
	kv := engine.kvWrapper(c)
	api = &etcdV3ReadAPI{
		kV: kv,
	}
	w := v3Worker{
		baseWorker: baseWorker{
			api:              api,
			locker:           engine.locker,
			metrics:          engine.metrics,
			workerID:         workerID,
			done:             make(chan bool, 1),
			callbackListener: engine.callbackListener,
		},
		engine: engine,
	}
	return w, nil
}

func (w *v3Worker) run() {
	atomicSet(&w.stopped, false)
	for !is(&w.stopping) {
		w.singleRun()
	}
	atomicSet(&w.stopped, true)
}

type workCallback func()

func (bw *baseWorker) stop() {
	atomicSet(&bw.stopping, true)
	bw.done <- true
}

func (bw *baseWorker) isStopped() bool {
	return is(&bw.stopped)
}

func (bw *baseWorker) doWork(loggerPtr **zap.Logger,
	rulePtr *staticRule, lockTTL int, callback workCallback,
	metricsInfo metricsInfo, lockKey string, ruleID string, source string) {
	logger := *loggerPtr
	logger = logger.With(zap.String("ruleID", ruleID), zap.String("mutex", lockKey))
	rule := *rulePtr
	capi, err1 := bw.api.getCachedAPI(rule.getKeys())
	if err1 != nil {
		logger.Error("Error querying for rule", zap.Error(err1))
		return
	}
	sat, err1 := rule.satisfied(capi)
	if err1 != nil {
		logger.Error("Error checking rule", zap.Error(err1))
		return
	}
	if !sat || is(&bw.stopping) {
		if !sat {
			metrics.IncSatisfiedThenNot(metricsInfo.method, metricsInfo.keyPattern, "worker.doWorkBeforeLock")
			bw.metrics.IncSatisfiedThenNot(metricsInfo.method, metricsInfo.keyPattern, "worker.doWorkBeforeLock")
		}
		return
	}
	l, err2 := bw.locker.Lock(lockKey, lock.PatternForLock(metricsInfo.keyPattern), lock.MethodForLock("worker_lock"))
	if err2 != nil {
		logger.Debug("Failed to acquire lock", zap.Error(err2))
		return
	}
	defer func() {
		err := l.Unlock()
		if err != nil {
			logger.Error("Could not unlock mutex", zap.Error(err))
		}
	}()
	// Check for a second time, since checking and locking
	// are not atomic.
	capi, err1 = bw.api.getCachedAPI(rule.getKeys())
	if err1 != nil {
		logger.Error("Error querying for rule", zap.Error(err1))
		return
	}
	sat, err1 = rule.satisfied(capi)
	if err1 != nil {
		logger.Error("Error checking rule", zap.Error(err1))
		return
	}
	if !sat {
		metrics.IncSatisfiedThenNot(metricsInfo.method, metricsInfo.keyPattern, "worker.doWorkAfterLock")
		bw.metrics.IncSatisfiedThenNot(metricsInfo.method, metricsInfo.keyPattern, "worker.doWorkAfterLock")
	}
	metrics.WorkerQueueWaitTime(metricsInfo.method, metricsInfo.startTime)
	bw.metrics.WorkerQueueWaitTime(metricsInfo.method, metricsInfo.startTime)
	if sat && !is(&bw.stopping) {
		attributes := (*rulePtr).getAttributes()
		attrMap := make(map[string]string)
		for _, attrName := range attributes.names() {
			attrMap[attrName] = *attributes.GetAttribute(attrName)
		}
		logger.Info("callback started", zap.Any("attributes", attrMap))
		startTime := time.Now()
		callback()
		metrics.CallbackWaitTime(metricsInfo.keyPattern, ruleID, source, startTime)
		if bw.callbackListener != nil {
			bw.callbackListener.callbackDone(ruleID, attributes)
		}
		logger.Info("callback complete", zap.Any("attributes", attrMap), zap.Duration("duration", time.Since(startTime)))
	}
}

func (bw *baseWorker) addWorkerID(ruleContext map[string]string) {
	ruleContext["rule_worker"] = bw.workerID
}

func (w *v3Worker) singleRun() {
	var work v3RuleWork
	var task V3RuleTask
	select {
	case <-w.done:
		return
	case work = <-w.engine.workChannel:
		task = work.ruleTask
	}
	if is(&w.stopping) {
		return
	}
	// Make a copy of the metadata to add the worker ID to.
	// It is unclear why, but this avoids a concurrent modification
	// panic.
	newMetadata := make(map[string]string)
	for k, v := range task.Metadata {
		newMetadata[k] = v
	}
	w.addWorkerID(newMetadata)
	task.Metadata = newMetadata
	source := task.Metadata["source"]
	task.Logger = task.Logger.With(zap.String("worker", w.workerID))
	// Use wait group and go routine to prevent killing of workers
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer func() {
			wg.Done()
			if r := recover(); r != nil {
				task.Logger.Error("Panic", zap.Any("recover", r), zap.Stack("stack"))
			}
		}()
		// Get/populate context for task as callback is started
		context, cancelFunc := work.contextProvider()
		task.Context = context
		task.cancel = cancelFunc
		metricsInfo := newMetricsInfo(context, work.keyPattern, work.metricsStartTime)
		w.doWork(&task.Logger, &work.rule, w.engine.getLockTTLForRule(work.ruleIndex), func() { work.ruleTaskCallback(&task) }, metricsInfo, work.lockKey, work.ruleID, source)
	}()
	wg.Wait()
}
