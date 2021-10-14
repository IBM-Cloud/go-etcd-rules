package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	rulesEngineLockCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:      "lock_count",
		Subsystem: "etcd",
		Namespace: "rules",
		Help:      "etcd rules engine lock count",
	}, []string{"locker", "method", "pattern", "success"})
	rulesEngineSatisfiedThenNot = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:      "rule_satisfied_then_not",
		Subsystem: "etcd",
		Namespace: "rules",
		Help:      "etcd rules engine rule satisfied then not",
	}, []string{"method", "pattern", "phase"})
	rulesEngineEvaluations = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:      "evaluations",
		Subsystem: "etcd",
		Namespace: "rules",
		Help:      "etcd rules engine rule evaluations",
	}, []string{"method", "rule"})
	rulesEngineWorkerQueueWait = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:      "worker_queue_wait_ms",
		Subsystem: "etcd",
		Namespace: "rules",
		Help:      "etcd rules engine worker queue wait time in ms",
		Buckets:   []float64{1, 2, 5, 10, 20, 50, 100, 200, 500, 1000, 2000, 5000, 10000},
	}, []string{"method"})
	rulesEngineWorkBufferWaitTime = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:      "work_buffer_wait_ms",
		Subsystem: "etcd",
		Namespace: "rules",
		Help:      "etcd rules engine work buffer wait time in ms",
		Buckets:   []float64{1, 2, 5, 10, 20, 50, 100, 200, 500, 1000, 2000, 5000, 10000, 30000, 60000, 300000, 600000},
	}, []string{"method", "pattern"})
	rulesEngineCallbackWaitTime = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:      "callback_wait_ms",
		Subsystem: "etcd",
		Namespace: "rules",
		Help:      "etcd rules engine callback wait time in ms",
		Buckets:   []float64{1, 2, 5, 10, 20, 50, 100, 200, 500, 1000, 2000, 5000, 10000, 30000, 60000, 300000, 600000},
	}, []string{"pattern"})
	rulesEngineKeyProcessBufferCap = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:      "key_process_buffer_cap",
		Subsystem: "etcd",
		Namespace: "rules",
		Help:      "current capacity of the key processing buffer",
	})
	rulesEngineWatcherErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:      "watcher_errors",
		Subsystem: "etcd",
		Namespace: "rules",
		Help:      "etcd rules engine watcher errors",
	}, []string{"error", "prefix"})
)

func init() {
	prometheus.MustRegister(rulesEngineLockCount)
	prometheus.MustRegister(rulesEngineSatisfiedThenNot)
	prometheus.MustRegister(rulesEngineEvaluations)
	prometheus.MustRegister(rulesEngineWorkerQueueWait)
	prometheus.MustRegister(rulesEngineWorkBufferWaitTime)
	prometheus.MustRegister(rulesEngineCallbackWaitTime)
	prometheus.MustRegister(rulesEngineKeyProcessBufferCap)
	prometheus.MustRegister(rulesEngineWatcherErrors)
}

// IncLockMetric increments the lock count.
func IncLockMetric(locker string, methodName string, pattern string, lockSucceeded bool) {
	rulesEngineLockCount.WithLabelValues(locker, methodName, pattern, strconv.FormatBool(lockSucceeded)).Inc()
}

// IncSatisfiedThenNot increments the count of a rule having initially been
// satisfied and then not satisfied, either after the initial evaluation
// or after the lock was obtained.
func IncSatisfiedThenNot(methodName string, pattern string, phaseName string) {
	rulesEngineSatisfiedThenNot.WithLabelValues(methodName, pattern, phaseName).Inc()
}

// TimesEvaluated sets the number of times a rule has been evaluated.
func TimesEvaluated(methodName string, ruleID string, count int) {
	rulesEngineEvaluations.WithLabelValues(methodName, ruleID).Set(float64(count))
}

// WorkerQueueWaitTime tracks the amount of time a work item has been sitting in
// a worker queue.
func WorkerQueueWaitTime(methodName string, startTime time.Time) {
	rulesEngineWorkerQueueWait.WithLabelValues(methodName).Observe(float64(time.Since(startTime).Nanoseconds() / 1e6))
}

// WorkBufferWaitTime tracks the amount of time a work item was in the work buffer.
func WorkBufferWaitTime(methodName, pattern string, startTime time.Time) {
	rulesEngineWorkBufferWaitTime.WithLabelValues(methodName, pattern).Observe(float64(time.Since(startTime).Nanoseconds() / 1e6))
}

// CallbackWaitTime tracks how much time elapsed between when the rule was evaluated and the callback called.
func CallbackWaitTime(pattern string, startTime time.Time) {
	rulesEngineCallbackWaitTime.WithLabelValues(pattern).Observe(float64(time.Since(startTime).Nanoseconds() / 1e6))
}

// KeyProcessBufferCap tracks the capacity of the key processor buffer.
func KeyProcessBufferCap(count int) {
	rulesEngineKeyProcessBufferCap.Set(float64(count))
}

// IncWatcherErrMetric increments the watcher error count.
func IncWatcherErrMetric(err, prefix string) {
	rulesEngineWatcherErrors.WithLabelValues(err, prefix).Inc()
}
