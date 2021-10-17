package concurrency

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/clientv3"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/IBM-Cloud/go-etcd-rules/rules/teststore"
)

func check(err error) {
	if err != nil {
		panic(err.Error())
	}
}

func xTest_Blah(t *testing.T) {
	// ctx := context.Background()
	cfg := clientv3.Config{Endpoints: []string{"http://127.0.0.1:2379"}}
	cl, err := clientv3.New(cfg)
	check(err)
	kv := clientv3.NewKV(cl)
	// resp, err := kv.Get(ctx, "/locks", clientv3.WithPrefix())
	// check(err)
	// for _, kv := range resp.Kvs {
	// 	fmt.Printf("%v\n", kv)
	// }
	p := LockPruner{
		keys:    make(map[string]lockKey),
		timeout: time.Minute,
		kv:      kv,
		// lease:        clientv3.NewLease(cl),
		logger:       zaptest.NewLogger(t),
		lockPrefixes: []string{"/locks/hello"},
	}
	for i := 0; i < 10; i++ {
		p.checkLocks()
		time.Sleep(10 * time.Second)
	}
}

type errorKV struct {
	clientv3.KV
}

func (ek errorKV) Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	return nil, errors.New("get error")
}

func (ek errorKV) Txn(_ context.Context) clientv3.Txn {
	return errorTxn{}
}

type errorTxn struct {
	clientv3.Txn
}

func (et errorTxn) If(cs ...clientv3.Cmp) clientv3.Txn {
	return et
}

func (et errorTxn) Then(ops ...clientv3.Op) clientv3.Txn {
	return et
}

func (et errorTxn) Commit() (*clientv3.TxnResponse, error) {
	return nil, errors.New("commit error")
}

func Test_LockPruner_checkLockPrefix(t *testing.T) {
	const (
		testLockPrefix = "/locks/prefix/"
		testKey1       = testLockPrefix + "1"
		testKey2       = testLockPrefix + "2"
		testTimeout    = time.Hour
	)
	ctx := context.Background()
	now := time.Now()
	testCases := []struct {
		name string
		// The value is the offset relative to the
		// create revision and is subtracted from the
		// actual create revision; see below for an explanation.
		currentKeys       map[string]int64
		expectedRemaining []string
		seenKeys          map[string]lockKey
		// The revisions are relative to the create revision
		// and are subtracted like the currentKeys values.
		updatedSeenKeys map[string]lockKey
		deleteFailure   bool
		getFailure      bool
		observeExpired  bool
	}{
		{
			// No locks in etcd and none previously seen
			name:              "clean_slate",
			expectedRemaining: []string{},
			seenKeys:          map[string]lockKey{},
			updatedSeenKeys:   map[string]lockKey{},
		},
		{
			// A lock that was not previously seen
			name: "not_seen_before",
			currentKeys: map[string]int64{
				testKey1: 0,
			},
			expectedRemaining: []string{testKey1},
			seenKeys:          map[string]lockKey{},
			updatedSeenKeys: map[string]lockKey{
				testKey1: {
					createRevision: 0,
					firstSeen:      now,
				},
			},
		},
		{
			// A previously seen lock that has expired and should be deleted
			name: "expired",
			currentKeys: map[string]int64{
				testKey1: 0,
			},
			expectedRemaining: []string{},
			seenKeys: map[string]lockKey{
				testKey1: {
					firstSeen: now.Add(-(testTimeout + time.Second)),
				},
			},
			updatedSeenKeys: map[string]lockKey{},
			observeExpired:  true,
		},
		{
			// A previously seen lock that has not yet expired
			name: "not_expired",
			currentKeys: map[string]int64{
				testKey1: 0,
			},
			expectedRemaining: []string{testKey1},
			seenKeys: map[string]lockKey{
				testKey1: {
					firstSeen: now.Add(-(testTimeout - time.Minute)),
				},
			},
			updatedSeenKeys: map[string]lockKey{
				testKey1: {
					createRevision: 0,
					firstSeen:      now.Add(-(testTimeout - time.Minute)),
				},
			},
		},
		{
			// A previously seen lock that was replaced by
			// a newer instance
			name: "replaced_since_seen",
			currentKeys: map[string]int64{
				testKey1: 1, // Older revision
			},
			expectedRemaining: []string{testKey1},
			seenKeys: map[string]lockKey{
				testKey1: {
					firstSeen: now.Add(-(testTimeout + time.Second)),
				},
			},
			updatedSeenKeys: map[string]lockKey{
				testKey1: {
					createRevision: 0,
					firstSeen:      now,
				},
			},
		},
		{
			// A previously seen lock that is no longer in etcd
			name: "gone",
			currentKeys: map[string]int64{
				testKey1: 0,
			},
			expectedRemaining: []string{testKey1},
			seenKeys: map[string]lockKey{
				testKey1: {
					firstSeen: now.Add(-(testTimeout - time.Minute)),
				},
				testKey2: {
					firstSeen: now.Add(-(testTimeout - time.Minute)),
				},
			},
			updatedSeenKeys: map[string]lockKey{
				testKey1: {
					createRevision: 0,
					firstSeen:      now.Add(-(testTimeout - time.Minute)),
				},
			},
		},
		{
			// Failure to retrieve keys; nothing should change
			name: "get_failure",
			currentKeys: map[string]int64{
				testKey1: 0,
			},
			expectedRemaining: []string{testKey1},
			seenKeys: map[string]lockKey{
				testKey1: {
					firstSeen: now.Add(-(testTimeout + time.Second)),
				},
			},
			updatedSeenKeys: map[string]lockKey{
				testKey1: {
					createRevision: 0,
					firstSeen:      now.Add(-(testTimeout + time.Second)),
				},
			},
			getFailure: true,
		},
		{
			// A previously seen lock that has expired but fails to be deleted
			name: "expired_with_delete_failure",
			currentKeys: map[string]int64{
				testKey1: 0,
			},
			expectedRemaining: []string{testKey1},
			seenKeys: map[string]lockKey{
				testKey1: {
					firstSeen: now.Add(-(testTimeout + time.Second)),
				},
			},
			updatedSeenKeys: map[string]lockKey{
				testKey1: {
					createRevision: 0,
					firstSeen:      now.Add(-(testTimeout + time.Second)),
				},
			},
			deleteFailure: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			observeExpired := false
			// Set up preconditions
			_, cl := teststore.InitV3Etcd(t)
			kv := clientv3.NewKV(cl)
			ops := make([]clientv3.Op, 0, len(tc.currentKeys))
			for key := range tc.currentKeys {
				ops = append(ops, clientv3.OpPut(key, ""))
			}
			txnResp, err := kv.Txn(ctx).Then(ops...).Commit()
			require.NoError(t, err)
			// The etcd revision of the update
			var revision int64
			for _, resp := range txnResp.Responses {
				pr := resp.GetResponsePut()
				revision = pr.Header.Revision
				break
			}
			seenKeys := make(map[string]lockKey)
			for key, metadata := range tc.seenKeys {
				// Set the revision with the specified offset.
				// The offset is to simulate an older lock with the same key
				// which would happen if a lock was unlocked and a new instance
				// obtained between checks. The logic distinguishes these kinds of
				// lock by their create revisions. The value associated with a lock
				// key should never change in etcd. A zero offset means that
				// this is still the same lock that was observed previously.
				metadata.createRevision = revision - tc.currentKeys[key]
				seenKeys[key] = metadata
			}
			lp := LockPruner{
				timeout: testTimeout,
				keys:    seenKeys,
				observeExpiredLock: func(prefix string) {
					assert.Equal(t, testLockPrefix, prefix)
					observeExpired = true
				},
			}
			if tc.getFailure {
				lp.kv = errorKV{}
			} else {
				lp.kv = kv
			}
			if tc.deleteFailure {
				lp.deleteLockKey = func(ctx context.Context, key string, createRevision int64, keyLogger *zap.Logger) (success bool) {
					return false
				}
			} else {
				lp.deleteLockKey = lp.runtimeDeleteLockKey
			}
			// Execute code
			lp.checkLockPrefix(ctx, testLockPrefix, zaptest.NewLogger(t))
			// Check results
			resp, err := kv.Get(ctx, testLockPrefix, clientv3.WithPrefix())
			require.NoError(t, err)
			assert.Equal(t, tc.observeExpired, observeExpired)
			remaining := make([]string, 0, len(resp.Kvs))
			for _, result := range resp.Kvs {
				remaining = append(remaining, string(result.Key))
			}
			sort.Strings(remaining)
			assert.EqualValues(t, tc.expectedRemaining, remaining)
			// Verify that the observed key map was correctly updated.
			for key, metadata := range seenKeys {
				expected, ok := tc.updatedSeenKeys[key]
				if assert.True(t, ok, "missing expected key entry in keys map: %s", key) {
					// Verify the timestamp
					assert.WithinDuration(t, expected.firstSeen, metadata.firstSeen, time.Second)
					// Set values to be equal, since they've been verified and didn't have to match exactly.
					expected.firstSeen = metadata.firstSeen
					// Apply the revision offset, which works like tc.currentKeys.
					expected.createRevision = revision - expected.createRevision
					seenKeys[key] = metadata

					assert.Equal(t, expected, metadata)
				}
			}
			// Verify that no extraneous keys are in tc.updatedSeenKeys
			for key := range tc.updatedSeenKeys {
				_, ok := seenKeys[key]
				assert.True(t, ok, "extraneous expected key entry in keys map: %s", key)
			}
		})
	}
}

