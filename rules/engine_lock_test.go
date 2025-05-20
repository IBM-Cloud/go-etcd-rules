package rules

import (
	"testing"

	"github.com/IBM-Cloud/go-etcd-rules/rules/teststore"

	"github.com/IBM-Cloud/go-etcd-rules/concurrency"
	"github.com/IBM-Cloud/go-etcd-rules/rules/lock"
	"github.com/stretchr/testify/assert"
)

// Note: This test mimics the actual paths used for locking in production code.
//
//	Shared sessions allows for the lock to be reentrant, while not-shared
//	prevents reentrant behavior.
func Test_ReentrantLocking(t *testing.T) {
	_, cl := teststore.InitV3Etcd(t)
	for _, useTryLock := range []bool{false, true} {
		for _, useShared := range []bool{false, true} {
			for _, closeSession := range []bool{false, true} {
				if !useShared && closeSession {
					continue
				}
				var name string
				if useTryLock {
					name = "use_try_lock"
				} else {
					name = "use_lock"
				}
				if useShared {
					name += "_shared"
				}
				if closeSession {
					name += "_closed"
				}
				t.Run(name, func(t *testing.T) {
					logger := getTestLogger()
					var rlckr lock.RuleLocker
					if useShared {
						sessionManager := concurrency.NewSessionManager(cl, logger)
						rlckr = lock.NewSessionLocker(sessionManager.GetSession, 5, closeSession, useTryLock)
					} else {
						baseEtcdLocker := lock.NewV3Locker(cl, 5, useTryLock)
						baseMapLocker := lock.NewMapLocker()
						rlckr = lock.NewNestedLocker(baseMapLocker, baseEtcdLocker)
					}
					rlck, err1 := rlckr.Lock("test")
					assert.NoError(t, err1)
					duplck, err2 := rlckr.Lock("test")
					if useShared {
						assert.NoError(t, err2)
						assert.NoError(t, duplck.Unlock())
					} else {
						assert.Error(t, err2)
					}
					if closeSession {
						assert.NoError(t, rlck.Unlock())
					} else {
						assert.NoError(t, rlck.Unlock())
					}
				})
			}
		}
	}
}
