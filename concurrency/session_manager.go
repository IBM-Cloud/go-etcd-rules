package concurrency

import (
	"fmt"
	"sync"
	"time"

	"go.etcd.io/etcd/clientv3"
	"go.uber.org/zap"
)

type SessionManager struct {
	client  *clientv3.Client
	logger  *zap.Logger
	session *Session
	mutex   sync.Mutex
	err     error
}

// NewSessionManager creates a new session manager that will return an error if the
// attempt to get a session fails or return a session manager instance that will
// create new sessions if the existing one dies.
func NewSessionManager(client *clientv3.Client, logger *zap.Logger) (*SessionManager, error) {
	sm := &SessionManager{
		client: client,
		logger: logger,
	}
	err := sm.initSession()
	return sm, err
}

func (sm *SessionManager) initSession() error {
	sm.logger.Info("Initializing session")
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.session, sm.err = NewSession(sm.client)
	if sm.err != nil {
		sm.logger.Error("error initializing session", zap.Error(sm.err))
		return sm.err
	}
	sm.logger.Info("new session initialized", zap.String("lease_id", fmt.Sprintf("%x", sm.session.Lease())))
	sessionDone := sm.session.Done()
	go func() {
		time.Sleep(time.Minute)
		// Create a new session if the session dies, most likely due to an etcd
		// server issue.
		<-sessionDone
		err := sm.initSession()
		for err != nil {
			// If getting a new session fails, retry unti it succeeds.
			// Attempts to get the managed session will fail quickly, which
			// seems to be best alternative.
			time.Sleep(time.Second * 10)
			err = sm.initSession()
		}
	}()
	return nil
}

func (sm *SessionManager) GetSession() (*Session, error) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	return sm.session, sm.err
}
