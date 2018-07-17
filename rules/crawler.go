package rules

import (
	"sync"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/uber-go/zap"
	"golang.org/x/net/context"
)

type crawler interface {
	run()
	stop()
	isStopped() bool
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
	cl *clientv3.Client,
) (crawler, error) {
	kv := kvWrapper(cl)
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

type v3EtcdCrawler struct {
	baseCrawler
	cl *clientv3.Client
	kv clientv3.KV
}

func (v3ec *v3EtcdCrawler) run() {
	atomicSet(&v3ec.stopped, false)
	for !v3ec.isStopping() {
		logger := v3ec.logger.With(
			zap.String("source", "crawler"),
			zap.String("crawler_prefix", v3ec.prefix),
			zap.Object("crawler_start", time.Now()),
		)
		logger.Info("Crawler run starting")
		if v3ec.mutex == nil {
			v3ec.singleRun(logger)
		} else {
			mutex := "/crawler/" + *v3ec.mutex + v3ec.prefix
			logger.Debug("Attempting to obtain mutex",
				zap.String("mutex", mutex), zap.Int("TTL", v3ec.mutexTTL))
			locker := newV3Locker(v3ec.cl)
			lock, err := locker.lock(mutex, v3ec.mutexTTL)
			if err != nil {
				logger.Debug("Could not obtain mutex; skipping crawler run", zap.Error(err))
			} else {
				v3ec.singleRun(logger)
				lock.unlock()
			}
		}
		logger.Info("Crawler run complete")
		for i := 0; i < v3ec.interval; i++ {
			time.Sleep(time.Second)
			if v3ec.isStopping() {
				break
			}
		}
	}
	atomicSet(&v3ec.stopped, true)
}

func (v3ec *v3EtcdCrawler) singleRun(logger zap.Logger) {
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
		logger.Error("Crawler error", zap.Error(err))
		return
	}
	for _, kv := range resp.Kvs {
		if v3ec.isStopping() {
			return
		}
		value := string(kv.Value[:])
		v3ec.kp.processKey(string(kv.Key[:]), &value, v3ec.api, logger, map[string]string{"source": "crawler", "prefix": v3ec.prefix})
	}
}
