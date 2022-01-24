package lock

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v3 "go.etcd.io/etcd/client/v3"

	"github.com/IBM-Cloud/go-etcd-rules/rules/teststore"
)

func Test_V3Locker(t *testing.T) {
	cfg, cl := teststore.InitV3Etcd(t)
	c, err := v3.New(cfg)
	assert.NoError(t, err)
	rlckr := v3Locker{
		cl:          cl,
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
		lckr := NewV3Locker(c, 5)
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
}
