package rules

import (
	"time"

	"github.com/coreos/etcd/client"
	"github.com/uber-go/zap"
	"golang.org/x/net/context"
)

type crawler interface {
	run()
}

func newCrawler(
	config client.Config,
	logger zap.Logger,
	prefix string,
	interval int,
	kp kProcessor,
) (crawler, error) {
	blank := etcdCrawler{}
	cl, err1 := client.New(config)
	if err1 != nil {
		return &blank, err1
	}
	kapi := client.NewKeysAPI(cl)
	api := etcdReadAPI{
		kAPI: kapi,
	}
	c := etcdCrawler{
		api:      &api,
		interval: interval,
		kapi:     kapi,
		kp:       kp,
		logger:   logger,
		prefix:   prefix,
	}
	return &c, nil
}

//type clientKeysAPI interface {
//	Get(ctx context.Context, key string, opts *client.GetOptions) (*client.Response, error)
//}

type etcdCrawler struct {
	api      readAPI
	interval int
	kapi     client.KeysAPI
	kp       kProcessor
	logger   zap.Logger
	prefix   string
}

func (ec *etcdCrawler) run() {
	for {
		ec.logger.Info("Starting crawler run")
		ec.singleRun()
		ec.logger.Info("Crawler run complete")
		time.Sleep(time.Duration(ec.interval) * time.Second)
	}
}

func (ec *etcdCrawler) singleRun() {
	ec.crawlPath(ec.prefix)
}

func (ec *etcdCrawler) crawlPath(path string) {
	resp, err := ec.kapi.Get(context.Background(), path, nil)
	if err != nil {
		return
	}
	if resp.Node.Dir {
		for _, node := range resp.Node.Nodes {
			ec.crawlPath(node.Key)
		}
		return
	}
	node := resp.Node
	logger := ec.logger.With(zap.String("source", "crawler"))
	ec.kp.processKey(node.Key, &node.Value, ec.api, logger)
}
