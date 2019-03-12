package rules

import (
	"testing"

	"github.com/coreos/etcd/clientv3"
	"github.com/stretchr/testify/assert"
)

func TestV3Locker(t *testing.T) {
	cfg, cl := initV3Etcd()
	c, err := clientv3.New(cfg)
	assert.NoError(t, err)
	newV3Locker(c)
	rlckr := v3Locker{
		cl:      cl,
		metrics: newMetricsCollector(),
	}
	rlck, err1 := rlckr.lock("test", 10)
	assert.NoError(t, err1)
	_, err2 := rlckr.lockWithTimeout("test", 10, 1)
	assert.Error(t, err2)
	rlck.unlock()

	done1 := make(chan bool)
	done2 := make(chan bool)

	go func() {
		lckr := newV3Locker(c)
		lck, lErr := lckr.lock("test1", 10)
		assert.NoError(t, lErr)
		done1 <- true
		<-done2
		if lck != nil {
			lck.unlock()
		}
	}()
	<-done1
	_, err = rlckr.lock("test1", 1)
	assert.Error(t, err)
	done2 <- true
}
