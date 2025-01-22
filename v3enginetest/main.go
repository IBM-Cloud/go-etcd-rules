package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/IBM-Cloud/go-etcd-rules/rules"
	v3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

var (
	idCount   = 4
	pollCount = int32(5)
)

const (
	dataPath   = "/rulesEngine/data/:id"
	blockPath  = "/rulesEngine/block/:id"
	donePath   = "/rulesEngine/done/:id"
	doneRuleID = "done"
	doneID     = "4567"
)

type polled struct {
	ID        string
	pollCount int32
}

func check(err error) {
	if err != nil {
		panic(err.Error())
	}
}

func checkWatchResp(resp []v3.WatchResponse) {
	// This number isn't deterministic, because it can't be known
	// how many watch events are captured in terms of TTLs and
	// when the engine finishes shutting down.
	if len(resp) < 3 {
		panic(fmt.Errorf("not the correct amount of responses returned from the watch channel: %d", len(resp)))
	}

	for _, r := range resp {
		if len(r.Events) != 1 {
			panic(fmt.Errorf("incorrect number of events for watch channel response"))
		}
	}
}

// Main
// Optional paramater for port "6969"
func main() {
	port := "6969"
	if len(os.Args) > 1 {
		port = os.Args[1]
	}
	logger, err := zap.NewDevelopment(zap.Fields(zap.String("port", port)))
	check(err)

	// cleanup etcd from previous runs
	cfg := v3.Config{Endpoints: []string{"http://127.0.0.1:2379"}}
	cl, err := v3.New(cfg)
	check(err)
	kv := v3.NewKV(cl)
	// Clear out any data that may interfere
	_, err = kv.Delete(context.Background(), "/rulesEngine", v3.WithPrefix())
	check(err)

	// build the rules engine options, include a metrics collector and
	// a context provider which includes a method named later used when
	// metrics are called
	metricsCollector := rules.NewMockMetricsCollector()
	metricsCollector.SetLogger(logger)
	mFunc := func() rules.MetricsCollector { return &metricsCollector }
	ctx, cancel := context.WithCancel(context.Background())
	ctx = rules.SetMethod(ctx, "intTest")
	cpFunc := func() (context.Context, context.CancelFunc) {
		return ctx, cancel
	}
	// Set up a callback handler
	cbHandler := rules.NewHTTPCallbackHander()
	http.HandleFunc("/callback", cbHandler.HandleRequest)
	go func() {
		err := http.ListenAndServe(":"+port, nil) // #nosec G114 - For testing
		check(err)
	}()

	// Set environment variable so the rules engine will use it
	os.Setenv(rules.WebhookURLEnv, "http://localhost:"+port+"/callback") // #nosec G104 - For testing

	engine := rules.NewV3Engine(cfg, logger,
		rules.EngineContextProvider(cpFunc),
		rules.EngineMetricsCollector(mFunc),
		rules.EngineSyncInterval(5),
		rules.EngineCrawlMutex("inttest", 5),
		rules.EngineLockAcquisitionTimeout(5))
	mw := &rules.MockWatcherWrapper{
		Logger:    logger,
		Responses: []v3.WatchResponse{},
	}
	m := rules.MockWatchWrapper{Mww: mw}
	engine.SetWatcherWrapper(m.WrapWatcher)
	// This is a test of the rules engine polling capability,
	// where an element is automatically added to the rule
	// that checks for a nil value of a key and that key
	// is set with a TTL after the callback has finished
	// to retrigger the callback after a delay if the provided
	// rule is still satisfied.
	preReq, err := rules.NewEqualsLiteralRule(dataPath, nil)
	check(err)
	// Ensure that a non-nil data value exists
	preReq = rules.NewNotRule(preReq)
	block, err := rules.NewEqualsLiteralRule(blockPath, nil)
	check(err)
	preReq = rules.NewAndRule(preReq, block)
	ps := map[string]*polled{}
	done := make(chan *polled)
	err = engine.AddPolling("/rulesEnginePolling/:id", preReq, 2, func(task *rules.V3RuleTask) {
		// This callback compares expected data values with actual data values, based on the number
		// of times the callback has been called for a particular ID. The data value is incremented
		// during each callback call until the target value (5) is reached at which point a blocker
		// value is set that will prevent further polling even after the polling key TTL has expired.
		task.Logger.Info("Callback called")
		// This is thread safe, because the map is only being read and not written to.
		id, _ := task.Attr.GetAttribute("id")
		p := ps[id]
		pPollCount := atomic.LoadInt32(&p.pollCount)
		// Retrieve a value from etcd.
		path := task.Attr.Format(dataPath)
		task.Logger.Info("polling", zap.String("id", p.ID), zap.String("path", path))
		resp, err := kv.Get(task.Context, path) //keysAPI.Get(task.Context, path, nil)
		check(err)
		value := string(resp.Kvs[0].Value)
		task.Logger.Info("Compare pollcount", zap.String("id", p.ID), zap.String("etcd", value), zap.Int32("local", pPollCount))
		if value != fmt.Sprint(pPollCount) {
			panic("Poll count does not match!")
		}
		// This is the base case. The expected number of polls has occurred, so
		// polling should stop.
		if pPollCount == pollCount {
			_, err = kv.Put(task.Context, task.Attr.Format(blockPath), "done")
			check(err)
			done <- p
			return
		}
		if pPollCount > pollCount {
			panic("Poll count higher than max!")
		}
		atomic.AddInt32(&p.pollCount, 1)
		_, err = kv.Put(task.Context, path, fmt.Sprint(atomic.LoadInt32(&p.pollCount)))
		check(err)
	})
	check(err)

	// Set up structs for locally mirroring etcd data with expected values.
	// Multiple IDs are used to verify that callbacks can distinguish between
	// field instances.
	for i := 0; i < idCount; i++ {
		id := fmt.Sprint(i)
		p := polled{ID: id}
		ps[id] = &p
	}

	// Set up a simple callback to verify that the callback handler is working correctly.
	doneFalse := "false"
	doneRule, err := rules.NewEqualsLiteralRule(donePath, &doneFalse)
	check(err)
	engine.AddRule(doneRule, "/rulesEngineDone/:id", func(task *rules.V3RuleTask) {
		path := task.Attr.Format(donePath)
		doneTrue := "true"
		_, err := kv.Put(task.Context, path, doneTrue)
		check(err)
	}, rules.RuleID(doneRuleID))

	engine.Run()
	time.Sleep(time.Second)
	// Write data to be polled to etcd; this will trigger the callback.
	for i := 0; i < idCount; i++ {
		id := fmt.Sprint(i)
		_, err := kv.Put(context.Background(), strings.Replace(dataPath, ":id", id, 1), "0")
		check(err)
	}

	// Wait for the polling to be done
	for i := 0; i < idCount; i++ {
		p := <-done
		logger.Info("Done", zap.String("ID", p.ID))
	}
	ctx, cancel = context.WithTimeout(context.Background(), time.Duration(30)*time.Second)
	defer cancel()
	checkWatchResp(mw.Responses)

	// Trigger the done rule
	_, err = kv.Put(context.Background(), strings.Replace(donePath, ":id", doneID, 1), doneFalse)
	check(err)

	// Verify that it ran
	tenSecCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = cbHandler.WaitForCallback(tenSecCtx, doneRuleID, map[string]string{"id": doneID})
	check(err)
	_ = engine.Shutdown(ctx) // #nosec G104 -- For testing only
}
