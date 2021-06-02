package main

import (
	"github.com/mateusf777/pubsub/client"
	"github.com/mateusf777/pubsub/log"
)

func main() {
	log.SetLevel(log.INFO)

	conn, err := client.Connect(":9999")
	if err != nil {
		log.Error("%v", err)
		return
	}
	defer conn.Close()

	log.Info("start sending")
	resp, err := conn.Request("time", nil)
	if err != nil {
		log.Error("%v", err)
		return
	}

	log.Info("%v", resp.Value)
	log.Info("finish sending")
}
