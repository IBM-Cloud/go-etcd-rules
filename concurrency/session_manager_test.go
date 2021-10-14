package concurrency

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/clientv3"
	"go.uber.org/zap"

	"github.com/IBM-Cloud/go-etcd-rules/rules/teststore"
)

func Test_SessionManager(t *testing.T) {
	_, client := teststore.InitV3Etcd(t)
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

func Test_SessionManager_Close(t *testing.T) {
	lgr, err := zap.NewDevelopment()
	require.NoError(t, err)
	_, goodClient := teststore.InitV3Etcd(t)
	badClient, _ := clientv3.New(clientv3.Config{
		Endpoints: []string{"http://127.0.0.1:2377"},
	})
	testCases := []struct {
		name string

		client     *clientv3.Client
		newSession func() (*Session, error)
	}{
		{
			name:   "ok",
			client: goodClient,
			newSession: func() (*Session, error) {
				return NewSession(goodClient)
			},
		},
		{
			name:   "bad",
			client: badClient,
			newSession: func() (*Session, error) {
				return nil, errors.New("bad")
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mgr := &SessionManager{
				logger:     lgr,
				retryDelay: time.Millisecond,
				get:        make(chan sessionManagerGetRequest),
				close:      make(chan struct{}),
				newSession: tc.newSession,
			}
			go mgr.run()
			var wg sync.WaitGroup
			// Use a lot of goroutines to ensure any concurrency
			// issues are caught by race condition checks.
			for i := 0; i < 1000; i++ {
				// Make a copy for the goroutine
				localI := i
				wg.Add(1)
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), time.Second)
					defer cancel()
					session, _ := mgr.GetSession(ctx)
					if localI%10 == 0 {
						// Disrupt things by closing sessions, forcing
						// the manager to create new ones.
						if session != nil {
							_ = session.Close()
						}
					}
					if localI%25 == 0 {
						mgr.Close()
					}
					wg.Done()
				}()
			}
			wg.Wait()
		})
	}
}

func Test_NewSessionManager(t *testing.T) {
	_, client := teststore.InitV3Etcd(t)
	lgr, err := zap.NewDevelopment()
	require.NoError(t, err)
	mgr := NewSessionManager(client, lgr)
	assert.Equal(t, lgr, mgr.logger)
	assert.Equal(t, sessionManagerRetryDelay, mgr.retryDelay)
	assert.NotNil(t, mgr.get)
	assert.NotNil(t, mgr.close)
	session, err := mgr.newSession()
	require.NoError(t, err)
	session.Close()
	mgr.Close()
}
