package rules

import (
	"github.com/coreos/etcd/client"
	"github.com/uber-go/zap"
)

type Attributes interface {
	GetAttribute(string) *string
	Format(string) string
}

type RuleTask struct {
	Attr   Attributes
	Conf   client.Config
	Logger zap.Logger
}

type RuleTaskCallback func(task *RuleTask)

type ruleWork struct {
	ruleTask         RuleTask
	ruleTaskCallback RuleTaskCallback
	ruleIndex        int
	lockKey          string
	rule             staticRule
}

func dummyCallback(task *RuleTask) {
}
