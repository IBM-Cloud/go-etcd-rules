package rules

import (
	"testing"

	"github.com/IBM-Cloud/go-etcd-rules/rules/concurrency"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/clientv3"
)

func TestV3Locker(t *testing.T) {
	cfg, cl := initV3Etcd(t)
	c, err := clientv3.New(cfg)
	require.NoError(t, err)
	session, err := concurrency.NewSession(cl)
	require.NoError(t, err)
	defer session.Close()

	rlckr := v3Locker{
		cl:          cl,
		lockTimeout: 5,
		getSession:  func() (*concurrency.Session, error) { return session, nil },
	}
	rlck, err1 := rlckr.lock("test")
	assert.NoError(t, err1)
	_, err2 := rlckr.lockWithTimeout("test", 10)
	assert.Error(t, err2)
	rlck.unlock()

	done1 := make(chan bool)
	done2 := make(chan bool)

	go func() {
		session, err := concurrency.NewSession(cl)
		require.NoError(t, err)
		defer session.Close()
		lckr := newV3Locker(c, 5, func() (*concurrency.Session, error) { return session, nil })
		lck, lErr := lckr.lock("test1")
		assert.NoError(t, lErr)
		done1 <- true
		<-done2
		if lck != nil {
			lck.unlock()
		}
	}()
	<-done1
	_, err = rlckr.lock("test1")
	assert.Error(t, err)
	done2 <- true
}
