package rules

import (
	"testing"

	"github.com/IBM-Cloud/go-etcd-rules/rules/teststore"

	"github.com/IBM-Cloud/go-etcd-rules/concurrency"
	"github.com/IBM-Cloud/go-etcd-rules/rules/lock"
	"github.com/stretchr/testify/assert"
)

func Test_Reentrant(t *testing.T) {
	_, cl := teststore.InitV3Etcd(t)
	for _, useTryLock := range []bool{false, true} {
		for _, useShared := range []bool{false, true} {
			var name string
			if useTryLock {
				name = "use_try_lock"
			} else {
				name = "use_lock"
			}
			if useShared {
				name += "_shared"
			}
			t.Run(name, func(t *testing.T) {
				logger := getTestLogger()
				var rlckr lock.RuleLocker
				if useShared {
					sessionManager := concurrency.NewSessionManager(cl, logger)
					rlckr = lock.NewSessionLocker(sessionManager.GetSession, 5, false, useTryLock)
				} else {
					baseEtcdLocker := lock.NewV3Locker(cl, 5, useTryLock)
					baseMapLocker := lock.NewMapLocker()
					rlckr = lock.NewNestedLocker(baseMapLocker, baseEtcdLocker)
				}
				rlck, err1 := rlckr.Lock("test")
				assert.NoError(t, err1)
				_, err2 := rlckr.Lock("test")
				assert.Error(t, err2)
				assert.NoError(t, rlck.Unlock())
			})
		}
	}
}
