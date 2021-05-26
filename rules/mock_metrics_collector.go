package rules

import (
	"go.uber.org/zap"
	"time"
)

// MockMetricsCollector a mock metrics collector used in unit tests
type MockMetricsCollector struct {
	logger *zap.Logger
	// store what the IncLockMetric function was called with
	IncLockMetricPattern     []string
	IncLockMetricLockSuccess []bool
	IncLockMetricMethod      []string
	// store what the IncSatisfiedThenNot function was called with
	IncSatisfiedThenNotPattern   []string
	IncIncSatisfiedThenNotPhase  []string
	IncIncSatisfiedThenNotMethod []string
	// store what the TimesEvaluated function was called with
	TimesEvaluatedRuleID []string
	TimesEvaluatedCount  []int
	TimesEvaluatedMethod []string
	// store what the WorkerQueueWaitTime was called with
	WorkerQueueWaitTimeTimes  []time.Time
	WorkerQueueWaitTimeMethod []string
	// store what the ObserveWatchEvents was called with
	ObserveWatchEventsPrefixes   []string
	ObserveWatchEventsEvents     []int
	ObserveWatchEventsTotalBytes []int
}

func NewMockMetricsCollector() MockMetricsCollector {
	return MockMetricsCollector{}
}

func (m *MockMetricsCollector) SetLogger(lgr *zap.Logger) {
	m.logger = lgr
}

func (m *MockMetricsCollector) IncLockMetric(methodName string, pattern string, lockSucceeded bool) {
	if m.logger != nil {
		m.logger.Info("metrics.IncLockMetric", zap.String("methodName", methodName), zap.String("pattern", pattern))
	}
	m.IncLockMetricMethod = append(m.IncLockMetricMethod, methodName)
	m.IncLockMetricPattern = append(m.IncLockMetricPattern, pattern)
	m.IncLockMetricLockSuccess = append(m.IncLockMetricLockSuccess, lockSucceeded)
}

func (m *MockMetricsCollector) IncSatisfiedThenNot(methodName string, pattern string, phaseName string) {
	if m.logger != nil {
		m.logger.Info("metrics.IncSatisfiedThenNot", zap.String("methodName", methodName),
			zap.String("pattern", pattern), zap.String("phaseName", phaseName))
	}
	m.IncIncSatisfiedThenNotMethod = append(m.IncIncSatisfiedThenNotMethod, methodName)
	m.IncSatisfiedThenNotPattern = append(m.IncSatisfiedThenNotPattern, pattern)
	m.IncIncSatisfiedThenNotPhase = append(m.IncIncSatisfiedThenNotPhase, phaseName)
}

func (m *MockMetricsCollector) TimesEvaluated(methodName string, ruleID string, count int) {
	if m.logger != nil {
		m.logger.Info("metrics.TimesEvaluated", zap.String("methodName", methodName), zap.Int("count", count))
	}
	m.TimesEvaluatedMethod = append(m.TimesEvaluatedMethod, methodName)
	m.TimesEvaluatedRuleID = append(m.TimesEvaluatedRuleID, ruleID)
	m.TimesEvaluatedCount = append(m.TimesEvaluatedCount, count)
}

func (m *MockMetricsCollector) WorkerQueueWaitTime(methodName string, startTime time.Time) {
	if m.logger != nil {
		m.logger.Info("metrics.WorkerQueueWaitTime", zap.String("methodName", methodName), zap.Time("startTime", startTime))
	}
	m.WorkerQueueWaitTimeTimes = append(m.WorkerQueueWaitTimeTimes, startTime)
	m.WorkerQueueWaitTimeMethod = append(m.WorkerQueueWaitTimeMethod, methodName)
}

func (m *MockMetricsCollector) ObserveWatchEvents(prefix string, events, totalBytes int) {
	if m.logger != nil {
		m.logger.Info("metrics.ObserveWatchEvents", zap.String("prefix", prefix), zap.Int("events", events), zap.Int("totalBytes", totalBytes))
	}
	m.ObserveWatchEventsPrefixes = append(m.ObserveWatchEventsPrefixes, prefix)
	m.ObserveWatchEventsEvents = append(m.ObserveWatchEventsEvents, events)
	m.ObserveWatchEventsTotalBytes = append(m.ObserveWatchEventsTotalBytes, totalBytes)

}
