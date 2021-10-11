package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/IBM-Cloud/go-etcd-rules/rules/concurrency"
	"go.etcd.io/etcd/clientv3"
)

func check(err error) {
	if err != nil {
		panic(err.Error())
	}
}

var session *concurrency.Session
var sessionMutex sync.Mutex
var sessionDone <-chan struct{}

func manageSession(client *clientv3.Client) {
	initSession(client)
}

func initSession(client *clientv3.Client) {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()
	var err error
	session, err = concurrency.NewSession(client)
	check(err)
	fmt.Printf("Session lease ID: %x\n", session.Lease())
	sessionDone = session.Done()
	go func() {
		<-sessionDone
		initSession(client)
	}()
}

func main() {
	cfg := clientv3.Config{Endpoints: []string{"http://127.0.0.1:2379"}}
	cl, err := clientv3.New(cfg)
	check(err)
	// session, err = concurrency.NewSession(cl)
	// check(err)
	manageSession(cl)
	// mutex := concurrency.NewMutex(session, "/locks/hello")
	// err = mutex.TryLock(context.Background())
	// check(err)
	// fmt.Println(mutex.Key())
	// time.Sleep(time.Minute)
	// mutex.Unlock(context.Background())
	// fmt.Println("Unlocked")
	// time.Sleep(time.Minute)
	// session.Close()
	// fmt.Println("Session closed")
	// d := session.Done()
	// go func() {
	// 	<-d
	// 	fmt.Println("done")
	// }()
	for {
		sessionMutex.Lock()
		mutex := concurrency.NewMutex(session, "/locks/hello")
		err = mutex.TryLock(context.Background())
		sessionMutex.Unlock()
		if err != nil {
			fmt.Println(err)
			time.Sleep(time.Second * 10)
			continue
			// break
		}
		fmt.Println(mutex.Key())
		time.Sleep(time.Second * 3)
		err = mutex.Unlock(context.Background())
		if err == nil {
			fmt.Println("Unlocked")
		} else {
			fmt.Println(err)
			// break
		}
		time.Sleep(time.Second * 3)
	}
}
