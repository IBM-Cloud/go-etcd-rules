package rules

import (
	"sync"
	"time"

	v3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"golang.org/x/net/context"

	"github.com/IBM-Cloud/go-etcd-rules/internal/jitter"
	"github.com/IBM-Cloud/go-etcd-rules/metrics"
	"github.com/IBM-Cloud/go-etcd-rules/rules/lock"
)

func newIntCrawler(
	cl *v3.Client,
	interval jitter.DurationGenerator,
	kp extKeyProc,
	metrics MetricsCollector,
	logger *zap.Logger,
	mutex *string,
	mutexTimeout int,
	prefixes []string,
	kvWrapper WrapKV,
	delay jitter.DurationGenerator,
	locker lock.RuleLocker,
	name string,
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
		name:                name,
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
	name         string
	api          readAPI
	cancelFunc   context.CancelFunc
	cancelMutex  sync.Mutex
	cl           *v3.Client
	delay        jitter.DurationGenerator
	interval     jitter.DurationGenerator
	kp           extKeyProc
	kv           v3.KV
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
		logger := ic.logger.With(zap.String("source", "crawler"))
		if ic.mutex == nil {
			ic.singleRun(logger)
		} else {
			mutex := "/crawler/" + *ic.mutex
			logger.Info("Attempting to obtain mutex",
				zap.String("mutex", mutex), zap.Int("Timeout", ic.mutexTimeout))
			lock, err := ic.locker.Lock(mutex, lock.MethodForLock("crawler"), lock.PatternForLock(mutex))
			if err != nil {
				logger.Error("Could not obtain mutex; skipping crawler run", zap.Error(err), zap.String("mutex", mutex))
			} else {
				logger.Info("Crawler mutex obtained", zap.String("mutex", mutex))
				ic.singleRun(logger)
				err := lock.Unlock()
				if err != nil {
					logger.Error("Could not unlock mutex", zap.Error(err), zap.String("mutex", mutex))
				}
			}
		}
		intervalSeconds := int(ic.interval.Generate().Seconds())
		logger.Debug("Pausing before next crawler run", zap.Int("wait_time_seconds", intervalSeconds))
		for i := 0; i < intervalSeconds; i++ {
			time.Sleep(time.Second)
			if ic.isStopping() {
				break
			}
		}
	}
	atomicSet(&ic.stopped, true)
}

func (ic *intCrawler) singleRun(logger *zap.Logger) {
	crawlerStart := time.Now()
	logger.Info("Starting crawler run", zap.Int("prefixes", len(ic.prefixes)))
	crawlerMethodName := "crawler"
	if ic.isStopping() {
		return
	}
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Duration(15)*time.Minute)
	defer cancelFunc()
	ic.cancelMutex.Lock()
	ic.cancelFunc = cancelFunc
	ic.cancelMutex.Unlock()
	values := map[string]string{}
	// starting a new run so reset the rules processed count so we get reliable metrics
	ic.rulesProcessedCount = make(map[string]int)

	queryStart := time.Now()
	for _, prefix := range ic.prefixes {
		pCtx := SetMethod(ctx, crawlerMethodName+"-"+prefix)
		resp, err := ic.kv.Get(pCtx, prefix, v3.WithPrefix())
		if err != nil {
			logger.Error("Error retrieving prefix", zap.String("prefix", prefix), zap.Error(err))
			return
		}
		for _, kv := range resp.Kvs {
			values[string(kv.Key)] = string(kv.Value)
		}
	}
	metrics.CrawlerQueryTime(ic.name, queryStart)
	metrics.CrawlerValuesCount(ic.name, len(values))
	evalStart := time.Now()
	ic.processData(values, logger)
	metrics.CrawlerEvalTime(ic.name, evalStart)

	ic.metricMutex.Lock()
	defer ic.metricMutex.Unlock()
	for ruleID, count := range ic.rulesProcessedCount {
		metrics.TimesEvaluated(crawlerMethodName, ruleID, count)
		ic.metrics.TimesEvaluated(crawlerMethodName, ruleID, count)
	}
	logger.Info("Crawler run complete", zap.Duration("time", time.Since(crawlerStart)), zap.Int("values", len(values)))
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
		time.Sleep(ic.delay.Generate())
	}
}

func (ic *intCrawler) incRuleProcessedCount(ruleID string) {
	ic.metricMutex.Lock()
	defer ic.metricMutex.Unlock()
	ic.rulesProcessedCount[ruleID] = ic.rulesProcessedCount[ruleID] + 1
}
