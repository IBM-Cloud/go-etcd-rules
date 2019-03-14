package rules

import "fmt"

// metricsCollector used for collecting metrics, implement this interface using
// your metrics collector of choice (ie Prometheus)
type MetricsCollector interface {
	IncLockMetric(pattern string, lockSucceeded bool)
	IncSatisfiedThenNot(pattern string, phaseName string)
	TimesEvaluatedCount(ruleID string, count int)
}

// a no-op metrics collector, used as the default metrics collector
type noOpMetricsCollector struct {
}

func newMetricsCollector() MetricsCollector {
	return &noOpMetricsCollector{}
}

func (m *noOpMetricsCollector) IncLockMetric(pattern string, lockSucceeded bool) {
}

// IncSatisfiedThenNot tracks rules that are satisfied initially then further along
// in processing are no longer true
func (m *noOpMetricsCollector) IncSatisfiedThenNot(pattern string, phaseName string) {
	// TODO - can we take in the context here?
	fmt.Printf("TrueThenEvalFalse: %s, %s\n", pattern, phaseName)
}

func (m *noOpMetricsCollector) TimesEvaluatedCount(ruleID string, count int) {
	fmt.Printf("TimesEvaluatedCount: %s, %d\n", ruleID, count)
}
