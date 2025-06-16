package lock

import (
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
			rlck, err := rlckr.Lock("test")
			assert.NoError(t, err, err)
			_, err = rlckr.lockWithTimeout("test", 10)
			assert.Error(t, err, err)
			err = rlck.Unlock()
			assert.NoError(t, err, err)

			done1 := make(chan bool)
			done2 := make(chan bool)
			go func() {
				lckr := NewV3Locker(c, 5, useTryLock)
				lck, err := lckr.Lock("test1")
				assert.NoError(t, err, err)
				done1 <- true
				<-done2
				if lck != nil {
					err = lck.Unlock()
					assert.NoError(t, err, err)
				}
			}()
			<-done1
			_, err = rlckr.Lock("test1")
			assert.Error(t, err, err)
			done2 <- true
		})
	}
}
