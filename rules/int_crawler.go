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
	metrics MetricsCollector,
	logger *zap.Logger,
	mutex *string,
	mutexTTL int,
	mutexTimeout int,
	prefixes []string,
	kvWrapper WrapKV,
	delay int,
) (crawler, error) {
	kv := kvWrapper(cl)
	api := etcdV3ReadAPI{
		kV: kv,
	}
	c := intCrawler{
		api:                 &api,
		cl:                  cl,
		interval:            interval,
		kp:                  kp,
		metrics:             metrics,
		logger:              logger,
		mutex:               mutex,
		mutexTTL:            mutexTTL,
		mutexTimeout:        mutexTimeout,
		prefixes:            prefixes,
		kv:                  kv,
		delay:               delay,
		rulesProcessedCount: make(map[string]int),
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
	api          readAPI
	cancelFunc   context.CancelFunc
	cancelMutex  sync.Mutex
	cl           *clientv3.Client
	delay        int
	interval     int
	kp           extKeyProc
	kv           clientv3.KV
	metrics      MetricsCollector
	logger       *zap.Logger
	mutex        *string
	mutexTTL     int
	mutexTimeout int
	prefixes     []string
	stopped      uint32
	stopping     uint32
	// tracks the number of times a rule is processed in a single run
	rulesProcessedCount map[string]int
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
				zap.String("mutex", mutex), zap.Int("TTL", ic.mutexTTL), zap.Int("Timeout", ic.mutexTimeout))
			locker := newV3Locker(ic.cl, ic.mutexTimeout)
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
	crawlerMethodName := "crawler"
	if ic.isStopping() {
		return
	}
	//logger := ic.logger.With(zap.String("source", "crawler"))
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Duration(1)*time.Minute)
	defer cancelFunc()
	ctx = SetMethod(ctx, crawlerMethodName)
	ic.cancelMutex.Lock()
	ic.cancelFunc = cancelFunc
	ic.cancelMutex.Unlock()
	values := map[string]string{}
	// starting a new run so reset the rules processed count so we get reliable metrics
	ic.rulesProcessedCount = make(map[string]int)
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
	for ruleID, count := range ic.rulesProcessedCount {
		ic.metrics.TimesEvaluated(crawlerMethodName, ruleID, count)
	}
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
			ic.kp.processKey(k, &v, ic.api, logger, map[string]string{"source": "crawler"}, ic.incRuleProcessedCount)
		}
		time.Sleep(time.Duration(ic.delay) * time.Millisecond)
	}
}

func (ic *intCrawler) incRuleProcessedCount(ruleID string) {
	ic.rulesProcessedCount[ruleID] = ic.rulesProcessedCount[ruleID] + 1
}
