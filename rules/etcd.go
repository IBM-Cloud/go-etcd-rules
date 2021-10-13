package rules

import (
	"errors"
	"sync"
	"time"

	"github.com/IBM-Cloud/go-etcd-rules/metrics"
	"go.etcd.io/etcd/mvcc/mvccpb"

	"go.etcd.io/etcd/clientv3"
	"golang.org/x/net/context"
)

type etcdV3ReadAPI struct {
	kV clientv3.KV
}

// This method is currently not used but is being kept around to limit
// the blast radius of implementing batch gets for rule evaluations.
// The arrangement of interfaces is not ideal and should be addressed
// once time permits.
func (edv3ra *etcdV3ReadAPI) get(key string) (*string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	ctx = SetMethod(ctx, "rule_eval")
	resp, err := edv3ra.kV.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if resp.Count == 0 {
		return nil, nil
	}
	val := string(resp.Kvs[0].Value)
	return &val, nil
}

func (edv3ra *etcdV3ReadAPI) getCachedAPI(keys []string) (readAPI, error) {
	ctx, cancel := context.WithTimeout(SetMethod(context.Background(), "rule_eval"), time.Minute)
	defer cancel()
	uniqueKeys := make(map[string]bool)
	// Get rid of duplicates
	for _, key := range keys {
		uniqueKeys[key] = true
	}

	ops := make([]clientv3.Op, len(uniqueKeys))
	idx := 0
	for key := range uniqueKeys {
		ops[idx] = clientv3.OpGet(key)
		idx++
	}
	// An etcd transaction consists of four parts:
	// 1. A set of "if" conditions.  The empty set evaluates to true, as in this case.
	// 2. A set of "then" operations which are performed if the "if" condition is true.
	//    In this case these operations are always performed.
	// 3. A set of "else" operations which are performed if the "if" condition is not true.
	//    In this case these operations are never performed.
	// 4. A commit, which finalizes the transaction and commits it. In this case everything is a read,
	//    so "commiting" doesn't really do anything to change the state of etcd.
	txnResp, err := edv3ra.kV.Txn(ctx).If().Then(ops...).Else().Commit()
	if err != nil {
		return nil, err
	}
	values := make(map[string]string)
	for _, resp := range txnResp.Responses {
		r := resp.GetResponseRange()
		for _, kv := range r.Kvs {
			key := string(kv.Key)
			val := string(kv.Value)
			values[key] = val
		}
	}

	return &cacheReadAPI{
		values: values,
	}, nil
}

type keyWatcher interface {
	next() (string, *string, error)
	cancel()
}

func newEtcdV3KeyWatcher(watcher clientv3.Watcher, prefix string, timeout time.Duration, metrics AdvancedMetricsCollector) *etcdV3KeyWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	kw := etcdV3KeyWatcher{
		baseKeyWatcher: baseKeyWatcher{
			prefix:      prefix,
			timeout:     timeout,
			ctx:         ctx,
			cancelFunc:  cancel,
			cancelMutex: sync.Mutex{},
			metrics:     metrics,
		},
		w:      watcher,
		stopCh: make(chan bool),
	}
	kw.metrics.ObserveWatchEvents(prefix, 0, 0)
	return &kw
}

type baseKeyWatcher struct {
	ctx         context.Context
	cancelFunc  context.CancelFunc
	cancelMutex sync.Mutex
	prefix      string
	timeout     time.Duration
	metrics     AdvancedMetricsCollector
}

type etcdV3KeyWatcher struct {
	baseKeyWatcher
	ch     clientv3.WatchChan
	stopCh chan bool
	events []*clientv3.Event
	w      clientv3.Watcher
}

func (ev3kw *etcdV3KeyWatcher) cancel() {
	ev3kw.stopCh <- true
}

func (ev3kw *etcdV3KeyWatcher) reset() {
	ev3kw.cancelMutex.Lock()
	defer ev3kw.cancelMutex.Unlock()

	// Cancel existing watcher channel to release resources
	ev3kw.cancelFunc()
	// Force a new watcher channel to be created with new context
	ev3kw.ch = nil
	ev3kw.ctx, ev3kw.cancelFunc = context.WithCancel(context.Background())
}

func (ev3kw *etcdV3KeyWatcher) next() (string, *string, error) {
	if ev3kw.ch == nil {
		ev3kw.ch = ev3kw.w.Watch(ev3kw.ctx, ev3kw.prefix, clientv3.WithPrefix())
	}
	if ev3kw.events == nil || len(ev3kw.events) == 0 {
		select {
		case <-ev3kw.stopCh:
			ev3kw.reset()
			err := ev3kw.w.Close()
			if err == nil {
				err = errors.New("Watcher closing")
			}
			return "", nil, err
		case wr, stillOpen := <-ev3kw.ch:
			var err error
			// Check if channel is closed without
			// an event with an error having been
			// added.
			if !stillOpen {
				err = errors.New("Channel closed")
			}
			// If there is an error, the logic appears to always
			// close the channel, so there is no need to try to
			// close it here.
			if err == nil {
				err = wr.Err()
			}
			if err != nil {
				// There is a fixed set of possible errors.
				// See https://github.com/etcd-io/etcd/blob/release-3.4/clientv3/watch.go#L115-L126
				metrics.IncWatcherErrMetric(err.Error(), ev3kw.prefix)
				ev3kw.reset()
				return "", nil, err
			}
			ev3kw.events = wr.Events
		}
	}
	if ev3kw.events == nil || len(ev3kw.events) == 0 {
		ev3kw.reset()
		return "", nil, errors.New("No events received from watcher channel; instantiating new channel")
	}

	// observe event metrics when they are received
	var events, size int
	for _, event := range ev3kw.events {
		events++
		size += (*mvccpb.Event)(event).Size()
	}
	ev3kw.metrics.ObserveWatchEvents(ev3kw.prefix, events, size)

	event := ev3kw.events[0]
	ev3kw.events = ev3kw.events[1:]
	key := string(event.Kv.Key)
	if event.Type == clientv3.EventTypeDelete { // Covers lease expiration
		return key, nil, nil
	}
	val := string(event.Kv.Value)
	return key, &val, nil
}
