package main

import (
	"fmt"
	"sync"

	"github.com/mateusf777/pubsub/example/common"

	"github.com/mateusf777/pubsub/client"
	logger "github.com/mateusf777/pubsub/log"
)

func main() {
	wg := &sync.WaitGroup{}
	for i := 0; i < common.Routines; i++ {
		wg.Add(1)
		go send(wg)
	}

	wg.Wait()
}

func send(wg *sync.WaitGroup) {
	log := logger.New()
	log.Level = logger.INFO

	conn, err := client.Connect(":9999")
	if err != nil {
		log.Error("%v", err)
		return
	}
	defer wg.Done()
	defer func() {
		conn.Drain()
		log.Info("Connection closed")
	}()

	log.Info("start sending")
	for i := 0; i < common.Messages; i++ {
		msg := fmt.Sprintf("this is a longer test with count: %d", i)
		err := conn.Publish("test", []byte(msg))
		if err != nil {
			log.Error("%v", err)
			return
		}
	}
	log.Info("finish sending")
}
