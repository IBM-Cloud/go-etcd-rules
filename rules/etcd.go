package rules

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/clientv3"
	"golang.org/x/net/context"
)

type baseReadAPI struct {
	cancelFunc context.CancelFunc
}

func (bra *baseReadAPI) getContext() context.Context {
	var ctx context.Context
	ctx, bra.cancelFunc = context.WithTimeout(context.Background(), time.Duration(60)*time.Second)
	ctx = SetMethod(ctx, "rule_eval")
	return ctx
}

func (bra *baseReadAPI) cancel() {
	bra.cancelFunc()
}

type etcdReadAPI struct {
	baseReadAPI
	keysAPI  client.KeysAPI
	noQuorum bool
}

func (edra *etcdReadAPI) get(key string) (*string, error) {
	ctx := edra.getContext()
	defer edra.cancel()
	options := &client.GetOptions{Quorum: true}
	if edra.noQuorum {
		options = nil
	}
	resp, err := edra.keysAPI.Get(ctx, key, options)
	if err != nil {
		if !strings.HasPrefix(err.Error(), "100") {
			return nil, err
		}
		return nil, nil
	}
	return &resp.Node.Value, nil
}

type etcdV3ReadAPI struct {
	baseReadAPI
	kV clientv3.KV
}

func (edv3ra *etcdV3ReadAPI) get(key string) (*string, error) {
	ctx := edv3ra.baseReadAPI.getContext()
	defer edv3ra.cancel()
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

type keyWatcher interface {
	next() (string, *string, error)
	cancel()
}

func newEtcdKeyWatcher(api client.KeysAPI, prefix string, timeout time.Duration) keyWatcher {
	w := api.Watcher(prefix, &client.WatcherOptions{
		Recursive: true,
	})
	watcher := etcdKeyWatcher{
		baseKeyWatcher: baseKeyWatcher{
			prefix:  prefix,
			timeout: timeout,
		},
		api: api,
		w:   w,
	}
	return &watcher
}

func newEtcdV3KeyWatcher(watcher clientv3.Watcher, prefix string, timeout time.Duration) *etcdV3KeyWatcher {
	_, cancel := context.WithCancel(context.Background())
	kw := etcdV3KeyWatcher{
		baseKeyWatcher: baseKeyWatcher{
			prefix:     prefix,
			timeout:    timeout,
			cancelFunc: cancel,
		},
		w:      watcher,
		stopCh: make(chan bool),
	}
	return &kw
}

type baseKeyWatcher struct {
	cancelFunc  context.CancelFunc
	cancelMutex sync.Mutex
	prefix      string
	timeout     time.Duration
	stopping    uint32
}

func (bkw *baseKeyWatcher) getContext() context.Context {
	ctx := context.Background()
	bkw.cancelMutex.Lock()
	defer bkw.cancelMutex.Unlock()
	if bkw.timeout > 0 {
		ctx, bkw.cancelFunc = context.WithTimeout(ctx, bkw.timeout)
	} else {
		ctx, bkw.cancelFunc = context.WithCancel(ctx)
	}
	ctx = SetMethod(ctx, "watcher")
	return ctx
}

type etcdKeyWatcher struct {
	baseKeyWatcher
	api client.KeysAPI
	w   client.Watcher
}

func (ekw *etcdKeyWatcher) next() (string, *string, error) {
	resp, err := ekw.w.Next(ekw.getContext())
	if err != nil {
		// Get a new watcher to clear the event index
		ekw.w = ekw.api.Watcher(ekw.prefix, &client.WatcherOptions{
			Recursive: true,
		})
		return "", nil, err
	}
	node := resp.Node
	if resp.Action == "delete" || resp.Action == "expire" {
		return node.Key, nil, nil
	}
	return node.Key, &node.Value, nil
}

func (bkw *baseKeyWatcher) cancel() {
	atomicSet(&bkw.stopping, true)
	bkw.cancelMutex.Lock()
	defer bkw.cancelMutex.Unlock()
	if bkw.cancelFunc != nil {
		bkw.cancelFunc()
		bkw.cancelFunc = nil
	}
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

func (ev3kw *etcdV3KeyWatcher) next() (string, *string, error) {
	if ev3kw.ch == nil {
		ev3kw.ch = ev3kw.w.Watch(context.Background(), ev3kw.prefix, clientv3.WithPrefix())
	}
	for ev3kw.events == nil || len(ev3kw.events) == 0 {
		select {
		case <-ev3kw.stopCh:
			ev3kw.cancelFunc()
			err := ev3kw.w.Close()
			if err == nil {
				err = errors.New("Watcher closing")
			}
			return "", nil, err
		case wr := <-ev3kw.ch:
			// If there is an error, the logic appears to always
			// close the channel, so there is no need to try to
			// close it here.
			if err := wr.Err(); err != nil {
				ev3kw.ch = nil
				return "", nil, err
			}
			ev3kw.events = wr.Events
		}
	}
	event := ev3kw.events[0]
	ev3kw.events = ev3kw.events[1:]
	key := string(event.Kv.Key)
	if event.Type == clientv3.EventTypeDelete { // Covers lease expiration
		return key, nil, nil
	}
	val := string(event.Kv.Value)
	return key, &val, nil
}
