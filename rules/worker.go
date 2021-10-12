package rules

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

type baseWorker struct {
	locker   ruleLocker
	metrics  MetricsCollector
	api      readAPI
	workerID string
	stopping uint32
	stopped  uint32
	done     chan bool
}

type v3Worker struct {
	baseWorker
	engine *v3Engine
}

func newV3Worker(workerID string, engine *v3Engine) (v3Worker, error) {
	var api readAPI
	// var locker ruleLocker
	c := engine.cl
	kv := engine.kvWrapper(c)
	// locker = newV3Locker(c, engine.options.lockAcquisitionTimeout)
	api = &etcdV3ReadAPI{
		kV: kv,
	}
	w := v3Worker{
		baseWorker: baseWorker{
			api:      api,
			locker:   engine.locker,
			metrics:  engine.metrics,
			workerID: workerID,
			done:     make(chan bool, 1),
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
	metricsInfo metricsInfo, lockKey string) {
	logger := *loggerPtr
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
			incSatisfiedThenNot(metricsInfo.method, metricsInfo.keyPattern, "worker.doWorkBeforeLock")
			bw.metrics.IncSatisfiedThenNot(metricsInfo.method, metricsInfo.keyPattern, "worker.doWorkBeforeLock")
		}
		return
	}
	l, err2 := bw.locker.lock(lockKey)
	if err2 != nil {
		logger.Debug("Failed to acquire lock", zap.String("lock_key", lockKey), zap.Error(err2))
		incLockMetric(metricsInfo.method, metricsInfo.keyPattern, false)
		bw.metrics.IncLockMetric(metricsInfo.method, metricsInfo.keyPattern, false)
		return
	}
	incLockMetric(metricsInfo.method, metricsInfo.keyPattern, true)
	bw.metrics.IncLockMetric(metricsInfo.method, metricsInfo.keyPattern, true)
	defer func() { _ = l.unlock() }()
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
		incSatisfiedThenNot(metricsInfo.method, metricsInfo.keyPattern, "worker.doWorkAfterLock")
		bw.metrics.IncSatisfiedThenNot(metricsInfo.method, metricsInfo.keyPattern, "worker.doWorkAfterLock")
	}
	workerQueueWaitTime(metricsInfo.method, metricsInfo.startTime)
	bw.metrics.WorkerQueueWaitTime(metricsInfo.method, metricsInfo.startTime)
	if sat && !is(&bw.stopping) {
		startTime := time.Now()
		callback()
		callbackWaitTime(metricsInfo.keyPattern, startTime)
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
	w.addWorkerID(task.Metadata)
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
		w.doWork(&task.Logger, &work.rule, w.engine.getLockTTLForRule(work.ruleIndex), func() { work.ruleTaskCallback(&task) }, work.metricsInfo, work.lockKey)
	}()
	wg.Wait()
}
