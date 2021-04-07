package rules

import (
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
	"time"
)

var (
	rulesEngineLockCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:      "lock_count",
		Subsystem: "etcd",
		Namespace: "rules",
		Help:      "etcd rules engine lock count",
	}, []string{"method", "pattern", "success"})
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
)

func init() {
	prometheus.MustRegister(rulesEngineLockCount)
	prometheus.MustRegister(rulesEngineSatisfiedThenNot)
	prometheus.MustRegister(rulesEngineEvaluations)
	prometheus.MustRegister(rulesEngineWorkerQueueWait)
	prometheus.MustRegister(rulesEngineWorkBufferWaitTime)
}

func incLockMetric(methodName string, pattern string, lockSucceeded bool) {
	rulesEngineLockCount.WithLabelValues(methodName, pattern, strconv.FormatBool(lockSucceeded)).Inc()
}

func incSatisfiedThenNot(methodName string, pattern string, phaseName string) {
	rulesEngineSatisfiedThenNot.WithLabelValues(methodName, pattern, phaseName).Inc()
}

func timesEvaluated(methodName string, ruleID string, count int) {
	rulesEngineEvaluations.WithLabelValues(methodName, ruleID).Set(float64(count))
}

func workerQueueWaitTime(methodName string, startTime time.Time) {
	rulesEngineWorkerQueueWait.WithLabelValues(methodName).Observe(float64(time.Since(startTime).Nanoseconds() / 1e6))
}

func workBufferWaitTime(methodName, pattern string, startTime time.Time) {
	rulesEngineWorkBufferWaitTime.WithLabelValues(methodName, pattern).Observe(float64(time.Since(startTime).Nanoseconds() / 1e6))
}
