package lock

import (
	"errors"
	"sync"
)

func NewMapLocker() RuleLocker {
	ml := newMapLocker()
	// Using the adapter to reduce the number of critical sections down to
	// 1, lessening the chances of concurrency issues being introduced.
	return toggleLockerAdapter{
		toggle:    ml.toggle,
		errLocked: ErrLockedLocally,
	}
}

type mapLocker struct {
	mutex *sync.Mutex
	m     map[string]bool
}

func newMapLocker() mapLocker {
	return mapLocker{
		m:     make(map[string]bool),
		mutex: &sync.Mutex{},
	}
}

func (ml mapLocker) toggle(key string, lock bool) bool {
	ml.mutex.Lock()
	defer ml.mutex.Unlock()
	// 4 possibilities:
	// 1. key is locked and lock is true: return false
	// 2. key is locked and lock is false: unlock key and return true
	if ml.m[key] {
		if !lock {
			delete(ml.m, key)
		}
		return !lock
	}
	// 3. key is unlocked and lock is true: lock key and return true
	// 4. key is unlocked and lock is false: return true
	if lock {
		ml.m[key] = true
	}
	return true
}

// ErrLockedLocally indicates that a local goroutine holds the lock
// and no attempt will be made to obtain the lock via etcd.
var ErrLockedLocally = errors.New("locked locally")

type toggleLockerAdapter struct {
	toggle    func(key string, lock bool) bool
	errLocked error
}

func (tla toggleLockerAdapter) Lock(key string, options ...Option) (RuleLock, error) {
	ok := tla.toggle(key, true)
	if !ok {
		return nil, tla.errLocked
	}
	return toggleLock{
		toggle: tla.toggle,
		key:    key,
	}, nil
}

type toggleLock struct {
	toggle func(key string, lock bool) bool
	key    string
}

func (tl toggleLock) Unlock(_ ...Option) error {
	_ = tl.toggle(tl.key, false)
	return nil
}
