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

func (tl *mockLock) Unlock() {
	tl.channel <- true
}
