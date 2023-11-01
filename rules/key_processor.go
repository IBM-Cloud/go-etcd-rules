package rules

import (
	"fmt"
	"time"

	"github.com/IBM-Cloud/go-etcd-rules/metrics"
	"go.uber.org/zap"
)

type keyProc interface {
	// the timesEvaluated parameter is used to increment the number of times a rule is processed, the
	// data is then used by the metrics collector
	processKey(key string, value *string, api readAPI, logger *zap.Logger, metadata map[string]string,
		timesEvaluated func(rulesID string))
}

type setableKeyProcessor interface {
	extKeyProc
	setCallback(int, interface{})
	setContextProvider(int, ContextProvider)
	setLockKeyPattern(int, string)
	setRuleID(index int, ruleID string)
}

type workDispatcher interface {
	dispatchWork(index int, rule staticRule, logger *zap.Logger, keyPattern string, metadata map[string]string, ruleID string)
}

type baseKeyProcessor struct {
	contextProviders map[int]ContextProvider
	lockKeyPatterns  map[int]string
	ruleIDs          map[int]string
	rm               *ruleManager
}

func (bkp *baseKeyProcessor) setLockKeyPattern(index int, pattern string) {
	bkp.lockKeyPatterns[index] = pattern
}

func (bkp *baseKeyProcessor) setRuleID(index int, ruleID string) {
	bkp.ruleIDs[index] = ruleID
}

func (bkp *baseKeyProcessor) setContextProvider(index int, cp ContextProvider) {
	bkp.contextProviders[index] = cp
}

type v3KeyProcessor struct {
	baseKeyProcessor
	callbacks    map[int]V3RuleTaskCallback
	channel      chan v3RuleWork
	kpChannel    chan *keyTask
	lastNotified int
}

func (v3kp *v3KeyProcessor) setCallback(index int, callback interface{}) {
	v3kp.callbacks[index] = callback.(V3RuleTaskCallback)
}

func (v3kp *v3KeyProcessor) dispatchWork(index int, rule staticRule, logger *zap.Logger, keyPattern string, metadata map[string]string, ruleID string) {
	task := V3RuleTask{
		Attr:     rule.getAttributes(),
		Logger:   logger,
		Metadata: metadata,
	}
	work := v3RuleWork{
		ruleID:           ruleID,
		rule:             rule,
		ruleIndex:        index,
		ruleTask:         task,
		ruleTaskCallback: v3kp.callbacks[index],
		lockKey:          FormatWithAttributes(keyPattern, rule.getAttributes()),

		// context info
		keyPattern:       keyPattern,
		metricsStartTime: time.Now(),
		contextProvider:  v3kp.contextProviders[index],
	}

	start := time.Now()
	v3kp.channel <- work
	// measures the amount of time work is blocked from being added to the buffer
	metrics.WorkBufferWaitTime(getMethodNameFromProvider(work.contextProvider), keyPattern, start)
}

func newV3KeyProcessor(channel chan v3RuleWork, rm *ruleManager, kpChannel chan *keyTask, concurrency int, logger *zap.Logger) v3KeyProcessor {
	kp := v3KeyProcessor{
		baseKeyProcessor: baseKeyProcessor{
			contextProviders: map[int]ContextProvider{},
			lockKeyPatterns:  map[int]string{},
			rm:               rm,
			ruleIDs:          make(map[int]string),
		},
		callbacks:    map[int]V3RuleTaskCallback{},
		channel:      channel,
		kpChannel:    kpChannel,
		lastNotified: -1,
	}
	logger.Info("Starting key processor workers", zap.Int("concurrency", concurrency))
	for i := 0; i < concurrency; i++ {
		go kp.keyWorker(logger)
	}
	go kp.bufferCapacitySampler(logger)
	return kp
}

func (v3kp *v3KeyProcessor) processKey(key string, value *string, api readAPI, logger *zap.Logger,
	metadata map[string]string, timesEvaluated func(rulesID string)) {
	logger.Debug("submitting key to be processed", zap.String("key", key))
	task := &keyTask{
		key:            key,
		value:          value,
		api:            api,
		logger:         logger,
		metadata:       metadata,
		timesEvaluated: timesEvaluated,
	}
	v3kp.kpChannel <- task
}

func (v3kp *v3KeyProcessor) bufferCapacitySampler(logger *zap.Logger) {
	for {
		remainingBuffer := cap(v3kp.kpChannel) - len(v3kp.kpChannel)
		metrics.KeyProcessBufferCap(remainingBuffer)
		currentHour := time.Now().UTC().Hour()
		if (float32(remainingBuffer)/float32(cap(v3kp.kpChannel))) < 0.05 && v3kp.lastNotified != currentHour {
			logger.Warn("Rules engine buffer is near capacity", zap.Int("capacity", cap(v3kp.kpChannel)), zap.Int("remaining", remainingBuffer))
			v3kp.lastNotified = currentHour
		}
		time.Sleep(time.Minute)
	}
}

func (v3kp *v3KeyProcessor) keyWorker(logger *zap.Logger) {
	logger.Info("Starting key worker")
	for {
		task := <-v3kp.kpChannel
		task.logger.Debug("Key processing task retrieved")
		v3kp.baseKeyProcessor.processKey(task.key, task.value, task.api, task.logger, v3kp, task.metadata, task.timesEvaluated)
	}
}

type keyTask struct {
	key            string
	value          *string
	api            readAPI
	logger         *zap.Logger
	metadata       map[string]string
	timesEvaluated func(rulesID string)
}

func (bkp *baseKeyProcessor) processKey(key string, value *string, rapi readAPI, logger *zap.Logger, dispatcher workDispatcher,
	metadata map[string]string, timesEvaluated func(rulesID string)) {
	logger.Debug("Processing key", zap.String("key", key))
	rules := bkp.rm.getStaticRules(key, value)
	valueString := "<nil>"
	if value != nil {
		valueString = *value
	}
	var keys []string
	for rule := range rules {
		keys = append(keys, rule.getKeys()...)
	}
	api, err := rapi.getCachedAPI(keys)
	if err != nil {
		logger.Error("Error getting keys to evaluate rules", zap.Error(err), zap.Int("rules", len(rules)), zap.Int("keys", len(keys)))
		return
	}
	for rule, index := range rules {
		ruleID := bkp.ruleIDs[index]
		if timesEvaluated != nil {
			timesEvaluated(ruleID)
		}
		satisfied, _ := rule.satisfied(api) // #nosec G104 -- Map lookup
		if logger.Core().Enabled(zap.DebugLevel) {
			logger.Debug("Rule evaluated", zap.Bool("satisfied", satisfied), zap.String("rule", rule.String()), zap.String("value", fmt.Sprintf("%.30s", valueString)), zap.String("key", key))
		}
		if satisfied {
			keyPattern, ok := bkp.lockKeyPatterns[index]
			if !ok {
				logger.Error("Unable to find key pattern for rule", zap.Int("index", index))
				continue
			}
			dispatcher.dispatchWork(index, rule, logger, keyPattern, metadata, ruleID)
		}
	}
}

func (bkp *baseKeyProcessor) isWork(key string, value *string, api readAPI) bool {
	rules := bkp.rm.getStaticRules(key, value)
	for rule := range rules {
		satisfied, _ := rule.satisfied(api) // #nosec G104 -- Map lookup
		if satisfied {
			return true
		}
	}
	return false
}
