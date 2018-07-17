package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

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
	cr.singleRun(getTestLogger())
	assert.Equal(t, "/root/child", kp.keys[0])
	cr.prefix = "/notroot"
	cr.singleRun(getTestLogger())
	_, err := newV3Crawler(
		cfg,
		5,
		&kp,
		getTestLogger(),
		nil,
		0,
		"/root",
		defaultWrapKV,
		c,
	)
	assert.NoError(t, err)
}
