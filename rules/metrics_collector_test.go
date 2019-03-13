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
	// store what the TimesEvaluatedCount function was called with
	timesEvaluatedRuleID []string
	timesEvaluatedCount  []int
}

func newMockMetricsCollector() mockMetricsCollector {
	return mockMetricsCollector{}
}

func (m *mockMetricsCollector) IncLockMetric(pattern string, lockSucceeded bool) {
	m.incLockMetricPattern = append(m.incLockMetricPattern, pattern)
	m.incLockMetricLockSuccess = append(m.incLockMetricLockSuccess, lockSucceeded)
}

func (m *mockMetricsCollector) IncSatisfiedThenNot(pattern string, phaseName string) {
	m.incSatisfiedThenNotPattern = append(m.incSatisfiedThenNotPattern, pattern)
	m.incIncSatisfiedThenNotPhase = append(m.incIncSatisfiedThenNotPhase, phaseName)
}

func (m *mockMetricsCollector) TimesEvaluatedCount(ruleID string, count int) {
	m.timesEvaluatedRuleID = append(m.timesEvaluatedRuleID, ruleID)
	m.timesEvaluatedCount = append(m.timesEvaluatedCount, count)
	fmt.Printf("metrics.TimesEvaluatedCount - %s, %d\n", ruleID, count)
}
