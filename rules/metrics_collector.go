package rules

import "fmt"

// metricsCollector used for collecting metrics, implement this interface using
// your metrics collector of choice (ie Prometheus)
type metricsCollector interface {
	IncLockMetric(pattern string, lockSucceeded bool)
	IncSatisfiedThenNot(pattern string, phaseName string)
	TimesEvaluatedCount(ruleID string, count int)
}

// a no-op metrics collector, used as the default metrics collector
type defaultMetricsCollector struct {
}

func newMetricsCollector() metricsCollector {
	return &defaultMetricsCollector{}
}

func (m *defaultMetricsCollector) IncLockMetric(pattern string, lockSucceeded bool) {
}

// IncSatisfiedThenNot tracks rules that are satisfied initially then further along
// in processing are no longer true
func (m *defaultMetricsCollector) IncSatisfiedThenNot(pattern string, phaseName string) {
	// TODO - can we take in the context here?
	fmt.Printf("TrueThenEvalFalse: %s, %s\n", pattern, phaseName)
}

func (m *defaultMetricsCollector) TimesEvaluatedCount(ruleID string, count int) {
	fmt.Printf("TimesEvaluatedCount: %s, %d\n", ruleID, count)
}
