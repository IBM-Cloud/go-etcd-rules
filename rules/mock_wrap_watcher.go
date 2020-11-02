package rules

import (
	"context"
	"github.com/coreos/etcd/clientv3"
	"go.uber.org/zap"
)

type MockWatcherWrapper struct {
	Logger    *zap.Logger
	Responses []clientv3.WatchResponse
	KvWatcher clientv3.Watcher
}

func (ww *MockWatcherWrapper) Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
	c := make(chan clientv3.WatchResponse)
	watcherChan := ww.KvWatcher.Watch(ctx, key, opts...)
	ww.Logger.Info("initiating watch", zap.String("key", key))
	go func() {
		watcherResp := <-watcherChan
		ww.Logger.Info("watch response received", zap.String("resp", key))
		ww.Responses = append(ww.Responses, watcherResp)
		c <- watcherResp
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

func (mw *MockWatchWrapper) WrapWatcher(kvw clientv3.Watcher) clientv3.Watcher {
	mw.Mww.KvWatcher = kvw
	return mw.Mww
}
