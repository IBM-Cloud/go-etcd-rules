package rules

import (
	"time"

	"go.uber.org/zap"
	"golang.org/x/net/context"
)

// Attributes provide access to the key/value pairs associated
// with dynamic keys.  For instance, a dynamic key "/static/:dynamic"
// that is matched against "/static/value1" would contain an yield
// an attribute with the key "dynamic" and the value "value1".
// Attributes implementers should also implement AttributeFinder so that
// the more performant/explicit implementation can be used in internal functions
type Attributes interface {
	GetAttribute(string) *string
	Format(string) string
}

// AttributeFinder is a more performant replacement for the GetAttribute
// method of Attributes. Internal functions use the FindAttribute method
// if passed an implementation of Attributes that also implements AttributeFinder
type AttributeFinder interface {
	FindAttribute(string) (string, bool)
}

type extendedAttributes interface {
	Attributes
	AttributeFinder
	names() []string
}

// V3RuleTask instances contain contextual object instances and metadata
// for use by rule callbacks.
type V3RuleTask struct {
	Attr     Attributes
	Logger   *zap.Logger
	Context  context.Context
	cancel   context.CancelFunc
	Metadata map[string]string
}

// V3RuleTaskCallback is the function type for functions that are called
// as a reulst of a specified rule being satisfied using the etcd v3
// API.
type V3RuleTaskCallback func(task *V3RuleTask)

type v3RuleWork struct {
	//	baseWork
	rule             staticRule
	ruleID           string
	ruleTask         V3RuleTask
	ruleTaskCallback V3RuleTaskCallback
	ruleIndex        int
	lockKey          string
	// context handling
	keyPattern       string
	metricsStartTime time.Time
	contextProvider  ContextProvider
}