func Test_LockPruner_runtimeDeleteLockKey(t *testing.T) {
	const (
		testKey = "/locks/test/key1"
	)
	ctx := context.Background()
	testCases := []struct {
		name           string
		expected       bool
		setKey         bool
		keyAbsent      bool
		commitError    bool
		revisionOffset int64
	}{
		{
			// The key was deleted from etcd
			name:      "key_deleted",
			expected:  true,
			setKey:    true,
			keyAbsent: true,
		},
		{
			// The key was not deleted because the one in etcd
			// is from a different revision
			name:           "different_revision",
			expected:       true,
			setKey:         true,
			revisionOffset: 1,
		},
		{
			// The key is no longer present in etcd because another
			// client deleted it or its lease expired
			name:           "key_missing",
			expected:       true,
			keyAbsent:      true,
			revisionOffset: 1,
		},
		{
			// There is an error committing the transaction
			name:        "commit_error",
			expected:    false,
			commitError: true,
			keyAbsent:   true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lgr := zaptest.NewLogger(t)
			_, cl := teststore.InitV3Etcd(t)
			kv := clientv3.NewKV(cl)
			lp := LockPruner{}
			if tc.commitError {
				lp.kv = errorKV{}
			} else {
				lp.kv = kv
			}
			var revision int64
			if tc.setKey {
				resp, err := kv.Put(ctx, testKey, "")
				require.NoError(t, err)
				revision = resp.Header.Revision
			}

			result := lp.runtimeDeleteLockKey(ctx, testKey, revision+tc.revisionOffset, lgr)
			assert.Equal(t, tc.expected, result)
			resp, err := kv.Get(ctx, testKey)
			require.NoError(t, err)
			assert.Equal(t, tc.keyAbsent, len(resp.Kvs) == 0, "unexpected etcd state")
		})
	}
}
