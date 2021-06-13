package main

import (
	"fmt"
	"sync"

	"github.com/mateusf777/pubsub/client"
	"github.com/mateusf777/pubsub/log"
)

const (
	routines = 8
	messages = 1000000
)

func main() {
	log.SetLevel(log.INFO)

	wg := &sync.WaitGroup{}
	for i := 0; i < routines; i++ {
		wg.Add(1)
		go send(wg)
	}

	wg.Wait()
}

func send(wg *sync.WaitGroup) {
	conn, err := client.Connect(":9999")
	if err != nil {
		log.Error("%v", err)
		return
	}
	defer wg.Done()
	defer conn.Drain()

	log.Info("start sending")
	for i := 0; i < messages; i++ {
		msg := fmt.Sprintf("this is a longer test with count: %d", i)
		err := conn.Publish("test", []byte(msg))
		if err != nil {
			log.Error("%v", err)
			return
		}
	}
	log.Info("finish sending")
}
