package rules

import (
	"github.com/coreos/etcd/client"
	"github.com/uber-go/zap"
)

type kProcessor interface {
	processKey(key string, value *string, api readAPI, logger zap.Logger)
}

type keyProcessor struct {
	callbacks       map[int]RuleTaskCallback
	channel         chan ruleWork
	config          client.Config
	lockKeyPatterns map[int]string
	rm              *ruleManager
}

func newKeyProcessor(channel chan ruleWork, config client.Config, rm *ruleManager) keyProcessor {
	kp := keyProcessor{
		callbacks:       map[int]RuleTaskCallback{},
		channel:         channel,
		config:          config,
		lockKeyPatterns: map[int]string{},
		rm:              rm,
	}
	return kp
}

func (kp *keyProcessor) processKey(key string, value *string, api readAPI, logger zap.Logger) {
	logger.Debug("Processing key", zap.String("key", key))
	rules := kp.rm.getStaticRules(key, value)
	for rule, index := range rules {
		satisfied, _ := rule.satisfied(api)
		if satisfied {
			task := RuleTask{
				Attr:   rule.getAttributes(),
				Conf:   kp.config,
				Logger: logger,
			}
			keyPattern, ok := kp.lockKeyPatterns[index]
			if !ok {
				logger.Error("Unable to find key pattern for rule", zap.Int("index", index))
				continue
			}
			work := ruleWork{
				ruleIndex:        index,
				ruleTask:         task,
				ruleTaskCallback: kp.callbacks[index],
				lockKey:          formatWithAttributes(keyPattern, rule.getAttributes()),
				rule:             rule,
			}
			kp.channel <- work
		}
	}
}
