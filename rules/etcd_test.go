package rules

import (
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

func initV3Etcd() (clientv3.Config, *clientv3.Client) {
	cfg := clientv3.Config{
		Endpoints: []string{"http://127.0.0.1:2379"},
	}
	c, _ := clientv3.New(cfg)
	c.Delete(context.Background(), "/", clientv3.WithPrefix())
	return cfg, c
}

func TestV3EtcdReadAPI(t *testing.T) {
	_, c := initV3Etcd()
	kV := clientv3.NewKV(c)
	api := etcdV3ReadAPI{kV: kV}

	kV.Put(context.Background(), "/test0", "value")

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

func TestEctdV3Watcher(t *testing.T) {
	_, cl := initV3Etcd()
	w := clientv3.NewWatcher(cl)
	watcher := newEtcdV3KeyWatcher(w, "/pre", time.Duration(60)*time.Second)
	done := make(chan bool)
	go checkWatcher1(done, t, watcher)
	time.Sleep(time.Duration(3) * time.Second)
	cl.Put(context.Background(), "/pre/test", "value")
	<-done
	go checkWatcher2(done, t, watcher)
	time.Sleep(time.Duration(3) * time.Second)
	cl.Delete(context.Background(), "/pre/test")
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
