package rules

import (
	"errors"
	"strings"
	"time"

	"github.com/IBM-Cloud/go-etcd-rules/internal/jitter"
	"github.com/IBM-Cloud/go-etcd-rules/metrics"
	v3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

func newV3Watcher(ec *v3.Client, prefix string, logger *zap.Logger, proc keyProc, watchTimeout int, kvWrapper WrapKV, metrics AdvancedMetricsCollector, watcherWrapper WrapWatcher, processDelay jitter.DurationGenerator) (watcher, error) {
	api := etcdV3ReadAPI{
		kV: kvWrapper(ec),
	}
	ew := newEtcdV3KeyWatcher(watcherWrapper(v3.NewWatcher(ec)), prefix, time.Duration(watchTimeout)*time.Second, metrics)
	return watcher{
		api:          &api,
		kw:           ew,
		kp:           proc,
		logger:       logger.With(zap.String("source", "watcher")),
		processDelay: processDelay,
	}, nil
}

type watcher struct {
	api          readAPI
	kw           keyWatcher
	kp           keyProc
	logger       *zap.Logger
	processDelay jitter.DurationGenerator
	stopping     uint32
	stopped      uint32
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
		if strings.Contains(err.Error(), "connection refused") {
			w.logger.Info("Cluster unavailable; waiting one minute to retry")
			time.Sleep(time.Minute)
		} else {
			// Watcher are always closed periodically, no need to log that
			if !errors.Is(err, ErrWatcherClosing) {
				w.logger.Error("Watcher error", zap.Error(err))
			}
			// Maximum logging rate is 1 per second.
			time.Sleep(time.Second)
		}
		return
	}
	delay := w.processDelay.Generate()
	if delay > 0 {
		w.logger.Debug("Pausing before processing next key", zap.Duration("wait_time", delay))
		time.Sleep(delay) // TODO ideally a context should be used for fast shutdown, e.g. select { case <-ctx.Done(); case <-time.After(delay) }
	}
	w.logger.Debug("Calling process key", zap.String("key", key))
	w.kp.processKey(key, value, w.api, w.logger, map[string]string{sourceSource: sourceWatcher}, incRuleProcessedCount)
}

func incRuleProcessedCount(ruleID string) {
	metrics.TimesEvaluated("watcher", ruleID, 1)
}
