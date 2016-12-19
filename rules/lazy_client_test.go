package rules

import (
	"testing"
	"time"

	"github.com/coreos/etcd/client"
	"github.com/stretchr/testify/assert"
)

func TestLazyClient(t *testing.T) {
	cfg, _, _ := initEtcd()
	attr := mapAttributes{map[string]string{"a": "b"}}

	lc := NewLazyClient(&cfg, &attr, time.Duration(5)*time.Second)
	err0 := lc.LazySet("/:a/attr0", "val0")
	assert.NoError(t, err0)
	val0, err1 := lc.LazyGet("/:a/attr0")
	assert.NoError(t, err1)
	assert.NotNil(t, val0)
	if val0 != nil {
		assert.Equal(t, "val0", *val0)
	}
	nodes, err2 := lc.List("/b", nil)
	assert.NoError(t, err2)
	assert.Equal(t, 1, len(nodes))

	badCfg := client.Config{}
	badLc := NewLazyClient(&badCfg, &attr, time.Duration(5)*time.Second)
	err3 := badLc.LazySet("/:a/attr0", "val0")
	assert.Error(t, err3)
}
