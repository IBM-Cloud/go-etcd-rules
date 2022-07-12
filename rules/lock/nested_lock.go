package lock

// NewNestedLocker creates a locker that protects the inner
// locker with an outer locker, so that no unnecessary calls
// are made to the inner locker when attempting to obtain
// an unavailable lock.
func NewNestedLocker(outer, inner RuleLocker) RuleLocker {
	return nestedLocker{
		own:    outer,
		nested: inner,
	}
}

type nestedLocker struct {
	own    RuleLocker
	nested RuleLocker
}

func (nl nestedLocker) Lock(key string, options ...Option) (RuleLock, error) {
	// Try to obtain own lock first, preempting attempts
	// to obtain the nested (more expensive) lock if
	// getting the local lock fails.
	lock, err := nl.own.Lock(key, options...)
	if err != nil {
		return nil, err
	}
	// Try to obtain the nested lock
	nested, err := nl.nested.Lock(key, options...)
	if err != nil {
		// First unlock own lock
		_ = lock.Unlock() // #nosec G104 -- Try to unlock
		return nil, err
	}
	return nestedLock{
		own:    lock,
		nested: nested,
	}, nil
}

type nestedLock struct {
	own    RuleLock
	nested RuleLock
}

func (nl nestedLock) Unlock() error {
	// Always unlock own lock, but after
	// nested lock. This prevents attempting
	// to get a new instance of the nested lock
	// before the own lock is cleared. If the nested
	// lock persists due to an error, it should be
	// cleared with separate logic.

	err := nl.nested.Unlock()
	ownError := nl.own.Unlock()
	// The nested lock is assumed to be more expensive so
	// its error takes precedence.
	if err == nil {
		err = ownError
	}
	return err
}
