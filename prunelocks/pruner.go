package prunelocks

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

type Pruner struct {
	keys         map[string]lockKey
	timeout      time.Duration
	lockPrefixes []string
	kv           clientv3.KV
	lease        clientv3.Lease
	logger       *zap.Logger
}

func (p Pruner) checkLocks() {
	ctx := context.Background()
	for _, lockPrefix := range p.lockPrefixes {
		p.checkLockPrefix(ctx, lockPrefix, p.logger)
	}
}

func (p Pruner) checkLockPrefix(ctx context.Context, lockPrefix string, prefixLogger *zap.Logger) {
	keysRetrieved := make(map[string]bool)
	resp, _ := p.kv.Get(ctx, lockPrefix, clientv3.WithPrefix())
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
		key, found = p.keys[keyString]
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
			p.keys[keyString] = key
		}
		// Key seen before with same create revision
		now := time.Now()

		if now.Sub(key.firstSeen) < p.timeout {
			keyLogger.Info("Lock not expired")
		} else {
			keyLogger.Info("Lock expired; deleting key")
			resp, err := p.kv.Txn(ctx).If(clientv3.Compare(clientv3.CreateRevision(keyString), "=", key.createRevision)).Then(clientv3.OpDelete(keyString)).Commit()
			if err != nil {
				keyLogger.Error("error deleting key", zap.Error(err))
			} else {
				keyLogger.Info("deleted key", zap.Bool("succeeded", resp.Succeeded))
				if resp.Succeeded && kv.Lease != 0 {
					keyLogger.Error("revoking lease")
					_, err := p.lease.Revoke(ctx, clientv3.LeaseID(kv.Lease))
					if err != nil {
						keyLogger.Error("error revoking lease", zap.Error(err))
					} else {
						keyLogger.Info("revoked lease")
					}
				}
			}
		}
	}
	for keyString := range p.keys {
		if strings.HasPrefix(keyString, lockPrefix) && !keysRetrieved[keyString] {
			prefixLogger.Info("removing key from map", zap.String("key", keyString))
			delete(p.keys, keyString)
		}
	}
}
