package rules

import (
	"github.com/coreos/etcd/clientv3"
	"go.uber.org/zap"
)

// monitors data stores
type storeMonitor interface {
	monitorStore(ruleMgr ruleManager)
	stoppable
}

// monitors data stores using two types of monitors: a crawler and watchers
type storeDualMonitor struct {
	watchers  []watcher
	crawler   crawler
	client    *clientv3.Client
	options   engineOptions
	keyProc   setableKeyProcessor
	kvWrapper WrapKV
	stopped   uint32
	logger    *zap.Logger
}

func newStoreDualMonitor(cl *clientv3.Client, options engineOptions,
	keyProc setableKeyProcessor, logger *zap.Logger, kvWrapper WrapKV) storeMonitor {
	return &storeDualMonitor{
		watchers:  []watcher{},
		logger:    logger,
		client:    cl,
		options:   options,
		keyProc:   keyProc,
		kvWrapper: kvWrapper,
	}
}

// monitors a data store using both watchers and a crawler for all prefixes stored
// in the rule manager
func (d *storeDualMonitor) monitorStore(ruleMgr ruleManager) {
	prefixes := ruleMgr.getPrefixes()
	var watcherList []watcher
	var prefixSlice []string
	d.logger.Info("starting watchers")
	for prefix := range prefixes {
		prefixSlice = append(prefixSlice, prefix)
		logger := d.logger.With(zap.String("prefix", prefix))
		w, err := newV3Watcher(d.client, prefix, logger, d.keyProc, d.options.watchTimeout, d.kvWrapper)
		if err != nil {
			logger.Fatal("Failed to initialize watcher", zap.String("prefix", prefix))
		}
		watcherList = append(watcherList, w)
		go w.run()
	}

	cLogger := d.logger
	var err error
	d.crawler, err = newIntCrawler(d.client,
		d.options.syncInterval,
		d.keyProc,
		cLogger,
		d.options.crawlMutex,
		d.options.crawlerTTL,
		prefixSlice,
		d.kvWrapper,
		d.options.syncDelay)
	if err != nil {
		d.logger.Fatal("Failed to initialize crawler", zap.Error(err))
	}
	d.logger.Info("starting crawler")
	go d.crawler.run()
}

func (d *storeDualMonitor) stop() {
	d.logger.Debug("Stopping crawler")

	stopstoppables([]stoppable{d.crawler})

	d.logger.Debug("Stopping watchers")
	var s []stoppable
	for _, w := range d.watchers {
		s = append(s, &w)
	}
	stopstoppables(s)
	atomicSet(&d.stopped, true)
	d.logger.Info("store monitor stopped")

}

func (d *storeDualMonitor) isStopped() bool {
	return is(&d.stopped)
}
