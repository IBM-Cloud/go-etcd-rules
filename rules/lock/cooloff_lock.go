package lock

import (
	"fmt"
	"time"
)

// NewCoolOffLocker creates a simple locker that will prevent a lock from
// being obtained if a previous attempt (successful or not) was made within
// the specified expiration period. It is intended to be used with other lockers
// to prevent excessive locking using more expensive resources. It is theoretically
// possible for two callers to obtain the same lock, if the cooloff period expires
// before the first caller releases the lock. This locker should be used with a nested
// locker to prevent two callers from accessing the same protected resource.
func NewCoolOffLocker(expiration time.Duration) RuleLocker {
	locker := coolOffLocker{
		lock:            make(chan coolOffLockerRequest),
		coolOffDuration: expiration,
	}
	go locker.run()
	return locker
}

type coolOffLocker struct {
	lock            chan coolOffLockerRequest
	coolOffDuration time.Duration
}

func (col coolOffLocker) Lock(key string, options ...Option) (RuleLock, error) {
	req := coolOffLockerRequest{
		key:  key,
		resp: make(chan error),
	}
	col.lock <- req
	err := <-req.resp
	if err != nil {
		return nil, err
	}
	return coolOffLock{}, nil
}

func (col coolOffLocker) run() {
	locks := make(map[string]time.Time)
	for {
		req := <-col.lock

		now := time.Now()
		// Remove any expired keys
		var toDelete []string
		for k, v := range locks {
			if now.After(v) {
				toDelete = append(toDelete, k)
			}
		}
		for _, key := range toDelete {
			delete(locks, key)
		}
		// Create response
		var err error
		if _, ok := locks[req.key]; ok {
			err = fmt.Errorf("cooloff expires in %s", col.coolOffDuration)
		}
		// Failed attempts to get the lock should also update the cooloff
		locks[req.key] = now.Add(col.coolOffDuration)
		req.resp <- err
	}
}

type coolOffLockerRequest struct {
	key  string
	resp chan error
}

type coolOffLock struct {
}

func (coolOffLock) Unlock() error {
	return nil
}
