package concurrency

import (
	"context"
	"sort"
	"testing"
	"time"

	"go.etcd.io/etcd/clientv3"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/IBM-Cloud/go-etcd-rules/rules/teststore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		keys:         make(map[string]lockKey),
		timeout:      time.Minute,
		kv:           kv,
		lease:        clientv3.NewLease(cl),
		logger:       zaptest.NewLogger(t),
		lockPrefixes: []string{"/locks/hello"},
	}
	for i := 0; i < 10; i++ {
		p.checkLocks()
		time.Sleep(10 * time.Second)
	}
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
			// A previously seen lock that has expired
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
			updatedSeenKeys: map[string]lockKey{
				testKey1: {
					createRevision: 0,
					firstSeen:      now,
				},
			},
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
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
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
				kv:      kv,
				keys:    seenKeys,
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
		})
	}
}
