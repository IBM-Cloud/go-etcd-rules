package rules

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v3 "go.etcd.io/etcd/client/v3"
	"golang.org/x/net/context"

	"github.com/IBM-Cloud/go-etcd-rules/rules/teststore"
)

func TestV3EtcdReadAPI(t *testing.T) {
	_, c := teststore.InitV3Etcd(t)
	kV := v3.NewKV(c)
	api := etcdV3ReadAPI{kV: kV}

	_, err := kV.Put(context.Background(), "/test0", "value")
	require.NoError(t, err)

	val, err := api.get("/test0")
	assert.NoError(t, err)
	_ = assert.NotNil(t, val) && assert.Equal(t, "value", *val)

	val, err = api.get("/test1")
	assert.NoError(t, err)
	assert.Nil(t, val)
}

func Test_etcdV3ReadAPI_getCachedAPI(t *testing.T) {
	_, c := teststore.InitV3Etcd(t)
	kV := v3.NewKV(c)
	api := etcdV3ReadAPI{kV: kV}

	_, err := kV.Put(context.Background(), "/test0", "value")
	require.NoError(t, err)

	cacheAPI, err := api.getCachedAPI([]string{"/test0"})

	require.NoError(t, err)

	val, err := cacheAPI.get("/test0")
	assert.NoError(t, err)
	_ = assert.NotNil(t, val) && assert.Equal(t, "value", *val)

	val, err = cacheAPI.get("/test1")
	assert.NoError(t, err)
	assert.Nil(t, val)
}

func TestEctdV3Watcher(t *testing.T) {
	_, cl := teststore.InitV3Etcd(t)
	w := v3.NewWatcher(cl)
	watcher := newEtcdV3KeyWatcher(w, "/pre", time.Duration(60)*time.Second, newMetricsCollector())
	done := make(chan bool)
	go checkWatcher1(done, t, watcher)
	time.Sleep(time.Duration(3) * time.Second)
	_, err := cl.Put(context.Background(), "/pre/test", "value")
	require.NoError(t, err)
	<-done
	go checkWatcher2(done, t, watcher)
	time.Sleep(time.Duration(3) * time.Second)
	_, err = cl.Delete(context.Background(), "/pre/test")
	require.NoError(t, err)
	<-done
}

func checkWatcher1(done chan bool, t *testing.T, watcher keyWatcher) {
	key, value, err := watcher.next()
	assert.NoError(t, err)
	assert.Equal(t, "/pre/test", key)
	_ = assert.NotNil(t, value) && assert.Equal(t, "value", *value)
	done <- true
}

func checkWatcher2(done chan bool, t *testing.T, watcher keyWatcher) {
	key, value, err := watcher.next()
	assert.NoError(t, err)
	assert.Equal(t, "/pre/test", key)
	assert.Nil(t, value)
	done <- true
}

func TestEctdV3WatcherCancel(t *testing.T) {
	_, cl := teststore.InitV3Etcd(t)
	w := v3.NewWatcher(cl)
	watcher := newEtcdV3KeyWatcher(w, "/pre", time.Duration(60)*time.Second, newMetricsCollector())
	done := make(chan bool)
	go checkWatcher3(done, t, watcher)
	time.Sleep(time.Duration(3) * time.Second)
	for i := 1; i <= 3; i++ {
		_, err := cl.Put(context.Background(), "/pre/test", "value")
		require.NoError(t, err)
	}
	time.Sleep(time.Duration(3) * time.Second)
	watcher.cancel()
	<-done
}

func checkWatcher3(done chan bool, t *testing.T, watcher keyWatcher) {
	for i := 1; i <= 3; i++ {
		key, value, err := watcher.next()
		assert.NoError(t, err)
		assert.Equal(t, "/pre/test", key)
		_ = assert.NotNil(t, value) && assert.Equal(t, "value", *value)
	}
	_, _, err := watcher.next()
	assert.EqualError(t, err, "Watcher closing")
	done <- true
}
