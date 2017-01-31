package rules

import (
	"time"

	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/clientv3"
	"github.com/uber-go/zap"
)

func newWatcher(config client.Config, prefix string, logger zap.Logger, proc keyProc, watchTimeout int) (watcher, error) {
	ec, err := client.New(config)
	if err != nil {
		return watcher{}, err
	}
	ea := client.NewKeysAPI(ec)
	api := etcdReadAPI{
		baseReadAPI: baseReadAPI{},
		keysAPI:     ea,
	}
	ew := newEtcdKeyWatcher(ea, prefix, time.Duration(watchTimeout)*time.Second)
	return watcher{
		api:    &api,
		kw:     ew,
		kp:     proc,
		logger: logger,
	}, nil
}

func newV3Watcher(config clientv3.Config, prefix string, logger zap.Logger, proc keyProc, watchTimeout int) (watcher, error) {
	ec, err := clientv3.New(config)
	if err != nil {
		return watcher{}, err
	}
	api := etcdV3ReadAPI{
		baseReadAPI: baseReadAPI{},
		kV:          ec,
	}
	ew := newEtcdV3KeyWatcher(ec, prefix, time.Duration(watchTimeout)*time.Second)
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
	logger   zap.Logger
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
		return
	}
	w.kp.processKey(key, value, w.api, w.logger)
}
