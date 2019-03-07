package rules

import (
	"sync"
	"time"

	"github.com/coreos/etcd/clientv3"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

func newIntCrawler(
	cl *clientv3.Client,
	interval int,
	kp extKeyProc,
	logger *zap.Logger,
	mutex *string,
	mutexTTL int,
	prefixes []string,
	kvWrapper WrapKV,
	delay int,
) (crawler, error) {
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
	logger      *zap.Logger
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
		logger := ic.logger.With(
			zap.String("source", "crawler"),
			zap.String("crawler_start", time.Now().Format("2006-01-02T15:04:05-0700")),
		)
		logger.Info("Starting crawler run")
		if ic.mutex == nil {
			ic.singleRun(logger)
		} else {
			mutex := "/crawler/" + *ic.mutex
			logger.Debug("Attempting to obtain mutex",
				zap.String("mutex", mutex), zap.Int("TTL", ic.mutexTTL))
			locker := newV3Locker(ic.cl)
			lock, err := locker.lock(mutex, ic.mutexTTL)
			if err != nil {
				logger.Debug("Could not obtain mutex; skipping crawler run", zap.Error(err))
			} else {
				ic.singleRun(logger)
				lock.unlock()
			}
		}
		logger.Info("Crawler run complete")
		for i := 0; i < ic.interval; i++ {
			time.Sleep(time.Second)
			if ic.isStopping() {
				break
			}
		}
	}
	atomicSet(&ic.stopped, true)
}

func (ic *intCrawler) singleRun(logger *zap.Logger) {
	if ic.isStopping() {
		return
	}

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
			logger.Error("Error retrieving prefix", zap.String("prefix", prefix), zap.Error(err))
			return
		}
		for _, kv := range resp.Kvs {
			values[string(kv.Key)] = string(kv.Value)
		}
	}
	ic.processData(values, logger)
}
func (ic *intCrawler) processData(values map[string]string, logger *zap.Logger) {
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
