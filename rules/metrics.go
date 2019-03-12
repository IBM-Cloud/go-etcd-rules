package rules

import (
	"time"

	"fmt"
	"github.com/coreos/etcd/clientv3"
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
type WrapKV func(clientv3.KV) clientv3.KV

func defaultWrapKV(kv clientv3.KV) clientv3.KV {
	return kv
}

// metricsCollector used for collecting metrics, implement this interface using
// your metrics collector of choice (ie Prometheus)
type metricsCollector interface {
	incLockMetric(path string, lockSucceeded bool)
}

// a no-op metrics collector, used as the default metrics collector
type metricsCollectorImpl struct {
}

func newMetricsCollector() metricsCollector {
	return &metricsCollectorImpl{}
}

func (m *metricsCollectorImpl) incLockMetric(path string, lockSucceeded bool) {
	// NOTE - this is just for testing, this will be cleared to no-op
	fmt.Printf("%s, %t\n", path, lockSucceeded)
}
