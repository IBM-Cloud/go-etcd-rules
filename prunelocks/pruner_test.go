package prunelocks

import (
	"testing"
	"time"

	"go.etcd.io/etcd/clientv3"
	"go.uber.org/zap/zaptest"
)

func check(err error) {
	if err != nil {
		panic(err.Error())
	}
}

func Test_Blah(t *testing.T) {
	// ctx := context.Background()
	cfg := clientv3.Config{Endpoints: []string{"http://127.0.0.1:2379"}}
	cl, err := clientv3.New(cfg)
	check(err)
	kv := clientv3.NewKV(cl)
	// resp, err := kv.Get(ctx, "/locks", clientv3.WithPrefix())
	// check(err)
	// for _, kv := range resp.Kvs {
	// 	fmt.Printf("%v\n", kv)
	// }
	p := Pruner{
		keys:         make(map[string]lockKey),
		timeout:      time.Minute,
		kv:           kv,
		lease:        clientv3.NewLease(cl),
		logger:       zaptest.NewLogger(t),
		lockPrefixes: []string{"/locks/hello"},
	}
	for i := 0; i < 10; i++ {
		p.checkLocks()
		time.Sleep(10 * time.Second)
	}
}
