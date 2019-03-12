package rules

import "fmt"

// a mock metrics collector used in unit tests
type mockMetricsCollector struct {
	// store what the IncLockMetric function was called with
	IncLockMetricPattern     []string
	IncLockMetricLockSuccess []bool
}

func newMockMetricsCollector() metricsCollector {
	return &mockMetricsCollector{}
}

func (m *mockMetricsCollector) IncLockMetric(pattern string, lockSucceeded bool) {
	m.IncLockMetricPattern = append(m.IncLockMetricPattern, pattern)
	m.IncLockMetricLockSuccess = append(m.IncLockMetricLockSuccess, lockSucceeded)
	fmt.Printf("%s, %t\n", pattern, lockSucceeded)
}
