package rules

import "fmt"

// a mock metrics collector used in unit tests
type mockMetricsCollector struct {
	// store what the IncLockMetric function was called with
	incLockMetricPattern     []string
	incLockMetricLockSuccess []bool
	// store what the IncSatisfiedThenNot function was called with
	incSatisfiedThenNotPattern  []string
	incIncSatisfiedThenNotPhase []string
}

func newMockMetricsCollector() mockMetricsCollector {
	return mockMetricsCollector{}
}

func (m *mockMetricsCollector) IncLockMetric(pattern string, lockSucceeded bool) {
	m.incLockMetricPattern = append(m.incLockMetricPattern, pattern)
	m.incLockMetricLockSuccess = append(m.incLockMetricLockSuccess, lockSucceeded)
	fmt.Printf("IncLockMetric - %s, %t\n", pattern, lockSucceeded)
}

func (m *mockMetricsCollector) IncSatisfiedThenNot(pattern string, phaseName string) {
	m.incSatisfiedThenNotPattern = append(m.incSatisfiedThenNotPattern, pattern)
	m.incIncSatisfiedThenNotPhase = append(m.incIncSatisfiedThenNotPhase, phaseName)
	fmt.Printf("IncSatisfiedThenNot - %s, %s\n", pattern, phaseName)
}
