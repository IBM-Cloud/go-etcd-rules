package rules

import "fmt"

// metricsCollector used for collecting metrics, implement this interface using
// your metrics collector of choice (ie Prometheus)
type metricsCollector interface {
	IncLockMetric(pattern string, lockSucceeded bool)
}

// a no-op metrics collector, used as the default metrics collector
type defaultMetricsCollector struct {
}

func newMetricsCollector() metricsCollector {
	return &defaultMetricsCollector{}
}

func (m *defaultMetricsCollector) IncLockMetric(pattern string, lockSucceeded bool) {
	// TODO - can we take in the context here?
	// NOTE - this is just for testing, this will be cleared to no-op
	fmt.Printf("%s, %t\n", pattern, lockSucceeded)
}
