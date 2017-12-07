package rules

import (
	"time"

	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/clientv3"
	"golang.org/x/net/context"
)

const (
	etcdMetricsMetadataKey = "etcdMetricsMetadata"
)

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
	return context.WithValue(ctx, etcdMetricsMetadataKey,
		&EtcdMetricsMetadata{Method: method},
	)
}

// GetMetricsMetadata gets metadata about an etcd call from the context
func GetMetricsMetadata(ctx context.Context) *EtcdMetricsMetadata {
	out := ctx.Value(etcdMetricsMetadataKey)
	if md, ok := out.(*EtcdMetricsMetadata); ok {
		return md
	}
	return nil
}

// WrapKeysAPI is used to provide a wrapper for the default KeysAPI used
// by the rules engine.
type WrapKeysAPI func(client.KeysAPI) client.KeysAPI

func defaultWrapKeysAPI(keysAPI client.KeysAPI) client.KeysAPI {
	return keysAPI
}

// WrapKV is used to provide a wrapper for the default etcd v3 KV implementation
// used by the rules engine.
type WrapKV func(clientv3.KV) clientv3.KV

func defaultWrapKV(kv clientv3.KV) clientv3.KV {
	return kv
}
