package rules

import (
	"github.com/coreos/etcd/client"
	"github.com/uber-go/zap"
)

// Attributes provide access to the key/value pairs associated
// with dynamic keys.  For instance, a dynamic key "/static/:dynamic"
// that is matched against "/static/value1" would contain an yield
// an attribute with the key "dynamic" and the value "value1".
type Attributes interface {
	GetAttribute(string) *string
	Format(string) string
}

// RuleTask instances contain contextual object instances and metadata
// for use by rule callbacks.
type RuleTask struct {
	Attr   Attributes
	Conf   client.Config
	Logger zap.Logger
}

// RuleTaskCallback is the function type for functions that are called
// as a result of a specified rule being satisfied.
type RuleTaskCallback func(task *RuleTask)

type ruleWork struct {
	ruleTask         RuleTask
	ruleTaskCallback RuleTaskCallback
	ruleIndex        int
	lockKey          string
	rule             staticRule
}
