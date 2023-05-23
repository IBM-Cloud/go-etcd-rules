package rules

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewMetricsInfo(t *testing.T) {
	ctx := context.Background()
	testCases := []struct {
		name               string
		ctx                context.Context
		pattern            string
		expectedMethodName string
	}{
		{
			name:               "empty_context",
			ctx:                ctx,
			expectedMethodName: notSetMethodName,
		},
		{
			name:               "method_set",
			ctx:                SetMethod(ctx, "testMethod"),
			expectedMethodName: "testMethod",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			startTime := time.Now()
			mi := newMetricsInfo(tc.ctx, tc.pattern, startTime)
			assert.Equal(t, tc.expectedMethodName, mi.method)
			assert.Equal(t, tc.pattern, mi.keyPattern)
			assert.True(t, time.Since(mi.startTime) < (1*time.Minute))
		})
	}
}

func TestNewMetricsCollector(t *testing.T) {
	m := newMetricsCollector()
	m.IncLockMetric("test", "/test/:key", false)
	m.IncSatisfiedThenNot("test", "/test/:key", "phase2")
	m.TimesEvaluated("test", "testRule1", 3)
	m.WorkerQueueWaitTime("test", time.Now())
}
