package concurrency

import (
	"context"
	"fmt"
	"sync"
	"time"

	v3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

const (
	sessionManagerRetryDelay = time.Second * 10
)

type SessionManager struct {
	// These fields must not be accessed by more than one
	// goroutine.
	// Session singleton that is refreshed if it closes.
	session *Session
	// Channel used by the session to communicate that it is closed.
	sessionDone <-chan struct{}

	logger     *zap.Logger
	retryDelay time.Duration
	get        chan sessionManagerGetRequest
	close      chan struct{}
	closeOnce  sync.Once
	newSession func() (*Session, error)
}

// NewSessionManager creates a new session manager that manages a session singleton
// that is replaced if it dies.
func NewSessionManager(client *v3.Client, logger *zap.Logger) *SessionManager {
	return newSessionManager(client, sessionManagerRetryDelay, logger)
}
func newSessionManager(client *v3.Client, retryDelay time.Duration, logger *zap.Logger) *SessionManager {
	sm := &SessionManager{
		logger:     logger,
		retryDelay: retryDelay,
		get:        make(chan sessionManagerGetRequest),
		close:      make(chan struct{}),
		newSession: func() (*Session, error) { return NewSession(client) },
	}
	go sm.run()
	return sm
}

// GetSession provides the singleton session or times out if a session
// cannot be obtained. The context needs to have a timeout, otherwise it
// is possible for the calling goroutine to hang.
func (sm *SessionManager) GetSession(ctx context.Context) (*Session, error) {
	request := sessionManagerGetRequest{
		resp: make(chan *Session),
	}
	go func() {
		sm.get <- request
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case session := <-request.resp:
		return session, nil
	}
}

// Close closes the manager, causing the current session to be closed
// and no new ones to be created.
func (sm *SessionManager) Close() {
	sm.closeOnce.Do(func() {
		close(sm.close)
	})
}

func (sm *SessionManager) resetSession() {
	sm.logger.Info("Initializing session")
	session, err := sm.newSession()
	for err != nil {
		sm.logger.Error("Error getting session", zap.Error(err))
		stopRetry := false
		func() {
			ctx, cancel := context.WithTimeout(context.Background(), sm.retryDelay)
			defer cancel()
			select {
			case <-ctx.Done():
				// Let pass so retry can be attempted.
			case <-sm.close:
				stopRetry = true
			}
		}()
		if stopRetry {
			return
		}
		session, err = sm.newSession()
	}
	sm.session = session
	sm.sessionDone = session.Done()
	sm.logger.Info("new session initialized", zap.String("lease_id", fmt.Sprintf("%x", sm.session.Lease())))
}

func (sm *SessionManager) run() {
	// Thread safety is handled by controlling all activity
	// through a single goroutine that interacts with other
	// goroutines via channels.
	sm.logger.Info("Starting session manager")
run:
	for {
		// If the session manager should be closed, give
		// that the highest priority.
		select {
		case <-sm.close:
			sm.logger.Info("Closing session manager")
			if sm.session != nil {
				// This may fail the session was already closed
				// due to some external cause, like etcd connectivity
				// issues. The result is just a log message.
				sm.session.Close()
			}
			break run
		default:
		}
		switch {
		case sm.sessionDone == nil:
			sm.resetSession()
			continue
		}
		// If the current session has closed,
		// prioritize creating a new one ahead
		// of remaining concerns.
		select {
		case <-sm.sessionDone:
			// Create new session
			sm.resetSession()
			continue
		default:
		}
		select {
		case <-sm.close:
			// Let the check above take care of cleanup
			continue
		case <-sm.sessionDone:
			// Let the check above take care of creating a new session
			continue
		case req := <-sm.get:
			// Get the current session
			req.resp <- sm.session
		}
	}
	sm.logger.Info("Session manager closed")
}

type sessionManagerGetRequest struct {
	resp chan *Session
}
