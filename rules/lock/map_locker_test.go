package lock

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_mapLocker_toggle(t *testing.T) {
	testCases := []struct {
		name string

		setup func(ml *mapLocker)

		key  string
		lock bool

		ok bool
	}{
		{
			name: "get_available",
			key:  "/foo",
			setup: func(ml *mapLocker) {
				ml.toggle("/bar", true)
			},
			lock: true,
			ok:   true,
		},
		{
			name: "get_unavailable",
			key:  "/foo",
			setup: func(ml *mapLocker) {
				ml.toggle("/foo", true)
			},
			lock: true,
			ok:   false,
		},
		{
			name: "release_existing",
			key:  "/foo",
			setup: func(ml *mapLocker) {
				ml.toggle("/foo", true)
			},
			lock: false,
			ok:   true,
		},
		{
			name: "release_nonexistent",
			key:  "/foo",
			lock: false,
			ok:   true,
		},
		{
			name: "get_from_closed",
			key:  "/foo",
			setup: func(ml *mapLocker) {
				ml.close()
			},
			lock: true,
			ok:   false,
		},
		{
			name: "release_from_closed",
			key:  "/foo",
			setup: func(ml *mapLocker) {
				ml.close()
			},
			lock: false,
			ok:   false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ml := newMapLocker()
			defer ml.close()

			if tc.setup != nil {
				tc.setup(&ml)
			}

			assert.Equal(t, tc.ok, ml.toggle(tc.key, tc.lock))

		})
	}
}

func Test_toggleLockAdapter(t *testing.T) {
	const (
		testKey = "/foo"
	)
	errLocked := errors.New("locked")
	testCases := []struct {
		name string

		lock     bool
		toggleOk bool

		err error
	}{
		{
			name:     "success",
			toggleOk: true,
		},
		{
			name:     "failure",
			toggleOk: false,
			err:      errLocked,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			expectedLock := true
			var err error
			tla := toggleLockerAdapter{
				toggle: func(key string, lock bool) bool {
					assert.Equal(t, expectedLock, lock)
					assert.Equal(t, testKey, key)
					return tc.toggleOk
				},
				errLocked: errLocked,
			}
			var _ RuleLocker = tla
			lock, err := tla.Lock(testKey)
			if tc.err != nil {
				assert.EqualError(t, err, tc.err.Error())
				return
			}
			assert.NoError(t, err)
			expectedLock = false
			_ = assert.NotNil(t, lock) && assert.NoError(t, lock.Unlock())
		})
	}
}
