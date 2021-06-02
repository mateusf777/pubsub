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

	count := 0
	log.Info("start sending")
	for {
		count++
		msg := fmt.Sprintf("this is a longer test with count: %d", count)
		err := conn.Publish("test", []byte(msg))
		if err != nil {
			log.Error("%v", err)
			break
		}
		if count >= messages {
			break
		}
	}
	log.Info("finish sending")
}
