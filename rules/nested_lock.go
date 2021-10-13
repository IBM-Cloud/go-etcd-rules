package rules

type nestedLocker struct {
	own    ruleLocker
	nested ruleLocker
}

func (nl nestedLocker) lock(key string) (ruleLock, error) {
	// Try to obtain own lock first, preempting attempts
	// to obtain the nested (more expensive) lock if
	// getting the local lock fails.
	lock, err := nl.own.lock(key)
	if err != nil {
		return nil, err
	}
	// Try to obtain the nested lock
	nested, err := nl.nested.lock(key)
	if err != nil {
		// First unlock own lock
		_ = lock.unlock()
		return nil, err
	}
	return nestedLock{
		own:    lock,
		nested: nested,
	}, nil
}

type nestedLock struct {
	own    ruleLock
	nested ruleLock
}

func (nl nestedLock) unlock() error {
	// Always unlock own lock, but after
	// nested lock. This prevents attempting
	// to get a new instance of the nested lock
	// before the own lock is cleared. If the nested
	// lock persists due to an error, it should be
	// cleared with separate logic.

	err := nl.nested.unlock()
	ownError := nl.own.unlock()
	// The nested lock is assumed to be more expensive so
	// its error takes precedence.
	if err == nil {
		err = ownError
	}
	return err
}
