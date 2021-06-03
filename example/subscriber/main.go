package main

import (
	"strconv"
	"time"

	psnet "github.com/mateusf777/pubsub/net"

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

	count := 0
	err = conn.Subscribe("test", func(msg *client.Message) {
		count++
		if count >= 8000000 {
			log.Info("received %d", count)
		}
	})
	if err != nil {
		log.Error("%v", err)
		return
	}

	err = conn.Subscribe("count", func(msg *client.Message) {
		resp := strconv.Itoa(count)
		err = msg.Respond([]byte(resp))
		if err != nil {
			log.Error("%v", err)
		}
		log.Debug("should have returned count")
	})
	if err != nil {
		log.Error("%v", err)
		return
	}

	err = conn.Subscribe("time", func(msg *client.Message) {
		log.Debug("getting time")
		resp := time.Now().String()
		err = msg.Respond([]byte(resp))
		if err != nil {
			log.Error("%v", err)
		}
		log.Debug("should have returned time")
	})
	if err != nil {
		log.Error("%v", err)
		return
	}

	psnet.Wait()
	log.Debug("Closing")
}
