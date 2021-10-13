package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
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
	IncLockMetric("etcd", "getKey", "/key/pattern", true)
	IncLockMetric("etcd", "getKey", "/second/pattern", false)

	checkMetrics(t, `rules_etcd_lock_count{locker="etcd",method="getKey",pattern="/key/pattern",success="true"} 1`)
	checkMetrics(t, `rules_etcd_lock_count{locker="etcd",method="getKey",pattern="/second/pattern",success="false"} 1`)
}

func TestIncSatisfiedThenNot(t *testing.T) {
	IncSatisfiedThenNot("getKey", "/key/pattern", "phaseName")
	checkMetrics(t, `rules_etcd_rule_satisfied_then_not{method="getKey",pattern="/key/pattern",phase="phaseName"} 1`)
}

func TestTimesEvaluated(t *testing.T) {
	TimesEvaluated("getKey", "rule1234", 5)
	checkMetrics(t, `rules_etcd_evaluations{method="getKey",rule="rule1234"} 5`)
}

func TestWokerQueueWaitTime(t *testing.T) {
	WorkerQueueWaitTime("getKey", time.Now())
	checkMetrics(t, `rules_etcd_worker_queue_wait_ms_count{method="getKey"} 1`)
}

func TestWorkBufferWaitTime(t *testing.T) {
	WorkBufferWaitTime("getKey", "/desired/key/pattern", time.Now())
	checkMetrics(t, `rules_etcd_work_buffer_wait_ms_count{method="getKey",pattern="/desired/key/pattern"} 1`)
}

func TestCallbackWaitTime(t *testing.T) {
	CallbackWaitTime("/desired/key/pattern", time.Now())
	checkMetrics(t, `rules_etcd_callback_wait_ms_count{pattern="/desired/key/pattern"} 1`)
}

func Test_keyProcessBufferCap(t *testing.T) {
	KeyProcessBufferCap(100)
	checkMetrics(t, `rules_etcd_key_process_buffer_cap 100`)
}

func Test_incWatcherErrMetric(t *testing.T) {
	IncWatcherErrMetric("err", "/desired/key/prefix")
	checkMetrics(t, `rules_etcd_watcher_errors{error="err",prefix="/desired/key/prefix"} 1`)
}
