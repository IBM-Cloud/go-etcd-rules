package rules

import (
	"github.com/coreos/etcd/clientv3"
	"go.uber.org/zap"
)

type keyProc interface {
	processKey(key string, value *string, api readAPI, logger *zap.Logger, metadata map[string]string)
}

type setableKeyProcessor interface {
	extKeyProc
	setCallback(int, interface{})
	setContextProvider(int, ContextProvider)
	setLockKeyPattern(int, string)
}

type workDispatcher interface {
	dispatchWork(index int, rule staticRule, logger *zap.Logger, keyPattern string, metadata map[string]string)
}

type baseKeyProcessor struct {
	contextProviders map[int]ContextProvider
	lockKeyPatterns  map[int]string
	rm               *ruleManager
}

func (bkp *baseKeyProcessor) setLockKeyPattern(index int, pattern string) {
	bkp.lockKeyPatterns[index] = pattern
}

func (bkp *baseKeyProcessor) setContextProvider(index int, cp ContextProvider) {
	bkp.contextProviders[index] = cp
}

type v3KeyProcessor struct {
	baseKeyProcessor
	callbacks map[int]V3RuleTaskCallback
	channel   chan v3RuleWork
	config    *clientv3.Config
}

func (v3kp *v3KeyProcessor) setCallback(index int, callback interface{}) {
	v3kp.callbacks[index] = callback.(V3RuleTaskCallback)
}

func (v3kp *v3KeyProcessor) dispatchWork(index int, rule staticRule, logger *zap.Logger, keyPattern string, metadata map[string]string) {
	context, cancelFunc := v3kp.contextProviders[index]()
	task := V3RuleTask{
		Attr: rule.getAttributes(),
		// This line is different
		Conf:     v3kp.config,
		Logger:   logger,
		Context:  context,
		cancel:   cancelFunc,
		Metadata: metadata,
	}
	work := v3RuleWork{
		rule:      rule,
		ruleIndex: index,
		ruleTask:  task,
		// This line is different
		ruleTaskCallback: v3kp.callbacks[index],
		lockKey:          FormatWithAttributes(keyPattern, rule.getAttributes()),
	}
	v3kp.channel <- work
}

func newV3KeyProcessor(channel chan v3RuleWork, config *clientv3.Config, rm *ruleManager) v3KeyProcessor {
	kp := v3KeyProcessor{
		baseKeyProcessor: baseKeyProcessor{
			contextProviders: map[int]ContextProvider{},
			lockKeyPatterns:  map[int]string{},
			rm:               rm,
		},
		callbacks: map[int]V3RuleTaskCallback{},
		channel:   channel,
		config:    config,
	}
	return kp
}

func (v3kp *v3KeyProcessor) processKey(key string, value *string, api readAPI, logger *zap.Logger, metadata map[string]string) {
	v3kp.baseKeyProcessor.processKey(key, value, api, logger, v3kp, metadata)
}

func (bkp *baseKeyProcessor) processKey(key string, value *string, api readAPI, logger *zap.Logger, dispatcher workDispatcher, metadata map[string]string) {
	logger.Debug("Processing key", zap.String("key", key))
	rules := bkp.rm.getStaticRules(key, value)
	for rule, index := range rules {
		satisfied, _ := rule.satisfied(api)
		if satisfied {
			keyPattern, ok := bkp.lockKeyPatterns[index]
			if !ok {
				logger.Error("Unable to find key pattern for rule", zap.Int("index", index))
				continue
			}
			dispatcher.dispatchWork(index, rule, logger, keyPattern, metadata)
		}
	}
}

func (bkp *baseKeyProcessor) isWork(key string, value *string, api readAPI) bool {
	rules := bkp.rm.getStaticRules(key, value)
	for rule := range rules {
		satisfied, _ := rule.satisfied(api)
		if satisfied {
			return true
		}
	}
	return false
}
