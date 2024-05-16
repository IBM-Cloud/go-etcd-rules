package lock

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v3 "go.etcd.io/etcd/client/v3"
	v3c "go.etcd.io/etcd/client/v3/concurrency"
	"golang.org/x/net/context"

	"github.com/IBM-Cloud/go-etcd-rules/rules/teststore"
)

func Test_V3Locker(t *testing.T) {
	cfg, cl := teststore.InitV3Etcd(t)
	c, err := v3.New(cfg)
	require.NoError(t, err)
	newSession := func(_ context.Context) (*v3c.Session, error) {
		return v3c.NewSession(cl, v3c.WithTTL(30))
	}

	for _, useTryLock := range []bool{false, true} {
		var name string
		if useTryLock {
			name = "use_try_lock"
		} else {
			name = "use_lock"
		}
		t.Run(name, func(t *testing.T) {
			rlckr := v3Locker{
				newSession:  newSession,
				lockTimeout: 5,
			}
			rlck, err1 := rlckr.Lock("test")
			assert.NoError(t, err1)
			_, err2 := rlckr.lockWithTimeout("test", 10)
			assert.Error(t, err2)
			assert.NoError(t, rlck.Unlock())

			done1 := make(chan bool)
			done2 := make(chan bool)

			go func() {
				lckr := NewV3Locker(c, 5, useTryLock)
				lck, lErr := lckr.Lock("test1")
				assert.NoError(t, lErr)
				done1 <- true
				<-done2
				if lck != nil {
					assert.NoError(t, lck.Unlock())
				}
			}()
			<-done1
			_, err = rlckr.Lock("test1")
			assert.Error(t, err)
			done2 <- true
		})
	}
}

func Test_V3LockerRegex(t *testing.T) {
	cfg, cl := teststore.InitV3Etcd(t)
	_, err := v3.New(cfg)
	require.NoError(t, err)
	newSession := func(_ context.Context) (*v3c.Session, error) {
		return v3c.NewSession(cl, v3c.WithTTL(30))
	}

	testcases := []struct {
		name    string
		lockKey string
		err     error
	}{
		{
			name:    "bad regex",
			lockKey: "/test?/",
			err:     fmt.Errorf("Path variable contains an invalid character"),
		},
		{
			name:    "good regex",
			lockKey: "/test/",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			rlckr := v3Locker{
				newSession:  newSession,
				lockTimeout: 5,
			}
			_, err := rlckr.Lock(tc.lockKey)
			assert.Equal(t, err, tc.err)
		})
	}
}
