package rules

import (
	"testing"
	"time"

	"github.com/coreos/etcd/client"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

func initEtcd() (client.Config, client.Client, client.KeysAPI) {
	cfg := client.Config{
		Endpoints: []string{"http://127.0.0.1:2379"},
	}
	c, _ := client.New(cfg)

	kapi := client.NewKeysAPI(c)

	resp, _ := kapi.Get(context.Background(), "/", nil)

	for _, node := range resp.Node.Nodes {
		kapi.Delete(context.Background(), node.Key, &client.DeleteOptions{Recursive: true, Dir: node.Dir})
	}

	return cfg, c, kapi
}

func TestEtcdReadAPI(t *testing.T) {

	_, _, kapi := initEtcd()

	api := etcdReadAPI{kAPI: kapi}

	kapi.Set(context.Background(), "/test0", "value", nil)

	val, err := api.get("/test0")
	assert.NoError(t, err)
	assert.NotNil(t, val)
	if val != nil {
		assert.Equal(t, "value", *val)
	}
	val, err = api.get("/test1")
	assert.NoError(t, err)
	assert.Nil(t, val)
}

func TestEctdWatcher(t *testing.T) {
	_, _, kapi := initEtcd()
	watcher := newEtcdKeyWatcher(kapi, "/pre", time.Duration(30)*time.Second)
	done := make(chan bool)
	go checkWatcher1(done, t, watcher)
	kapi.Set(context.Background(), "/pre/test", "value", nil)
	<-done
	go checkWatcher2(done, t, watcher)
	kapi.Delete(context.Background(), "/pre/test", nil)
	<-done
}

func checkWatcher1(done chan bool, t *testing.T, watcher keyWatcher) {
	key, value, err := watcher.next()
	assert.NoError(t, err)
	assert.Equal(t, "/pre/test", key)
	assert.NotNil(t, value)
	if value != nil {
		assert.Equal(t, "value", *value)
	}
	done <- true
}

func checkWatcher2(done chan bool, t *testing.T, watcher keyWatcher) {
	key, value, err := watcher.next()
	assert.NoError(t, err)
	assert.Equal(t, "/pre/test", key)
	assert.Nil(t, value)
	done <- true
}
