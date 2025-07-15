package lock

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_metricLocker_Lock(t *testing.T) {
	const (
		testLockerName = "my_locker"
		testMethodName = "my_method"
		testPattern    = "my_pattern"
		testKey        = "my_key"
	)
	errUnlock := errors.New("unlock")
	errLock := errors.New("lock")
	mockLock := FuncMockLock{
		UnlockF: func(_ ...Option) error {
			return errUnlock
		},
	}
	testCases := []struct {
		name string

		options []Option

		method    string
		pattern   string
		succeeded bool
		err       error
	}{
		{
			name: "success_with_both_options",
			options: []Option{
				PatternForLock(testPattern),
				MethodForLock(testMethodName),
			},
			pattern:   testPattern,
			method:    testMethodName,
			succeeded: true,
		},
		{
			name:      "success_with_neither_option",
			pattern:   unknown,
			method:    unknown,
			succeeded: true,
		},
		{
			name: "failure_with_pattern",
			options: []Option{
				PatternForLock(testPattern),
			},
			pattern:   testPattern,
			method:    unknown,
			succeeded: false,
			err:       errLock,
		},
		{
			name: "failure_with_method",
			options: []Option{
				MethodForLock(testMethodName),
			},
			pattern:   unknown,
			method:    testMethodName,
			succeeded: false,
			err:       errLock,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			nested := FuncMockLocker{
				LockF: func(key string, options ...Option) (RuleLock, error) {
					assert.Equal(t, testKey, key)
					return mockLock, tc.err
				},
			}
			observeLock := func(locker string, methodName string, pattern string, attempt uint, lockSucceeded bool) {
				assert.Equal(t, tc.pattern, pattern)
				assert.Equal(t, tc.method, methodName)
				assert.Equal(t, tc.succeeded, lockSucceeded)
			}
			observeUnlock := func(locker string, methodName string, pattern string) {
				assert.Equal(t, tc.pattern, pattern)
				assert.Equal(t, tc.method, methodName)
			}
			ml := withMetrics(nested, testLockerName,
				observeLock,
				observeUnlock,
			)
			lock, err := ml.Lock(testKey, tc.options...)
			if tc.err != nil {
				assert.EqualError(t, err, tc.err.Error())
				return
			}
			assert.NoError(t, err)
			// Do a sanity check on the lock, which is hardcoded to return a particular error
			// when attempting to unlock it.
			_ = assert.NotNil(t, lock) && assert.Equal(t, errUnlock, lock.Unlock())
		})
	}
}
