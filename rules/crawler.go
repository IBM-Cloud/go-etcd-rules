package rules

import (
	"sync"
	"time"

	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/clientv3"
	"github.com/uber-go/zap"
	"golang.org/x/net/context"
)

type crawler interface {
	run()
	stop()
	isStopped() bool
}

func newCrawler(
	config client.Config,
	logger zap.Logger,
	prefix string,
	interval int,
	kp keyProc,
	wrapKeysAPI WrapKeysAPI,
	delay int,
) (crawler, error) {
	blank := etcdCrawler{}
	cl, err1 := client.New(config)
	if err1 != nil {
		return &blank, err1
	}
	kapi := wrapKeysAPI(client.NewKeysAPI(cl))
	api := etcdReadAPI{
		keysAPI: kapi,
	}
	c := etcdCrawler{
		baseCrawler: baseCrawler{
			api:      &api,
			interval: interval,
			kp:       kp,
			logger:   logger,
			prefix:   prefix,
			delay:    delay,
		},
		kapi: kapi,
	}
	return &c, nil
}

func newV3Crawler(
	config clientv3.Config,
	interval int,
	kp keyProc,
	logger zap.Logger,
	mutex *string,
	mutexTTL int,
	prefix string,
	kvWrapper WrapKV,
) (crawler, error) {
	blank := etcdCrawler{}
	cl, err1 := clientv3.New(config)
	if err1 != nil {
		return &blank, err1
	}
	kv := kvWrapper(clientv3.NewKV(cl))
	api := etcdV3ReadAPI{
		kV: kv,
	}
	c := v3EtcdCrawler{
		baseCrawler: baseCrawler{
			api:      &api,
			interval: interval,
			kp:       kp,
			logger:   logger,
			mutex:    mutex,
			mutexTTL: mutexTTL,
			prefix:   prefix,
		},
		cl: cl,
		kv: kv,
	}
	return &c, nil
}

type baseCrawler struct {
	api         readAPI
	cancelFunc  context.CancelFunc
	cancelMutex sync.Mutex
	interval    int
	delay       int
	kp          keyProc
	logger      zap.Logger
	mutex       *string
	mutexTTL    int
	prefix      string
	stopping    uint32
	stopped     uint32
}

func (bc *baseCrawler) isStopping() bool {
	return is(&bc.stopping)
}

func (bc *baseCrawler) stop() {
	atomicSet(&bc.stopping, true)
	bc.cancelMutex.Lock()
	defer bc.cancelMutex.Unlock()
	if bc.cancelFunc != nil {
		bc.cancelFunc()
	}
}

func (bc *baseCrawler) isStopped() bool {
	return is(&bc.stopped)
}

type etcdCrawler struct {
	baseCrawler
	kapi client.KeysAPI
}

func (ec *etcdCrawler) run() {
	atomicSet(&ec.stopped, false)
	for !ec.isStopping() {
		ec.logger.Info("Starting crawler run")
		ec.singleRun()
		ec.logger.Info("Crawler run complete")
		for i := 0; i < ec.interval; i++ {
			time.Sleep(time.Second)
			if ec.isStopping() {
				break
			}
		}
	}
	atomicSet(&ec.stopped, true)
}

func (ec *etcdCrawler) singleRun() {
	ec.crawlPath(ec.prefix)
}

func (ec *etcdCrawler) crawlPath(path string) {
	if ec.isStopping() {
		return
	}
	ctx := context.Background()
	ctx = SetMethod(ctx, "crawler")
	time.Sleep(time.Millisecond * time.Duration(ec.delay))
	resp, err := ec.kapi.Get(ctx, path, &client.GetOptions{Quorum: true})
	if err != nil {
		return
	}
	if resp.Node.Dir {
		for _, node := range resp.Node.Nodes {
			ec.crawlPath(node.Key)
		}
		return
	}
	node := resp.Node
	logger := ec.logger.With(zap.String("source", "crawler"))
	ec.kp.processKey(node.Key, &node.Value, ec.api, logger, map[string]string{"source": "crawler", "prefix": ec.prefix})
}

type v3EtcdCrawler struct {
	baseCrawler
	cl *clientv3.Client
	kv clientv3.KV
}

func (v3ec *v3EtcdCrawler) run() {
	atomicSet(&v3ec.stopped, false)
	for !v3ec.isStopping() {
		v3ec.logger.Debug("Starting crawler run")
		if v3ec.mutex == nil {
			v3ec.singleRun()
		} else {
			mutex := "/crawler/" + *v3ec.mutex + v3ec.prefix
			v3ec.logger.Debug("Attempting to obtain mutex",
				zap.String("mutex", mutex), zap.Int("TTL", v3ec.mutexTTL))
			locker := newV3Locker(v3ec.cl)
			lock, err := locker.lock(mutex, v3ec.mutexTTL)
			if err != nil {
				v3ec.logger.Debug("Could not obtain mutex; skipping crawler run", zap.Error(err))
			} else {
				v3ec.singleRun()
				lock.unlock()
			}
		}
		v3ec.logger.Debug("Crawler run complete")
		for i := 0; i < v3ec.interval; i++ {
			time.Sleep(time.Second)
			if v3ec.isStopping() {
				break
			}
		}
	}
	atomicSet(&v3ec.stopped, true)
}

func (v3ec *v3EtcdCrawler) singleRun() {
	if v3ec.isStopping() {
		return
	}
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Duration(1)*time.Minute)
	ctx = SetMethod(ctx, "crawler")
	v3ec.cancelMutex.Lock()
	v3ec.cancelFunc = cancelFunc
	v3ec.cancelMutex.Unlock()
	resp, err := v3ec.kv.Get(ctx, v3ec.prefix, clientv3.WithPrefix())
	if err != nil {
		return
	}
	logger := v3ec.logger.With(zap.String("source", "crawler"))
	for _, kv := range resp.Kvs {
		if v3ec.isStopping() {
			return
		}
		value := string(kv.Value[:])
		v3ec.kp.processKey(string(kv.Key[:]), &value, v3ec.api, logger, map[string]string{"source": "crawler", "prefix": v3ec.prefix})
	}
}
