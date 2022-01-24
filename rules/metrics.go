package rules

import (
	"time"

	v3 "go.etcd.io/etcd/client/v3"
	"golang.org/x/net/context"
)

type contextKey string

func (c contextKey) String() string {
	return "rules context key " + string(c)
}

var contextKeyEtcdMetricsMetadata = contextKey("etcdMetricsMetadata")

// EtcdMetricsMetadata provides information about
// calls to etcd
type EtcdMetricsMetadata struct {
	Method   string
	Duration time.Duration
	Error    error
}

// SetMethod sets the method in the context of which an etcd call
// is being made, allowing metrics to differentiate between
// different types of calls to etcd.
func SetMethod(ctx context.Context, method string) context.Context {
	return context.WithValue(ctx, contextKeyEtcdMetricsMetadata,
		&EtcdMetricsMetadata{Method: method},
	)
}

// GetMetricsMetadata gets metadata about an etcd call from the context
func GetMetricsMetadata(ctx context.Context) *EtcdMetricsMetadata {
	out := ctx.Value(contextKeyEtcdMetricsMetadata)
	if md, ok := out.(*EtcdMetricsMetadata); ok {
		return md
	}
	return nil
}

// WrapKV is used to provide a wrapper for the default etcd v3 KV implementation
// used by the rules engine.
type WrapKV func(v3.KV) v3.KV

func defaultWrapKV(kv v3.KV) v3.KV {
	return kv
}

type WrapWatcher func(v3.Watcher) v3.Watcher

func defaultWrapWatcher(w v3.Watcher) v3.Watcher {
	return w
}
