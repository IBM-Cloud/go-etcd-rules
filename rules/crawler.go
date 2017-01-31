package rules

import (
	"time"

	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/clientv3"
	"github.com/uber-go/zap"
	"golang.org/x/net/context"
)

type crawler interface {
	run()
	stop()
	isStopped() bool
}

func newCrawler(
	config client.Config,
	logger zap.Logger,
	prefix string,
	interval int,
	kp keyProc,
) (crawler, error) {
	blank := etcdCrawler{}
	cl, err1 := client.New(config)
	if err1 != nil {
		return &blank, err1
	}
	kapi := client.NewKeysAPI(cl)
	api := etcdReadAPI{
		keysAPI: kapi,
	}
	c := etcdCrawler{
		baseCrawler: baseCrawler{
			api:      &api,
			interval: interval,
			kp:       kp,
			logger:   logger,
			prefix:   prefix,
		},
		kapi: kapi,
	}
	return &c, nil
}

func newV3Crawler(
	config clientv3.Config,
	logger zap.Logger,
	prefix string,
	interval int,
	kp keyProc,
) (crawler, error) {
	blank := etcdCrawler{}
	cl, err1 := clientv3.New(config)
	if err1 != nil {
		return &blank, err1
	}
	kv := clientv3.NewKV(cl)
	api := etcdV3ReadAPI{
		kV: kv,
	}
	c := v3EtcdCrawler{
		baseCrawler: baseCrawler{
			api:      &api,
			interval: interval,
			kp:       kp,
			logger:   logger,
			prefix:   prefix,
		},
		kv: kv,
	}
	return &c, nil
}

type baseCrawler struct {
	api        readAPI
	interval   int
	kp         keyProc
	logger     zap.Logger
	prefix     string
	stopping   bool
	stopped    bool
	cancelFunc context.CancelFunc
}

func (bc *baseCrawler) stop() {
	bc.stopping = true
	if bc.cancelFunc != nil {
		bc.cancelFunc()
	}
}

func (bc *baseCrawler) isStopped() bool {
	return bc.stopped
}

type etcdCrawler struct {
	baseCrawler
	kapi client.KeysAPI
}

func (ec *etcdCrawler) run() {
	ec.stopped = false
	for !ec.stopping {
		ec.logger.Debug("Starting crawler run")
		ec.singleRun()
		ec.logger.Debug("Crawler run complete")
		for i:=0; i< ec.interval; i++ {
			time.Sleep(time.Second)
			if ec.stopping {
				break
			}
		}
	}
	ec.stopped = true
}

func (ec *etcdCrawler) singleRun() {
	ec.crawlPath(ec.prefix)
}

func (ec *etcdCrawler) crawlPath(path string) {
	if ec.stopping {
		return
	}
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

type v3EtcdCrawler struct {
	baseCrawler
	kv clientv3.KV
}

func (v3ec *v3EtcdCrawler) run() {
	v3ec.stopped = false
	for !v3ec.stopping {
		v3ec.logger.Debug("Starting crawler run")
		v3ec.singleRun()
		v3ec.logger.Debug("Crawler run complete")
		for i:=0; i< v3ec.interval; i++ {
			time.Sleep(time.Second)
			if v3ec.stopping {
				break
			}
		}
	}
	v3ec.stopped = true
}

func (v3ec *v3EtcdCrawler) singleRun() {
	if v3ec.stopping {
		return
	}
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Duration(1)*time.Minute)
	v3ec.cancelFunc = cancelFunc
	defer cancelFunc()
	resp, err := v3ec.kv.Get(ctx, v3ec.prefix, clientv3.WithPrefix())
	if err != nil {
		return
	}
	logger := v3ec.logger.With(zap.String("source", "crawler"))
	for _, kv := range resp.Kvs {
		if v3ec.stopping {
			return
		}
		value := string(kv.Value[:])
		v3ec.kp.processKey(string(kv.Key[:]), &value, v3ec.api, logger)
	}
}
