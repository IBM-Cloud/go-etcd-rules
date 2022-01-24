package teststore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	v3 "go.etcd.io/etcd/client/v3"
)

// InitV3Etcd initializes etcd for test cases
func InitV3Etcd(t *testing.T) (v3.Config, *v3.Client) {
	cfg := v3.Config{
		Endpoints: []string{"http://127.0.0.1:2379"},
	}
	c, _ := v3.New(cfg)
	_, err := c.Delete(context.Background(), "/", v3.WithPrefix())
	require.NoError(t, err)
	return cfg, c
}
