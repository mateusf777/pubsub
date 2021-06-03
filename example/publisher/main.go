package main

import (
	"fmt"

	psnet "github.com/mateusf777/pubsub/net"

	"github.com/mateusf777/pubsub/client"
	"github.com/mateusf777/pubsub/log"
)

const (
	routines = 8
	messages = 1000000
)

func main() {
	log.SetLevel(log.INFO)

	for i := 0; i < routines; i++ {
		go send()
	}

	psnet.Wait()
}

func send() {
	conn, err := client.Connect(":9999")
	if err != nil {
		log.Error("%v", err)
		return
	}
	defer conn.Close()

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
