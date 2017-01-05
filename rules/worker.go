package rules

import (
	"github.com/IBM-Bluemix/go-etcd-lock/lock"
	"github.com/coreos/etcd/client"
	"github.com/uber-go/zap"
)

type worker struct {
	engine   *engine
	locker   lock.Locker
	api      readAPI
	workerID string
}

func newWorker(workerID string, engine *engine, config client.Config) (worker, error) {
	c, err := client.New(config)
	if err != nil {
		return worker{}, err
	}
	locker := lock.NewEtcdLocker(c, false)
	api := etcdReadAPI{
		kAPI: client.NewKeysAPI(c),
	}
	w := worker{
		engine:   engine,
		api:      &api,
		locker:   locker,
		workerID: workerID,
	}
	return w, nil
}

func (w *worker) run() {
	for {
		w.singleRun()
	}
}

func (w *worker) singleRun() {
	wrkr := *w
	work := <-wrkr.engine.workChannel
	task := work.ruleTask
	logger := task.Logger
	logger = logger.With(zap.String("worker", w.workerID))
	task.Logger = logger
	sat, err1 := work.rule.satisfied(wrkr.api)
	if err1 != nil {
		logger.Error("Error checking rule", zap.Error(err1))
		return
	}
	if !sat {
		return
	}
	l, err2 := wrkr.locker.Acquire(work.lockKey, wrkr.engine.getLockTTLForRule(work.ruleIndex))
	if err2 != nil {
		logger.Error("Failed to acquire lock", zap.String("lock_key", work.lockKey), zap.Error(err2))
		return
	}
	defer l.Release()
	// Check for a second time, since checking and locking
	// are not atomic.
	sat, err1 = work.rule.satisfied(wrkr.api)
	if err1 != nil {
		logger.Error("Error checking rule", zap.Error(err1))
		return
	}
	if sat {
		work.ruleTaskCallback(&task)
	}
}
