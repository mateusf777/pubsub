package main

import (
	"github.com/mateusf777/pubsub/client"
	logger "github.com/mateusf777/pubsub/log"
)

func main() {
	log := logger.New()
	log.Level = logger.INFO

	conn, err := client.Connect(":9999")
	if err != nil {
		log.Error("%v", err)
		return
	}
	defer conn.Close()

	log.Info("request time")
	resp, err := conn.Request("time", nil)
	if err != nil {
		log.Error("%v", err)
		return
	}

	log.Info("now: %s", resp.Data)
}
