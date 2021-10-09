package main

import (
	"context"
	"fmt"
	"time"

	"github.com/IBM-Cloud/go-etcd-rules/rules/concurrency"
	"go.etcd.io/etcd/clientv3"
)

func check(err error) {
	if err != nil {
		panic(err.Error())
	}
}

func main() {
	cfg := clientv3.Config{Endpoints: []string{"http://127.0.0.1:2379"}}
	cl, err := clientv3.New(cfg)
	check(err)
	session, err := concurrency.NewSession(cl)
	check(err)
	mutex := concurrency.NewMutex(session, "/locks/hello")
	err = mutex.TryLock(context.Background())
	check(err)
	fmt.Println(mutex.Key())
	time.Sleep(time.Minute)
	mutex.Unlock(context.Background())
	fmt.Println("Unlocked")
	time.Sleep(time.Minute)
	session.Close()
	fmt.Println("Session closed")
	for {
		time.Sleep(time.Second)
	}
}
