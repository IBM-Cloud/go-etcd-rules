package rules

import (
	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/clientv3"
	"github.com/uber-go/zap"
)

type baseWorker struct {
	locker   ruleLocker
	api      readAPI
	workerID string
	stopping uint32
	stopped  uint32
}

type worker struct {
	baseWorker
	engine *engine
}

type v3Worker struct {
	baseWorker
	engine *v3Engine
}

func newWorker(workerID string, engine *engine) (worker, error) {
	var api readAPI
	var locker ruleLocker
	c, err := client.New(engine.config)
	if err != nil {
		return worker{}, err
	}
	locker = newLockLocker(c)
	api = &etcdReadAPI{
		keysAPI: client.NewKeysAPI(c),
	}
	w := worker{
		baseWorker: baseWorker{
			api:      api,
			locker:   locker,
			workerID: workerID,
		},
		engine: engine,
	}
	return w, nil
}

func newV3Worker(workerID string, engine *v3Engine) (v3Worker, error) {
	var api readAPI
	var locker ruleLocker

	c, err := clientv3.New(engine.configV3)
	if err != nil {
		return v3Worker{}, err
	}
	locker = newV3Locker(c)
	api = &etcdV3ReadAPI{
		kV: c,
	}
	w := v3Worker{
		baseWorker: baseWorker{
			api:      api,
			locker:   locker,
			workerID: workerID,
		},
		engine: engine,
	}
	return w, nil
}

func (w *worker) run() {
	atomicSet(&w.stopped, false)
	for !is(&w.stopping) {
		w.singleRun()
	}
	atomicSet(&w.stopped, true)
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
}

func (bw *baseWorker) isStopped() bool {
	return is(&bw.stopped)
}

func (bw *baseWorker) doWork(loggerPtr *zap.Logger,
	rulePtr *staticRule, lockTTL int, callback workCallback,
	lockKey string) {
	logger := *loggerPtr
	rule := *rulePtr
	sat, err1 := rule.satisfied(bw.api)
	if err1 != nil {
		logger.Error("Error checking rule", zap.Error(err1))
		return
	}
	if !sat || is(&bw.stopping) {
		return
	}
	l, err2 := bw.locker.lock(lockKey, lockTTL)
	if err2 != nil {
		logger.Error("Failed to acquire lock", zap.String("lock_key", lockKey), zap.Error(err2))
		return
	}
	defer l.unlock()
	// Check for a second time, since checking and locking
	// are not atomic.
	sat, err1 = rule.satisfied(bw.api)
	if err1 != nil {
		logger.Error("Error checking rule", zap.Error(err1))
		return
	}
	if sat && !is(&bw.stopping) {
		callback()
	}
}

func (bw *baseWorker) addWorkerID(ruleContext map[string]string) {
	ruleContext["rule_worker"] = bw.workerID
}

func (w *worker) singleRun() {
	work := <-w.engine.workChannel
	task := work.ruleTask
	if is(&w.stopping) {
		return
	}
	w.addWorkerID(task.Metadata)
	task.Logger = task.Logger.With(zap.String("worker", w.workerID))
	w.doWork(&task.Logger, &work.rule, w.engine.getLockTTLForRule(work.ruleIndex), func() { work.ruleTaskCallback(&task) }, work.lockKey)
}

func (w *v3Worker) singleRun() {
	work := <-w.engine.workChannel
	task := work.ruleTask
	if is(&w.stopping) {
		return
	}
	w.addWorkerID(task.Metadata)
	task.Logger = task.Logger.With(zap.String("worker", w.workerID))
	w.doWork(&task.Logger, &work.rule, w.engine.getLockTTLForRule(work.ruleIndex), func() { work.ruleTaskCallback(&task) }, work.lockKey)
}
