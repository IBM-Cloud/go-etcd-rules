package rules

import (
	"testing"
	"time"

	"github.com/IBM-Cloud/go-etcd-rules/rules/concurrency"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/clientv3"
)

func TestV3Locker(t *testing.T) {
	cfg, cl := initV3Etcd(t)
	c, err := clientv3.New(cfg)
	require.NoError(t, err)
	session1, err := concurrency.NewSession(cl)
	require.NoError(t, err)
	defer session1.Close()

	lLocker := newLocalLocker()

	rlckr1 := v3Locker{
		lockTimeout: time.Minute,
		getSession:  func() (*concurrency.Session, error) { return session1, nil },
		lLocker:     lLocker,
	}
	defer lLocker.close()
	rlck, err1 := rlckr1.lock("/test")
	assert.NoError(t, err1)
	require.NotNil(t, rlck)

	session2, err := concurrency.NewSession(cl)
	require.NoError(t, err)
	defer session2.Close()

	rlckr2 := v3Locker{
		lockTimeout: time.Minute,
		getSession:  func() (*concurrency.Session, error) { return session2, nil },
		lLocker:     lLocker,
	}

	_, err2 := rlckr2.lockWithTimeout("/test", 10*time.Second)
	assert.Error(t, err2)
	assert.NoError(t, rlck.unlock())

	// Verify that behavior holds across goroutines

	done1 := make(chan bool)
	done2 := make(chan bool)

	go func() {
		session, err := concurrency.NewSession(cl)
		require.NoError(t, err)
		defer session.Close()
		lckr := newV3Locker(c, 5*time.Second, func() (*concurrency.Session, error) { return session, nil })
		lck, lErr := lckr.lock("/test1")
		assert.NoError(t, lErr)
		done1 <- true
		<-done2
		if lck != nil {
			assert.NoError(t, lck.unlock())
		}
	}()
	<-done1
	_, err = rlckr1.lock("/test1")
	assert.Error(t, err)
	done2 <- true
}

func Test_localLocker(t *testing.T) {
}

type mockLocker struct {
	lockF func(string) (ruleLock, error)
}

func (ml mockLocker) lock(key string) (ruleLock, error) {
	return ml.lockF(key)
}

type mockLock struct {
	unlockF func() error
}

func (ml mockLock) unlock() error {
	return ml.unlockF()
}
