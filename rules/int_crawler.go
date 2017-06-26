package rules

import (
	"sync"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/uber-go/zap"
	"golang.org/x/net/context"
)

func newIntCrawler(
	config clientv3.Config,
	interval int,
	kp extKeyProc,
	logger zap.Logger,
	mutex *string,
	mutexTTL int,
	prefixes []string,
	kvWrapper WrapKV,
	delay int,
) (crawler, error) {
	blank := etcdCrawler{}
	cl, err1 := clientv3.New(config)
	if err1 != nil {
		return &blank, err1
	}
	kv := kvWrapper(cl)
	api := etcdV3ReadAPI{
		kV: kv,
	}
	c := intCrawler{
		api:      &api,
		cl:       cl,
		interval: interval,
		kp:       kp,
		logger:   logger,
		mutex:    mutex,
		mutexTTL: mutexTTL,
		prefixes: prefixes,
		kv:       kv,
		delay:    delay,
	}
	return &c, nil
}

type extKeyProc interface {
	keyProc
	isWork(string, *string, readAPI) bool
}

type cacheReadAPI struct {
	values map[string]string
}

func (cra *cacheReadAPI) get(key string) (*string, error) {
	value, ok := cra.values[key]
	if !ok {
		return nil, nil
	}
	return &value, nil
}

type intCrawler struct {
	api         readAPI
	cancelFunc  context.CancelFunc
	cancelMutex sync.Mutex
	cl          *clientv3.Client
	delay       int
	interval    int
	kp          extKeyProc
	kv          clientv3.KV
	logger      zap.Logger
	mutex       *string
	mutexTTL    int
	prefixes    []string
	stopped     uint32
	stopping    uint32
}

func (ic *intCrawler) isStopping() bool {
	return is(&ic.stopping)
}

func (ic *intCrawler) stop() {
	atomicSet(&ic.stopping, true)
	ic.cancelMutex.Lock()
	defer ic.cancelMutex.Unlock()
	if ic.cancelFunc != nil {
		ic.cancelFunc()
	}
}

func (ic *intCrawler) isStopped() bool {
	return is(&ic.stopped)
}

func (ic *intCrawler) run() {
	atomicSet(&ic.stopped, false)
	for !ic.isStopping() {
		ic.logger.Debug("Starting crawler run")
		if ic.mutex == nil {
			ic.singleRun()
		} else {
			mutex := "/crawler/" + *ic.mutex
			ic.logger.Debug("Attempting to obtain mutex",
				zap.String("mutex", mutex), zap.Int("TTL", ic.mutexTTL))
			locker := newV3Locker(ic.cl)
			lock, err := locker.lock(mutex, ic.mutexTTL)
			if err != nil {
				ic.logger.Debug("Could not obtain mutex; skipping crawler run", zap.Error(err))
			} else {
				ic.singleRun()
				lock.unlock()
			}
		}
		ic.logger.Debug("Crawler run complete")
		for i := 0; i < ic.interval; i++ {
			time.Sleep(time.Second)
			if ic.isStopping() {
				break
			}
		}
	}
	atomicSet(&ic.stopped, true)
}

func (ic *intCrawler) singleRun() {
	if ic.isStopping() {
		return
	}
	logger := ic.logger.With(zap.String("source", "crawler"))
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Duration(1)*time.Minute)
	defer cancelFunc()
	ctx = SetMethod(ctx, "crawler")
	ic.cancelMutex.Lock()
	ic.cancelFunc = cancelFunc
	ic.cancelMutex.Unlock()
	values := map[string]string{}
	for _, prefix := range ic.prefixes {
		resp, err := ic.kv.Get(ctx, prefix, clientv3.WithPrefix())
		if err != nil {
			return
		}
		for _, kv := range resp.Kvs {
			values[string(kv.Key)] = string(kv.Value)
		}
	}
	api := &cacheReadAPI{values: values}
	for k, v := range values {
		if ic.isStopping() {
			return
		}
		// Check to see if any rule is satisfied from cache
		if ic.kp.isWork(k, &v, api) {
			// Process key if it is
			ic.kp.processKey(k, &v, ic.api, logger, map[string]string{"source": "crawler"})
		}
		time.Sleep(time.Duration(ic.delay) * time.Millisecond)
	}
}
