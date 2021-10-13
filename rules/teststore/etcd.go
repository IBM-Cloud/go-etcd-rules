package teststore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/clientv3"
)

// InitV3Etcd initializes etcd for test cases
func InitV3Etcd(t *testing.T) (clientv3.Config, *clientv3.Client) {
	cfg := clientv3.Config{
		Endpoints: []string{"http://127.0.0.1:2379"},
	}
	c, _ := clientv3.New(cfg)
	_, err := c.Delete(context.Background(), "/", clientv3.WithPrefix())
	require.NoError(t, err)
	return cfg, c
}
