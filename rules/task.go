package rules

import (
	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/clientv3"
	"github.com/uber-go/zap"
	"golang.org/x/net/context"
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
	Attr     Attributes
	Conf     client.Config
	Logger   zap.Logger
	Context  context.Context
	Metadata map[string]string
}

// V3RuleTask instances contain contextual object instances and metadata
// for use by rule callbacks.
type V3RuleTask struct {
	Attr     Attributes
	Conf     *clientv3.Config
	Logger   zap.Logger
	Context  context.Context
	Metadata map[string]string
}

// RuleTaskCallback is the function type for functions that are called
// as a result of a specified rule being satisfied.
type RuleTaskCallback func(task *RuleTask)

// V3RuleTaskCallback is the function type for functions that are called
// as a reulst of a specified rule being satisfied using the etcd v3
// API.
type V3RuleTaskCallback func(task *V3RuleTask)

//type baseWork struct {
//	attr Attributes
//	logger zap.Logger
//}

type ruleWork struct {
	//	baseWork
	rule             staticRule
	ruleTask         RuleTask
	ruleTaskCallback RuleTaskCallback
	ruleIndex        int
	lockKey          string
}

type v3RuleWork struct {
	//	baseWork
	rule             staticRule
	ruleTask         V3RuleTask
	ruleTaskCallback V3RuleTaskCallback
	ruleIndex        int
	lockKey          string
}
