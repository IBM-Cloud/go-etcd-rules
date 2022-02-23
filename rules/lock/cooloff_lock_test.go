package lock

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_coolOffLocker(t *testing.T) {
	const (
		timeout = time.Millisecond * 100
		key1    = "key1"
		key2    = "key2"
	)
	// Test cases will try to obtain two locks.
	// The duration between requests is controlled
	// via the "delay" field and whether or not the
	// second attempt uses the same key is controlled
	// via the "keyDifferent" field.
	testCases := []struct {
		name string

		delay        time.Duration
		keyDifferent bool

		err bool
	}{
		{
			name:         "ok_same_key_enough_delay",
			delay:        timeout * 2,
			keyDifferent: false,
		},
		{
			name:         "ok_different_key_no_delay",
			delay:        0,
			keyDifferent: true,
		},
		{
			name:         "fail_same_key_insufficient_delay",
			delay:        0,
			keyDifferent: false,
			err:          true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			locker := NewCoolOffLocker(timeout)
			// Obtain the first lock
			lock1, err := locker.Lock(key1)
			// This should always be successful
			require.NoError(t, err)
			require.NotNil(t, lock1)
			// Make sure that the lock works
			defer require.NotPanics(t, func() { lock1.Unlock() })

			// Wait some period of time
			time.Sleep(tc.delay)

			var secondLockKey string
			if tc.keyDifferent {
				secondLockKey = key2
			} else {
				secondLockKey = key1
			}
			lock2, err := locker.Lock(secondLockKey)

			if tc.err {
				if assert.Error(t, err) {
					// Verify that the error message uses the correct format.
					var (
						timeoutString string
						valueCount    int
					)
					valueCount, err = fmt.Fscanf(strings.NewReader(err.Error()), coolOffErrFormat, &timeoutString)
					assert.NoError(t, err)
					assert.Equal(t, 1, valueCount)
					assert.NotEmpty(t, timeoutString)
				}
				assert.Nil(t, lock2)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, lock2)
			// Make sure the second lock works
			require.NotPanics(t, func() { lock2.Unlock() })
		})
	}

}
