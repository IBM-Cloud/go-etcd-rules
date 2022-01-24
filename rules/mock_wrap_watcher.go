package rules

import (
	"context"

	v3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

type MockWatcherWrapper struct {
	Logger    *zap.Logger
	Responses []v3.WatchResponse
	KvWatcher v3.Watcher
}

func (ww *MockWatcherWrapper) Watch(ctx context.Context, key string, opts ...v3.OpOption) v3.WatchChan {
	c := make(chan v3.WatchResponse)
	watcherChan := ww.KvWatcher.Watch(ctx, key, opts...)
	ww.Logger.Info("initiating watch", zap.String("key", key))
	go func() {
		for {
			watcherResp := <-watcherChan
			ww.Logger.Info("watch response received", zap.String("resp", key))
			ww.Responses = append(ww.Responses, watcherResp)
			c <- watcherResp
		}
	}()

	return c
}

func (ww *MockWatcherWrapper) RequestProgress(ctx context.Context) error {
	return ww.KvWatcher.RequestProgress(ctx)
}

func (ww *MockWatcherWrapper) Close() error {
	return ww.KvWatcher.Close()
}

type MockWatchWrapper struct {
	Mww *MockWatcherWrapper
}

func (mw *MockWatchWrapper) WrapWatcher(kvw v3.Watcher) v3.Watcher {
	mw.Mww.KvWatcher = kvw
	return mw.Mww
}
