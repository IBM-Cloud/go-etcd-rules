package rules

import (
	"sync/atomic"
)

func is(val *uint32) bool {
	return (atomic.LoadUint32(val)) == 1
}

func atomicSet(val *uint32, b bool) {
	var r uint32
	if b {
		r = 1
	} else {
		r = 0
	}
	atomic.StoreUint32(val, r)
}
