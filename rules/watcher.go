package rules

import (
	"github.com/coreos/etcd/client"
	"github.com/uber-go/zap"
	"golang.org/x/net/context"
)

func newWatcher(config client.Config, prefix string, logger zap.Logger, proc kProcessor) (watcher, error) {
	ec, err := client.New(config)
	if err != nil {
		return watcher{}, err
	}
	ea := client.NewKeysAPI(ec)
	api := etcdReadAPI{
		kAPI: ea,
	}
	ew := newEtcdKeyWatcher(ea, prefix)
	return watcher{
		api:    &api,
		kw:     ew,
		kp:     proc,
		logger: logger,
	}, nil
}

type watcher struct {
	api    readAPI
	kw     keyWatcher
	kp     kProcessor
	logger zap.Logger
}

func (w *watcher) run() {
	for {
		w.singleRun()
	}
}

func (w *watcher) singleRun() {
	key, value, err := w.kw.next()
	if err != nil {
		w.logger.Error("Watcher error", zap.Error(err))
		return
	}
	w.kp.processKey(key, value, w.api, w.logger)
}

func (w *watcher) getContext() context.Context {
	return context.Background()
}
