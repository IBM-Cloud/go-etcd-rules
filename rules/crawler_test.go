package rules

import (
	"testing"

	"github.com/coreos/etcd/client"
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
		kapi:   kapi,
		kp:     &kp,
		logger: getTestLogger(),
		prefix: "/root",
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
	)
	assert.NoError(t, err)
	_, err = newCrawler(
		client.Config{},
		getTestLogger(),
		"/root",
		5,
		&kp,
	)
	assert.Error(t, err)
}
