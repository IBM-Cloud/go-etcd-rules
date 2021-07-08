package main

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/IBM-Cloud/go-etcd-rules/rules"
	"go.etcd.io/etcd/clientv3"
	"go.uber.org/zap"
)

var (
	idCount   = 4
	pollCount = int32(5)
)

const (
	dataPath  = "/rulesEngine/data/:id"
	blockPath = "/rulesEngine/block/:id"
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

func checkWatchResp(resp []clientv3.WatchResponse) {
	if len(resp) != 3 {
		panic(fmt.Errorf("not the correct amount of responses returned from the watch channel"))
	}

	for _, r := range resp {
		if len(r.Events) != 1 {
			panic(fmt.Errorf("incorrect number of events for watch channel response"))
		}
	}
}

func main() {
	logger, err := zap.NewDevelopment()
	check(err)

	// cleanup etcd from previous runs
	cfg := clientv3.Config{Endpoints: []string{"http://127.0.0.1:2379"}}
	cl, err := clientv3.New(cfg)
	check(err)
	kv := clientv3.NewKV(cl)
	_, err = kv.Delete(context.Background(), "/rulesEngine", clientv3.WithPrefix())
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
	engine := rules.NewV3Engine(cfg, logger, rules.EngineContextProvider(cpFunc), rules.EngineMetricsCollector(mFunc), rules.EngineSyncInterval(10))
	mw := &rules.MockWatcherWrapper{
		Logger:    logger,
		Responses: []clientv3.WatchResponse{},
	}
	m := rules.MockWatchWrapper{Mww: mw}
	engine.SetWatcherWrapper(m.WrapWatcher)
	preReq, err := rules.NewEqualsLiteralRule(dataPath, nil)
	check(err)
	preReq = rules.NewNotRule(preReq)
	block, err := rules.NewEqualsLiteralRule(blockPath, nil)
	check(err)
	preReq = rules.NewAndRule(preReq, block)
	ps := map[string]*polled{}
	done := make(chan *polled)
	for i := 0; i < idCount; i++ {
		id := fmt.Sprint(i)
		_, err := kv.Put(context.Background(), "/rulesEngine/data/"+id, "0")
		check(err)
		p := polled{ID: id}
		ps[id] = &p
	}
	err = engine.AddPolling("/rulesEnginePolling/:id", preReq, 2, func(task *rules.V3RuleTask) {
		task.Logger.Info("Callback called")
		p := ps[*task.Attr.GetAttribute("id")]
		pPollCount := atomic.LoadInt32(&p.pollCount)
		path := task.Attr.Format(dataPath)
		task.Logger.Info("polling", zap.String("id", p.ID), zap.String("path", path))
		resp, err := kv.Get(task.Context, path) //keysAPI.Get(task.Context, path, nil)
		check(err)
		value := string(resp.Kvs[0].Value)
		task.Logger.Info("Compare pollcount", zap.String("id", p.ID), zap.String("etcd", value), zap.Int32("local", pPollCount))
		if value != fmt.Sprint(pPollCount) {
			panic("Poll count does not match!")
		}
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
	engine.Run()
	for i := 0; i < idCount; i++ {
		p := <-done
		logger.Info("Done", zap.String("ID", p.ID))
	}
	ctx, cancel = context.WithTimeout(context.Background(), time.Duration(30)*time.Second)
	defer cancel()
	checkWatchResp(mw.Responses)
	_ = engine.Shutdown(ctx)
}
