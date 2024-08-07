etcd-rules
==========

[![Build Status](https://github.com/IBM-Cloud/go-etcd-rules/actions/workflows/go.yml/badge.svg?branch=master)](https://github.com/IBM-Cloud/go-etcd-rules/actions?branch=master)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

**NOTE: Built with the etcd 3.5.x client.**

This is a rules engine for use with etcd.  Simple dynamic rules allow the specification
of keys based on the gin attribute syntax and the comparison to literals or other
keys.  These rules can be nested inside of AND, OR and NOT rules to enable the expression
of complex relationships of values in etcd and the actions to be triggered when a set
of conditions has been met.  The engine watches etcd for updates and crawls the data tree
at configurable intervals. This library makes use of the IBM-Cloud/go-etcd-lock library
to enable concurrent monitoring by multiple application instances without collisions--the
first client to obtain the lock processes the change while the others quickly fail to acquire
the lock and move on.  A trigger callback function should update the model if the action
is successful so it is not retriggered. Recurring actions, such as continuous polling,
can be implemented with rules that reference nodes with TTLs such that the expiration of
a node triggers a rule and the callback adds back a node with the same key and TTL.

Import
------

```shell
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
    v3 "go.etcd.io/etcd/client/v3"
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

    cfg := v3.Config{
        Endpoints: []string{"http://127.0.0.1:2379"},
    }

    engine := rules.NewV3Engine(cfg, logger)

    serverActive, err := rules.NewEqualsLiteralRule("/servers/:serverid/state", lTP("active"))
    if err != nil {
        panic(err)
    }
    pollDelayGone, err := rules.NewEqualsLiteralRule("/servers/internal/:serverid/poll_delay", nil)
    if err != nil {
        panic(err)
    }

    engine.AddRule(
        rules.NewAndRule(serverActive, pollDelayGone),
        "/:serverid/poll",
        pollServer,
        RuleID("example")
    )

    engine.Run()

    end := make(chan bool)
    <-end
}

func pollServer(task *rules.V3RuleTask) {
    cl, err := v3.New(task.Conf)
    if err != nil {
        return
    }
    kv := v3.NewKV(cl)
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
    kv.Put(task.Context, task.Attr.Format("/servers/internal/:serverid/poll_delay"), "", v3.WithLease(resp.ID))
}
```
