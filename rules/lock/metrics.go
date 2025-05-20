package lock

import "github.com/IBM-Cloud/go-etcd-rules/metrics"

// WithMetrics decorates a locker with metrics.
func WithMetrics(ruleLocker RuleLocker, name string) RuleLocker {
	return withMetrics(ruleLocker, name, metrics.IncLockMetric, metrics.IncUnlockErrorMetric)
}
func withMetrics(ruleLocker RuleLocker, name string,
	observeLock func(locker string, methodName string, pattern string, lockSucceeded bool),
	observeUnlockError func(locker string, methodName string, pattern string)) RuleLocker {
	return metricLocker{
		RuleLocker:         ruleLocker,
		lockerName:         name,
		observeLock:        observeLock,
		observeUnlockError: observeUnlockError,
	}
}

type metricLocker struct {
	RuleLocker
	RuleLock
	lockerName         string
	observeLock        func(locker string, methodName string, pattern string, lockSucceeded bool)
	observeUnlockError func(locker string, methodName string, pattern string)
}

func (ml metricLocker) Lock(key string, options ...Option) (RuleLock, error) {
	opts := buildOptions(options...)
	var err error
	ml.RuleLock, err = ml.RuleLocker.Lock(key, options...)
	ml.observeLock(ml.lockerName, opts.method, opts.pattern, err == nil)
	return ml.RuleLock, err
}

func (ml metricLocker) Unlock(options ...Option) error {
	err := ml.RuleLock.Unlock(options...)
	if err != nil {
		opts := buildOptions(options...)
		ml.observeUnlockError(ml.lockerName, opts.method, opts.pattern)
	}
	return err
}
