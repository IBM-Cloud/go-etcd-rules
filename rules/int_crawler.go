package rules

import (
	"sync"
	"time"

	"go.etcd.io/etcd/clientv3"
	"go.uber.org/zap"
	"golang.org/x/net/context"

	"github.com/IBM-Cloud/go-etcd-rules/metrics"
	"github.com/IBM-Cloud/go-etcd-rules/rules/lock"
)

func newIntCrawler(
	cl *clientv3.Client,
	interval int,
	kp extKeyProc,
	metrics MetricsCollector,
	logger *zap.Logger,
	mutex *string,
	mutexTimeout int,
	prefixes []string,
	kvWrapper WrapKV,
	delay int,
	locker lock.RuleLocker,
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
		mutexTimeout:        mutexTimeout,
		prefixes:            prefixes,
		kv:                  kv,
		delay:               delay,
		rulesProcessedCount: make(map[string]int),
		locker:              locker,
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

func (cra *cacheReadAPI) getCachedAPI(keys []string) (readAPI, error) {
	return cra, nil
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
	mutexTimeout int
	prefixes     []string
	stopped      uint32
	stopping     uint32
	// tracks the number of times a rule is processed in a single run
	rulesProcessedCount map[string]int
	metricMutex         sync.Mutex
	locker              lock.RuleLocker
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
				zap.String("mutex", mutex), zap.Int("Timeout", ic.mutexTimeout))
			lock, err := ic.locker.Lock(mutex)
			if err != nil {
				logger.Debug("Could not obtain mutex; skipping crawler run", zap.Error(err))
			} else {
				ic.singleRun(logger)
				err := lock.Unlock()
				if err != nil {
					logger.Error("Could not unlock mutex", zap.Error(err))
				}
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
	ic.cancelMutex.Lock()
	ic.cancelFunc = cancelFunc
	ic.cancelMutex.Unlock()
	values := map[string]string{}
	// starting a new run so reset the rules processed count so we get reliable metrics
	ic.rulesProcessedCount = make(map[string]int)
	for _, prefix := range ic.prefixes {
		pCtx := SetMethod(ctx, crawlerMethodName+"-"+prefix)
		resp, err := ic.kv.Get(pCtx, prefix, clientv3.WithPrefix())
		if err != nil {
			logger.Error("Error retrieving prefix", zap.String("prefix", prefix), zap.Error(err))
			return
		}
		for _, kv := range resp.Kvs {
			values[string(kv.Key)] = string(kv.Value)
		}
	}
	ic.processData(values, logger)
	ic.metricMutex.Lock()
	defer ic.metricMutex.Unlock()
	for ruleID, count := range ic.rulesProcessedCount {
		metrics.TimesEvaluated(crawlerMethodName, ruleID, count)
		ic.metrics.TimesEvaluated(crawlerMethodName, ruleID, count)
	}
}
func (ic *intCrawler) processData(values map[string]string, logger *zap.Logger) {
	api := &cacheReadAPI{values: values}
	for k := range values {
		v := values[k]
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
	ic.metricMutex.Lock()
	defer ic.metricMutex.Unlock()
	ic.rulesProcessedCount[ruleID] = ic.rulesProcessedCount[ruleID] + 1
}
