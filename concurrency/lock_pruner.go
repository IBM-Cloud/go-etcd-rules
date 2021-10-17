package concurrency

import (
	"context"
	"strings"
	"time"

	"go.etcd.io/etcd/clientv3"
	"go.uber.org/zap"
)

type lockKey struct {
	createRevision int64
	firstSeen      time.Time
}

type LockPruner struct {
	keys               map[string]lockKey
	timeout            time.Duration
	lockPrefixes       []string
	kv                 clientv3.KV
	logger             *zap.Logger
	deleteLockKey      func(ctx context.Context, key string, createRevision int64, keyLogger *zap.Logger) (success bool)
	observeExpiredLock func(prefix string)
}

func NewLockPruner(timeout time.Duration, lockPrefixes []string, kv clientv3.KV, observeExpiredLock func(prefix string), logger *zap.Logger) LockPruner {
	lp := LockPruner{
		keys:               make(map[string]lockKey),
		timeout:            timeout,
		lockPrefixes:       lockPrefixes,
		kv:                 kv,
		logger:             logger,
		observeExpiredLock: observeExpiredLock,
	}
	// Top-ten question from code reviews: why isn't this set as part of the literal above?
	// Answer: because it requires referencing the struct instance "lp", which isn't defined
	// at that point.
	lp.deleteLockKey = lp.runtimeDeleteLockKey
	return lp
}

func (lp LockPruner) PruneLocks() {
	ctx := context.Background()
	for _, lockPrefix := range lp.lockPrefixes {
		prefixLogger := lp.logger.With(zap.String("prefix", lockPrefix))
		lp.checkLockPrefix(ctx, lockPrefix, prefixLogger)
	}
}

func (lp LockPruner) checkLockPrefix(ctx context.Context, lockPrefix string, prefixLogger *zap.Logger) {
	prefixLogger.Info("Checking prefix")
	keysRetrieved := make(map[string]bool)
	resp, err := lp.kv.Get(ctx, lockPrefix, clientv3.WithPrefix())
	if err != nil {
		prefixLogger.Error("error performing prefix query; aborting lock check", zap.Error(err))
		return
	}
	for _, kv := range resp.Kvs {
		// There are three possibilities:
		// 1. This lock was not seen before
		// 2. This lock was seen but has a different create revision
		// 3. This lock was seen and has the same create revision
		keyString := string(kv.Key)
		keysRetrieved[keyString] = true
		keyLogger := prefixLogger.With(zap.String("key", keyString), zap.Int64("create_revision", kv.CreateRevision), zap.Int64("lease", kv.Lease))
		keyLogger.Info("Found lock")
		var key lockKey
		var found bool
		// Key not seen before or seen before with different create revision
		key, found = lp.keys[keyString]
		keyLogger = keyLogger.With(zap.Bool("found", found))
		if found {
			keyLogger = keyLogger.With(zap.String("first_seen", key.firstSeen.Format(time.RFC3339)), zap.Int64("existing_create_revision", key.createRevision))
		}
		if !found || kv.CreateRevision != key.createRevision {
			keyLogger.Info("creating new key entry")
			key = lockKey{
				createRevision: kv.CreateRevision,
				firstSeen:      time.Now(),
			}
			lp.keys[keyString] = key
			continue
		}
		// Key seen before with same create revision
		now := time.Now()

		if now.Sub(key.firstSeen) < lp.timeout {
			keyLogger.Info("Lock not expired")
		} else {
			keyLogger.Info("Lock expired; deleting key")
			if lp.deleteLockKey(ctx, keyString, key.createRevision, keyLogger) {
				// Observe the expired key as a metric
				lp.observeExpiredLock(lockPrefix)
				// Remove the key from the keys map, since it is no longer in etcd
				delete(lp.keys, keyString)
			}
		}
	}
	for keyString := range lp.keys {
		if strings.HasPrefix(keyString, lockPrefix) && !keysRetrieved[keyString] {
			prefixLogger.Info("removing key from map", zap.String("key", keyString))
			delete(lp.keys, keyString)
		}
	}
}

func (lp LockPruner) runtimeDeleteLockKey(ctx context.Context, key string, createRevision int64, keyLogger *zap.Logger) (success bool) {
	// Only delete the key if the create revision matches.
	// Break up the txn build to be able to isolate panic source by line number
	txn := lp.kv.Txn(ctx)
	txn = txn.If(clientv3.Compare(clientv3.CreateRevision(key), "=", createRevision))
	txn = txn.Then(clientv3.OpDelete(key))
	resp, err := txn.Commit()
	if err != nil {
		keyLogger.Error("error deleting key", zap.Error(err))
		return false
	} else {
		keyLogger.Info("deleted key", zap.Bool("txn_succeeded", resp.Succeeded))
		// If the delete did not succeed because a key with a newer create revision
		// was added or another client deleted the key, the caller should still behave
		// as if the delete had succeeded, since in either case the key metadata is no
		// longer accurate and will get updated in the next run.
		return true
	}
}
