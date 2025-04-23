package lock

import "errors"

// MockLocker implements the RuleLocker interface.
type MockLocker struct {
	Channel  chan bool
	ErrorMsg *string
}

func (tlkr *MockLocker) Lock(key string, options ...Option) (RuleLock, error) {
	if tlkr.ErrorMsg != nil {
		return nil, errors.New(*tlkr.ErrorMsg)
	}
	tLock := mockLock{
		channel: tlkr.Channel,
	}
	return &tLock, nil
}

type mockLock struct {
	channel chan bool
}

func (tl *mockLock) Unlock(options ...Option) error {
	tl.channel <- true
	return nil
}

// FuncMockLocker instances are driven by functions that are provided.
type FuncMockLocker struct {
	LockF func(string, ...Option) (RuleLock, error)
}

// Lock is a mock implementation of RuleLock.Lock
func (ml FuncMockLocker) Lock(key string, options ...Option) (RuleLock, error) {
	return ml.LockF(key, options...)
}

// FuncMockLock instances are driven by functions that are provided.
type FuncMockLock struct {
	UnlockF func(options ...Option) error
}

// Unlock is a mock implementation of RuleLock.Unlock
func (ml FuncMockLock) Unlock(options ...Option) error {
	return ml.UnlockF(options...)
}
