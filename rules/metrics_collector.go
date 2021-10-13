package rules

import (
	"context"
	"time"
)

const (
	notSetMethodName = "notSet"
)

// metricsInfo used for passing around information required for creating metrics
type metricsInfo struct {
	// the key pattern of the rule being processed
	keyPattern string
	// the calling method, retrieved from the context
	method string
	// holds a start time if duration is going to be calculated later
	startTime time.Time
}

func newMetricsInfo(ctx context.Context, keyPattern string) metricsInfo {
	methodName := "worker_lock"
	if data := GetMetricsMetadata(ctx); data != nil {
		methodName = data.Method
	}
	return metricsInfo{
		keyPattern: keyPattern,
		method:     methodName,
		startTime:  time.Now(),
	}
}

// MetricsCollector used for collecting metrics, implement this interface using
// your metrics collector of choice (ie Prometheus)
type MetricsCollector interface {
	IncLockMetric(methodName string, pattern string, lockSucceeded bool)
	IncSatisfiedThenNot(methodName string, pattern string, phaseName string)
	TimesEvaluated(methodName string, ruleID string, count int)
	WorkerQueueWaitTime(methodName string, startTime time.Time)
}

// AdvancedMetricsCollector used for collecting metrics additional metrics beyond those required by the base
// MetricsCollector, implement this interface using your metrics collector of choice (ie Prometheus)
// Deprecated: instead make use of the WrapWatcher to inject metric collection on watch events
type AdvancedMetricsCollector interface {
	MetricsCollector
	ObserveWatchEvents(prefix string, events, totalBytes int)
}

// a no-op metrics collector, used as the default metrics collector
type noOpMetricsCollector struct {
}

func newMetricsCollector() AdvancedMetricsCollector {
	return &noOpMetricsCollector{}
}

func (m *noOpMetricsCollector) IncLockMetric(methodName string, pattern string, lockSucceeded bool) {

}

// IncSatisfiedThenNot tracks rules that are satisfied initially then further along
// in processing are no longer true
func (m *noOpMetricsCollector) IncSatisfiedThenNot(methodName string, pattern string, phaseName string) {

}

func (m *noOpMetricsCollector) TimesEvaluated(methodName string, ruleID string, count int) {

}

func (m *noOpMetricsCollector) WorkerQueueWaitTime(methodName string, startTime time.Time) {

}

func (m *noOpMetricsCollector) ObserveWatchEvents(prefix string, events, totalBytes int) {

}

type advancedMetricsCollectorAdaptor struct {
	MetricsCollector
}

func (amca advancedMetricsCollectorAdaptor) ObserveWatchEvents(prefix string, events, totalBytes int) {
	// do nothing
}
