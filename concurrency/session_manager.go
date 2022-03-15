package concurrency

import (
	"context"
	"sync"
	"time"

	v3 "go.etcd.io/etcd/client/v3"
	v3c "go.etcd.io/etcd/client/v3/concurrency"

	"go.uber.org/zap"
)

const (
	sessionManagerRetryDelay = time.Second * 10
)

type SessionManager struct {
	// Session singleton that is cleared if it closes.
	session      *v3c.Session
	sessionMutex sync.Mutex

	logger     *zap.Logger
	newSession func() (*v3c.Session, error)
}

// NewSessionManager creates a new session manager that manages a session singleton
// that is replaced if it dies.
func NewSessionManager(client *v3.Client, logger *zap.Logger) *SessionManager {
	return newSessionManager(client, sessionManagerRetryDelay, logger)
}
func newSessionManager(client *v3.Client, retryDelay time.Duration, logger *zap.Logger) *SessionManager {
	sm := &SessionManager{
		logger:     logger,
		newSession: func() (*v3c.Session, error) { return v3c.NewSession(client) },
	}
	return sm
}

// GetSession provides the singleton session or times out if a session
// cannot be obtained. The context needs to have a timeout, otherwise it
// is possible for the calling goroutine to hang.

func (sm *SessionManager) GetSession(ctx context.Context) (*v3c.Session, error) {
	sm.sessionMutex.Lock()
	defer sm.sessionMutex.Unlock()
	if sm.session == nil {
		var err error
		sm.session, err = sm.newSession()
		if err != nil {
			return nil, err
		}
		sessionDone := sm.session.Done()
		// Start goroutine to check for closed session.
		go func() {
			<-sessionDone
			// Clear out dead session
			sm.sessionMutex.Lock()
			defer sm.sessionMutex.Unlock()
			sm.session = nil
		}()
	}
	return sm.session, nil
}
