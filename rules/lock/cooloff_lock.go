package lock

import (
	"fmt"
	"sync"
	"time"
)

const (
	coolOffErrFormat = "cooloff expires in %s"
)

// NewCoolOffLocker creates a simple locker that will prevent a lock from
// being obtained if a previous attempt (successful or not) was made within
// the specified expiration period. It is intended to be used with other lockers
// to prevent excessive locking using more expensive resources (e.g. etcd). It is
// theoretically possible for two callers to obtain the same lock, if the cooloff
// period expires before the first caller releases the lock; therefore this locker
// needs to be used with a nested locker to prevent two callers from accessing the
// same protected resource.
func NewCoolOffLocker(expiration time.Duration) RuleLocker {
	locker := coolOffLocker{
		coolOffDuration: expiration,
		locks:           make(map[string]time.Time),
		mutex:           &sync.Mutex{},
	}
	return locker
}

type coolOffLocker struct {
	locks           map[string]time.Time
	mutex           *sync.Mutex
	coolOffDuration time.Duration
}

func (col coolOffLocker) Lock(key string, options ...Option) (RuleLock, error) {
	col.mutex.Lock()
	defer col.mutex.Unlock()
	now := time.Now()
	// Remove any expired keys
	var toDelete []string
	for k, v := range col.locks {
		if now.After(v) {
			toDelete = append(toDelete, k)
		}
	}
	for _, key := range toDelete {
		delete(col.locks, key)
	}
	var err error
	if _, ok := col.locks[key]; ok {
		err = fmt.Errorf(coolOffErrFormat, col.coolOffDuration)
	}
	// Failed attempts to get the lock should also update the cooloff,
	// so always add the key regardless of success or failure.
	col.locks[key] = now.Add(col.coolOffDuration)

	if err != nil {
		return nil, err
	}
	return coolOffLock{}, nil
}

type coolOffLock struct {
}

func (coolOffLock) Unlock(_ ...Option) error {
	return nil
}
