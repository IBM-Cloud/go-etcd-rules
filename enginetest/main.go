package main

import (
	"context"
	"fmt"
	"time"

	"github.com/IBM-Bluemix/go-etcd-rules/rules"
	"github.com/coreos/etcd/client"
	"github.com/uber-go/zap"
)

var (
	idCount   = 4
	pollCount = 5
)

const (
	dataPath  = "/data/:id"
	blockPath = "/block/:id"
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
	logger := zap.New(
		zap.NewJSONEncoder(zap.RFC3339Formatter("ts")),
		zap.AddCaller(),
		zap.DebugLevel,
	)
	cfg := client.Config{Endpoints: []string{"http://127.0.0.1:2379"}}
	cl, err := client.New(cfg)
	check(err)
	keysAPI := client.NewKeysAPI(cl)
	resp, err := keysAPI.Get(context.Background(), "/", nil)
	check(err)
	for _, node := range resp.Node.Nodes {
		_, err := keysAPI.Delete(context.Background(), node.Key, &client.DeleteOptions{Recursive: true, Dir: node.Dir})
		check(err)
	}
	engine := rules.NewEngine(cfg, logger, rules.EngineAutoCrawlGuides(true))
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
		keysAPI.Set(context.Background(), "/data/"+id, "0", nil)
		p := polled{ID: id}
		ps[id] = &p
	}
	engine.AddPolling("/polling/:id", preReq, 2, func(task *rules.RuleTask) {
		p := ps[*task.Attr.GetAttribute("id")]
		path := task.Attr.Format(dataPath)
		task.Logger.Info("polling", zap.String("id", p.ID), zap.String("path", path))
		resp, err := keysAPI.Get(task.Context, path, nil)
		check(err)
		value := resp.Node.Value
		task.Logger.Info("Compare pollcount", zap.String("id", p.ID), zap.String("etcd", value), zap.Int("local", p.pollCount))
		if value != fmt.Sprint(p.pollCount) {
			panic("Poll count does not match!")
		}
		if p.pollCount == pollCount {
			_, err := keysAPI.Set(task.Context, task.Attr.Format(blockPath), "done", nil)
			check(err)
			done <- p
			return
		}
		if p.pollCount > pollCount {
			panic("Poll count higher than max!")
		}
		p.pollCount++
		_, err = keysAPI.Set(task.Context, path, fmt.Sprint(p.pollCount), nil)
		check(err)
	})
	engine.Run()
	for i := 0; i < idCount; i++ {
		p := <-done
		logger.Info("Done", zap.String("ID", p.ID))
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(30)*time.Second)
	engine.Shutdown(ctx)
}
