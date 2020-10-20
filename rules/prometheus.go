package rules

import (
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
	"time"
)

var (
	labels          = []string{"region", "service", "action", "method", "prefix"}
	operationLabels = append(labels, "success")

	etcdOperationKeys = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:      "operation_keys",
		Subsystem: "etcd",
		Namespace: "data",
		Help:      "number of keys received or transmitted in operation",
		Buckets:   []float64{0, 1, 2, 5, 10, 20, 50, 100, 200, 500, 1000, 2000, 5000, 10000, 20000, 50000},
	}, operationLabels)
	etcdOperationSize = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:      "operation_bytes",
		Subsystem: "etcd",
		Namespace: "data",
		Help:      "size of keys received or transmitted in operation (in B)",
		Buckets:   []float64{0, 1, 2, 5, 10, 20, 50, 100, 200, 500, 1000, 2000, 5000, 10000, 20000, 50000, 100000, 1000000, 10000000},
	}, operationLabels)

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
)

func init() {
	prometheus.MustRegister(etcdOperationKeys)
	prometheus.MustRegister(etcdOperationSize)

	prometheus.MustRegister(rulesEngineLockCount)
	prometheus.MustRegister(rulesEngineSatisfiedThenNot)
	prometheus.MustRegister(rulesEngineEvaluations)
	prometheus.MustRegister(rulesEngineWorkerQueueWait)
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

func observeWatchEvents(prefix string, events, totalBytes int, mo ...metricOption) {
	labels := map[string]string{
		"region":  "",
		"service": "",
		"method":  "rules-engine-watcher",
		"action":  "watch",
		"prefix":  prefix,
		"success": "true",
	}
	for _, opt := range mo {
		if _, ok := labels[opt.key]; ok {
			labels[opt.key] = opt.value
		}
	}
	etcdOperationKeys.With(labels).Observe(float64(events))
	etcdOperationSize.With(labels).Observe(float64(totalBytes))
}
