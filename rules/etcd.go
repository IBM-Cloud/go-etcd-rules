package rules

import (
	"strings"

	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

type etcdReadAPI struct {
	kAPI client.KeysAPI
}

func (edra *etcdReadAPI) get(key string) (*string, error) {
	ctx := edra.getContext()
	resp, err := edra.kAPI.Get(ctx, key, nil)
	if err != nil {
		if !strings.HasPrefix(err.Error(), "100") {
			return nil, err
		}
		return nil, nil
	}
	return &resp.Node.Value, nil
}

func (edra *etcdReadAPI) getContext() context.Context {
	return context.Background()
}

type keyWatcher interface {
	next() (string, *string, error)
}

func newEtcdKeyWatcher(api client.KeysAPI, prefix string) keyWatcher {
	w := api.Watcher(prefix, &client.WatcherOptions{
		Recursive: true,
	})
	watcher := etcdKeyWatcher{
		w: w,
	}
	return &watcher
}

type etcdKeyWatcher struct {
	w client.Watcher
}

func (ekw *etcdKeyWatcher) next() (string, *string, error) {
	resp, err := ekw.w.Next(ekw.getContext())
	if err != nil {
		return "", nil, err
	}
	node := resp.Node
	if resp.Action == "delete" || resp.Action == "expire" {
		return node.Key, nil, nil
	}
	return node.Key, &node.Value, nil
}

func (edra *etcdKeyWatcher) getContext() context.Context {
	return context.Background()
}
