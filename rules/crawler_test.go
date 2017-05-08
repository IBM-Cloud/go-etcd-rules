package rules

import (
	"testing"

	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/clientv3"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

func TestCrawler(t *testing.T) {
	cfg := client.Config{
		Endpoints: []string{"http://127.0.0.1:2379"},
	}

	c, _ := client.New(cfg)

	kapi := client.NewKeysAPI(c)

	kapi.Delete(context.Background(), "/", &client.DeleteOptions{Recursive: true})
	kapi.Set(context.Background(), "/root", "", &client.SetOptions{Dir: true})
	kapi.Set(context.Background(), "/root/child", "val1", nil)

	kp := testKeyProcessor{
		keys: []string{},
	}
	cr := etcdCrawler{
		baseCrawler: baseCrawler{
			kp:     &kp,
			logger: getTestLogger(),
			prefix: "/root",
		},
		kapi: kapi,
	}
	cr.singleRun()
	assert.Equal(t, "/root/child", kp.keys[0])
	cr.prefix = "/notroot"
	cr.singleRun()

	_, err := newCrawler(
		cfg,
		getTestLogger(),
		"/root",
		5,
		&kp,
		defaultWrapKeysAPI,
	)
	assert.NoError(t, err)
	_, err = newCrawler(
		client.Config{},
		getTestLogger(),
		"/root",
		5,
		&kp,
		defaultWrapKeysAPI,
	)
	assert.Error(t, err)
}

func TestV3Crawler(t *testing.T) {
	cfg, c := initV3Etcd()
	kapi := c
	kapi.Put(context.Background(), "/root/child", "val1")

	kp := testKeyProcessor{
		keys: []string{},
	}
	cr := v3EtcdCrawler{
		baseCrawler: baseCrawler{
			kp:     &kp,
			logger: getTestLogger(),
			prefix: "/root",
		},
		kv: c,
	}
	cr.singleRun()
	assert.Equal(t, "/root/child", kp.keys[0])
	cr.prefix = "/notroot"
	cr.singleRun()
	_, err := newV3Crawler(
		cfg,
		5,
		&kp,
		getTestLogger(),
		nil,
		0,
		"/root",
		defaultWrapKV,
	)
	assert.NoError(t, err)
	cfg = clientv3.Config{}
	_, err = newV3Crawler(
		cfg,
		5,
		&kp,
		getTestLogger(),
		nil,
		0,
		"/root",
		defaultWrapKV,
	)
	assert.Error(t, err)
}
