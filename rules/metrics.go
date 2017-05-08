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

type EtcdMetricsMetadata struct {
	Action, Method string
	Duration       time.Duration
	Error          error
}

func SetMethod(ctx context.Context, method string) context.Context {
	return context.WithValue(ctx, etcdMetricsMetadataKey,
		&EtcdMetricsMetadata{Method: method},
	)
}

func GetMetricsMetadata(ctx context.Context) *EtcdMetricsMetadata {
	out := ctx.Value(etcdMetricsMetadataKey)
	if md, ok := out.(*EtcdMetricsMetadata); ok {
		return md
	}
	return nil
}

type WrapKeysAPI func(client.KeysAPI) client.KeysAPI

func defaultWrapKeysAPI(keysAPI client.KeysAPI) client.KeysAPI {
	return keysAPI
}

type WrapKV func(clientv3.KV) clientv3.KV

func defaultWrapKV(kv clientv3.KV) clientv3.KV {
	return kv
}
