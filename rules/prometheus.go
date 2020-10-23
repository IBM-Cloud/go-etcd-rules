package rules

import (
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
	"time"
)

const (
	region  = "region"
	service = "service"
	action  = "action"
	method  = "method"
	prefix  = "prefix"
	success = "success"
)

var (
	operationLabels   = []string{region, service, action, method, prefix, success}
	EtcdOperationKeys = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:      "operation_keys",
		Subsystem: "etcd",
		Namespace: "data",
		Help:      "number of keys received or transmitted in operation",
		Buckets:   []float64{0, 1, 2, 5, 10, 20, 50, 100, 200, 500, 1000, 2000, 5000, 10000, 20000, 50000},
	}, operationLabels)
	EtcdOperationSize = prometheus.NewHistogramVec(prometheus.HistogramOpts{
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
	prometheus.MustRegister(EtcdOperationKeys)
	prometheus.MustRegister(EtcdOperationSize)

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

func observeWatchEvents(p string, events, totalBytes int, mo ...metricOption) {
	labels := map[string]string{
		region:  "",
		service: "",
		method:  "rules-engine-watcher",
		action:  "watch",
		prefix:  p,
		success: "true",
	}
	/*
		note: due to the inability to register a metric with dynamic
		sets of labels, all labels must be present in the metric definition.
		Therefore if a metric option is passed that does not exist in the initialized
		set of labels it will be ignored. Any client wishing to make use of additional
		labels will need to initialize them in the metric and above map first
	*/
	for _, opt := range mo {
		if _, ok := labels[opt.key]; ok {
			labels[opt.key] = opt.value
		}
	}
	EtcdOperationKeys.With(labels).Observe(float64(events))
	EtcdOperationSize.With(labels).Observe(float64(totalBytes))
}
