package rules

import (
	"errors"
	"testing"

	"github.com/coreos/etcd/client"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

func TestCrawler(t *testing.T) {
	child := client.Node{
		Key:   "/root/child",
		Value: "val1",
	}
	children := []*client.Node{&child}
	nodes := map[string]client.Node{
		"/root": client.Node{
			Dir:   true,
			Key:   "/root",
			Nodes: children,
		},
		"/root/child": child,
	}
	kapi := testKeyAPI{
		nodes: nodes,
	}
	kp := testKeyProcessor{
		keys: []string{},
	}
	cr := etcdCrawler{
		kapi:   &kapi,
		kp:     &kp,
		logger: getTestLogger(),
		prefix: "/root",
	}
	cr.singleRun()
	assert.Equal(t, "/root/child", kp.keys[0])
	cr.prefix = "/notroot"
	cr.singleRun()

	_, err := newCrawler(
		client.Config{
			Endpoints: []string{"http://192.168.1.204:4001"},
		},
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

type testKeyAPI struct {
	nodes map[string]client.Node
}

func (tka *testKeyAPI) Get(ctx context.Context, key string, opts *client.GetOptions) (*client.Response, error) {
	node, ok := tka.nodes[key]
	if !ok {
		return nil, errors.New("100 Not found")
	}
	resp := client.Response{
		Node: &node,
	}
	return &resp, nil
}
