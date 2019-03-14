package rules

import (
	"time"
)

// a mock metrics collector used in unit tests
type MockMetricsCollector struct {
	// store what the IncLockMetric function was called with
	incLockMetricPattern     []string
	incLockMetricLockSuccess []bool
	incLockMetricMethod      []string
	// store what the IncSatisfiedThenNot function was called with
	incSatisfiedThenNotPattern   []string
	incIncSatisfiedThenNotPhase  []string
	incIncSatisfiedThenNotMethod []string
	// store what the TimesEvaluatedCount function was called with
	timesEvaluatedRuleID []string
	timesEvaluatedCount  []int
	timesEvaluatedMethod []string
	// store what the WorkerQueueWaitTime was called with
	workerQueueWaitTime       []time.Time
	workerQueueWaitTimeMethod []string
}

func newMockMetricsCollector() MockMetricsCollector {
	return MockMetricsCollector{}
}

func (m *MockMetricsCollector) IncLockMetric(methodName string, pattern string, lockSucceeded bool) {
	m.incLockMetricMethod = append(m.incLockMetricMethod, methodName)
	m.incLockMetricPattern = append(m.incLockMetricPattern, pattern)
	m.incLockMetricLockSuccess = append(m.incLockMetricLockSuccess, lockSucceeded)
}

func (m *MockMetricsCollector) IncSatisfiedThenNot(methodName string, pattern string, phaseName string) {
	m.incIncSatisfiedThenNotMethod = append(m.incIncSatisfiedThenNotMethod, methodName)
	m.incSatisfiedThenNotPattern = append(m.incSatisfiedThenNotPattern, pattern)
	m.incIncSatisfiedThenNotPhase = append(m.incIncSatisfiedThenNotPhase, phaseName)
}

func (m *MockMetricsCollector) TimesEvaluatedCount(methodName string, ruleID string, count int) {
	m.timesEvaluatedMethod = append(m.timesEvaluatedMethod, methodName)
	m.timesEvaluatedRuleID = append(m.timesEvaluatedRuleID, ruleID)
	m.timesEvaluatedCount = append(m.timesEvaluatedCount, count)
}

func (m *MockMetricsCollector) WorkerQueueWaitTime(methodName string, startTime time.Time) {
	m.workerQueueWaitTime = append(m.workerQueueWaitTime, startTime)
	m.workerQueueWaitTimeMethod = append(m.workerQueueWaitTimeMethod, methodName)
}
