package rules

import (
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

var (
	testRouter *mux.Router
)

func initRouter() *mux.Router {
	r := mux.NewRouter()
	r.Handle("/metrics-go-etcd-rules", promhttp.Handler()).Methods("GET")
	return r
}

func init() {
	testRouter = initRouter()
}

func makeTestRequest(t *testing.T, request *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, request)
	return w
}

func checkMetrics(t *testing.T, expectedOutput string) {
	request, err := http.NewRequest("GET", "/metrics-go-etcd-rules", nil)
	assert.NoError(t, err, "Could not create http request with error")

	w := makeTestRequest(t, request)
	body := w.Body.String()
	assert.Contains(t, body, expectedOutput)
}

func TestIncLockMetric(t *testing.T) {
	incLockMetric("getKey", "/key/pattern", true)
	incLockMetric("getKey", "/second/pattern", false)

	checkMetrics(t, `rules_etcd_lock_count{method="getKey",pattern="/key/pattern",success="true"} 1`)
	checkMetrics(t, `rules_etcd_lock_count{method="getKey",pattern="/second/pattern",success="false"} 1`)
}

func TestIncSatisfiedThenNot(t *testing.T) {
	incSatisfiedThenNot("getKey", "/key/pattern", "phaseName")
	checkMetrics(t, `rules_etcd_rule_satisfied_then_not{method="getKey",pattern="/key/pattern",phase="phaseName"} 1`)
}

func TestTimesEvaluated(t *testing.T) {
	timesEvaluated("getKey", "rule1234", 5)
	checkMetrics(t, `rules_etcd_evaluations{method="getKey",rule="rule1234"} 5`)
}

func TestWokerQueueWaitTime(t *testing.T) {
	workerQueueWaitTime("getKey", time.Now())
	checkMetrics(t, `rules_etcd_worker_queue_wait_ms_count{method="getKey"} 1`)
}

func TestObserveWatchEvents(t *testing.T) {
	observeWatchEvents("key/prefix", 3, 2)
	observeWatchEvents("/prefix/2", 4, 4, metricOption{key: "service", value: "random-service"}, metricOption{key: "region", value: "random region"})
	observeWatchEvents("/prefix/3", 5, 5, metricOption{key: "key", value: "value"})

	checkMetrics(t, `data_etcd_operation_keys_count{action="watch",method="rules-engine-watcher",prefix="key/prefix",region="",service="",success="true"} 1`)
	checkMetrics(t, `data_etcd_operation_keys_count{action="watch",method="rules-engine-watcher",prefix="/prefix/2",region="random region",service="random-service",success="true"} 1`)
	checkMetrics(t, `data_etcd_operation_keys_count{action="watch",method="rules-engine-watcher",prefix="/prefix/3",region="",service="",success="true"} 1`)
}
