package concurrency

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.uber.org/zap"

	"github.com/IBM-Cloud/go-etcd-rules/rules/teststore"
)

func Test_SessionManager(t *testing.T) {
	_, client := teststore.InitV3Etcd(t)
	defer client.Close()
	lgr, err := zap.NewDevelopment()
	require.NoError(t, err)
	mgr := newSessionManager(client, 0, lgr)
	var wg sync.WaitGroup
	// Use a lot of goroutines to ensure any concurrency
	// issues are caught by race condition checks.
	for i := 0; i < 1000; i++ {
		// Make a copy for the goroutine
		localI := i
		wg.Add(1)
		go func() {
			session, err := mgr.GetSession(context.Background())
			assert.NoError(t, err)
			if localI%10 == 0 {
				// Disrupt things by closing sessions, forcing
				// the manager to create new ones.
				_ = session.Close()
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func Test_NewSessionManager(t *testing.T) {
	_, client := teststore.InitV3Etcd(t)
	lgr, err := zap.NewDevelopment()
	require.NoError(t, err)
	mgr := NewSessionManager(client, lgr)
	assert.Equal(t, lgr, mgr.logger)
	session, err := mgr.newSession()
	if assert.NoError(t, err) {
		session.Close()
	}
	assert.NoError(t, client.Close())
}
