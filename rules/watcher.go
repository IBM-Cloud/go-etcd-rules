package rules

import (
	"strings"
	"time"

	"go.etcd.io/etcd/clientv3"
	"go.uber.org/zap"
)

func newV3Watcher(ec *clientv3.Client, prefix string, logger *zap.Logger, proc keyProc, watchTimeout int, kvWrapper WrapKV, metrics AdvancedMetricsCollector, watcherWrapper WrapWatcher) (watcher, error) {
	api := etcdV3ReadAPI{
		kV: kvWrapper(ec),
	}
	ew := newEtcdV3KeyWatcher(watcherWrapper(clientv3.NewWatcher(ec)), prefix, time.Duration(watchTimeout)*time.Second, metrics)
	return watcher{
		api:    &api,
		kw:     ew,
		kp:     proc,
		logger: logger,
	}, nil
}

type watcher struct {
	api      readAPI
	kw       keyWatcher
	kp       keyProc
	logger   *zap.Logger
	stopping uint32
	stopped  uint32
}

func (w *watcher) run() {
	atomicSet(&w.stopped, false)
	for !is(&w.stopping) {
		w.singleRun()
	}
	atomicSet(&w.stopped, true)
}

func (w *watcher) stop() {
	atomicSet(&w.stopping, true)
	w.kw.cancel()
}

func (w *watcher) isStopped() bool {
	return is(&w.stopped)
}

func (w *watcher) singleRun() {
	key, value, err := w.kw.next()
	if err != nil {
		w.logger.Error("Watcher error", zap.Error(err))
		if strings.Contains(err.Error(), "connection refused") {
			w.logger.Info("Cluster unavailable; waiting one minute to retry")
			time.Sleep(time.Minute)
		} else {
			// Maximum logging rate is 1 per second.
			time.Sleep(time.Second)
		}
		return
	}
	w.logger.Debug("Calling process key", zap.String("key", key))
	w.kp.processKey(key, value, w.api, w.logger, map[string]string{}, nil)
}
