package lock

import "github.com/IBM-Cloud/go-etcd-rules/metrics"

// WithMetrics decorates a locker with metrics.
func WithMetrics(ruleLocker RuleLocker, name string) RuleLocker {
	return withMetrics(ruleLocker, name, metrics.IncLockMetric)
}
func withMetrics(ruleLocker RuleLocker, name string,
	observeLock func(locker string, methodName string, pattern string, lockSucceeded bool)) RuleLocker {
	return metricLocker{
		RuleLocker:  ruleLocker,
		lockerName:  name,
		observeLock: observeLock,
	}
}

type metricLocker struct {
	RuleLocker
	lockerName  string
	observeLock func(locker string, methodName string, pattern string, lockSucceeded bool)
}

func (ml metricLocker) Lock(key string, options ...Option) (RuleLock, error) {
	opts := buildOptions(options...)
	lock, err := ml.RuleLocker.Lock(key, options...)
	ml.observeLock(ml.lockerName, opts.method, opts.pattern, err == nil)
	return lock, err
}
