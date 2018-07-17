etcd-rules
==========

[![Build Status](https://travis-ci.org/IBM-Cloud/go-etcd-rules.svg?branch=master)](https://travis-ci.org/IBM-Cloud/go-etcd-rules)
[![Coverage Status](https://coveralls.io/repos/github/IBM-Cloud/go-etcd-rules/badge.svg?branch=master)](https://coveralls.io/github/IBM-Cloud/go-etcd-rules?branch=master)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

This is a rules engine for use with etcd.  Simple dynamic rules allow the specification
of keys based on the gin attribute syntax and the comparison to literals or other
keys.  These rules can be nested inside of AND, OR and NOT rules to enable the expression
of complex relationships of values in etcd and the actions to be triggered when a set
of conditions has been met.  The engine watches etcd for updates and crawls the data tree
at configurable intervals so that changes that occurred beyond the watch time scope are picked
up and actions triggered by watches that initially failed can be retried without being lost.
This library makes use of the IBM-Cloud/go-etcd-lock library to enable concurrent monitoring
by multiple application instances without collisions--the first client to obtain the lock
processes the change while the others quickly fail to acquire the lock and move on.  A trigger
callback function should update the model if the action is successful so it is not retriggered.
Recurring actions, such as continuous polling, can be implemented with rules that reference
nodes with TTLs such that the expiration of a node triggers a rule and the callback adds back
a node with the same key and TTL.

Import
------

```
# Master via standard import
go get github.com/IBM-Cloud/go-etcd-rules/rules
```

Example
-------

```go
package main

import (
	"time"

	"github.com/IBM-Cloud/go-etcd-rules/rules"
	"github.com/coreos/etcd/clientv3"
	"github.com/uber-go/zap"
	"golang.org/x/net/context"
)

func lTP(val string) *string {
	s := val
	return &s
}

func main() {
	logger := zap.New(
		zap.NewJSONEncoder(zap.RFC3339Formatter("ts")),
		zap.AddCaller(),
	)

	cfg := clientv3.Config{
		Endpoints: []string{"http://127.0.0.1:2379"},
	}

	engine := rules.NewV3Engine(cfg, logger)

	serverActive, _ := rules.NewEqualsLiteralRule("/servers/:serverid/state", lTP("active"))
	pollDelayGone, _ := rules.NewEqualsLiteralRule("/servers/internal/:serverid/poll_delay", nil)

	engine.AddRule(
		rules.NewAndRule(serverActive, pollDelayGone),
		"/:serverid/poll",
		pollServer,
	)

	engine.Run()

	end := make(chan bool)
	<-end
}

func pollServer(task *rules.V3RuleTask) {
	cl, err := clientv3.New(task.Conf)
	if err != nil {
		return
	}
	kv := clientv3.NewKV(cl)
	resp, err := kv.Get(task.Context, task.Attr.Format("/servers/:serverid/ip"))
	if err != nil {
		return
	}
	if resp.Count == 0 {
		return
	}
	ip := string(resp.Kvs[0].Value)
	var status string
	if ping(ip.Node.Value) {
		status = "ok"
	} else {
		status = "down"
	}
	kv.Put(task.Context, task.Attr.Format("/servers/:serverid/status"), status)
	// Add new poll delay
	leaseCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resp, err = cl.Grant(leaseCtx, int64(5))
	if err != nil {
		return
	}
	kv.Put(task.Context, task.Attr.Format("/servers/internal/:serverid/poll_delay"), "", clientv3.WithLease(resp.ID))
}
```
