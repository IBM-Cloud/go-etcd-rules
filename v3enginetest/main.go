package main

import (
	"context"
	"fmt"
	"time"

	"github.com/IBM-Cloud/go-etcd-rules/rules"
	"github.com/coreos/etcd/clientv3"
	"go.uber.org/zap"
)

var (
	idCount   = 4
	pollCount = 5
)

const (
	dataPath  = "/rulesEngine/data/:id"
	blockPath = "/rulesEngine/block/:id"
)

type polled struct {
	ID        string
	pollCount int
}

func check(err error) {
	if err != nil {
		panic(err.Error())
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
	kv.Delete(context.Background(), "/rulesEngine", clientv3.WithPrefix())

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
	engine := rules.NewV3Engine(cfg, logger, rules.EngineContextProvider(cpFunc), rules.EngineMetricsCollector(mFunc))
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
		kv.Put(context.Background(), "/rulesEngine/data/"+id, "0")
		p := polled{ID: id}
		ps[id] = &p
	}
	engine.AddPolling("/rulesEnginePolling/:id", preReq, 2, func(task *rules.V3RuleTask) {
		p := ps[*task.Attr.GetAttribute("id")]
		path := task.Attr.Format(dataPath)
		task.Logger.Info("polling", zap.String("id", p.ID), zap.String("path", path))
		resp, err := kv.Get(task.Context, path) //keysAPI.Get(task.Context, path, nil)
		check(err)
		value := string(resp.Kvs[0].Value)
		task.Logger.Info("Compare pollcount", zap.String("id", p.ID), zap.String("etcd", value), zap.Int("local", p.pollCount))
		if value != fmt.Sprint(p.pollCount) {
			panic("Poll count does not match!")
		}
		if p.pollCount == pollCount {
			_, err = kv.Put(task.Context, task.Attr.Format(blockPath), "done")
			check(err)
			done <- p
			return
		}
		if p.pollCount > pollCount {
			panic("Poll count higher than max!")
		}
		p.pollCount++
		_, err = kv.Put(task.Context, path, fmt.Sprint(p.pollCount))
		check(err)
	})
	engine.Run()
	for i := 0; i < idCount; i++ {
		p := <-done
		logger.Info("Done", zap.String("ID", p.ID))
	}
	ctx, cancel = context.WithTimeout(context.Background(), time.Duration(30)*time.Second)
	defer cancel()
	engine.Shutdown(ctx)
}
