package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

type testExtKeyProcessor struct {
	testKeyProcessor
	workKeys map[string]string
	workTrue map[string]string
}

func (tekp *testExtKeyProcessor) isWork(key string, value *string, r readAPI) bool {
	_, ok := tekp.workTrue[key]
	tekp.workKeys[key] = ""
	return ok
}

func TestIntCrawler(t *testing.T) {
	_, c := initV3Etcd(t)
	kapi := c
	_, err := kapi.Put(context.Background(), "/root/child", "val1")
	require.NoError(t, err)
	_, err = kapi.Put(context.Background(), "/root1/child", "val1")
	require.NoError(t, err)
	_, err = kapi.Put(context.Background(), "/root2/child", "val1")
	require.NoError(t, err)

	kp := testExtKeyProcessor{
		testKeyProcessor: newTestKeyProcessor(),
		workTrue:         map[string]string{"/root/child": "", "/root2/child": ""},
		workKeys:         map[string]string{},
	}

	lgr, err := zap.NewDevelopment()
	assert.NoError(t, err)
	metrics := NewMockMetricsCollector()
	metrics.SetLogger(lgr)
	expectedRuleIDs := []string{"/root/child", "/root2/child"}
	expectedCount := []int{1, 1}
	expectedMethods := []string{"crawler", "crawler"}

	cr := intCrawler{
		kp:       &kp,
		logger:   getTestLogger(),
		prefixes: []string{"/root", "/root1"},
		kv:       c,
		metrics:  &metrics,
	}
	kp.setTimesEvalFunc(cr.incRuleProcessedCount)
	cr.singleRun(getTestLogger())
	assert.True(t, stringInArray("/root/child", kp.keys))
	assert.True(t, stringInArray("/root2/child", kp.keys))

	assert.Equal(t, map[string]string{
		"/root/child":  "",
		"/root1/child": "",
		"/root2/child": "",
	}, kp.workKeys)

	assert.True(t, stringInArray(expectedRuleIDs[0], metrics.TimesEvaluatedRuleID))
	assert.True(t, stringInArray(expectedRuleIDs[1], metrics.TimesEvaluatedRuleID))

	assert.Equal(t, expectedCount, metrics.TimesEvaluatedCount)
	assert.Equal(t, expectedMethods, metrics.TimesEvaluatedMethod)
}

func stringInArray(str string, arr []string) bool {
	for _, s := range arr {
		if str == s {
			return true
		}
	}
	return false
}
