package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/mateusf777/pubsub/client"
	"github.com/mateusf777/pubsub/log"
	"github.com/mateusf777/pubsub/pubsub"
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
	err = conn.Subscribe("test", func(msg pubsub.Message) {
		count++
		if count >= 8000000 {
			log.Info("received %d", count)
		}
	})
	if err != nil {
		log.Error("%v", err)
		return
	}

	err = conn.Subscribe("count", func(msg pubsub.Message) {
		resp := strconv.Itoa(count)
		err = conn.Publish(msg.Reply, []byte(resp))
		if err != nil {
			log.Error("%v", err)
		}
		log.Debug("should have returned count")
	})
	if err != nil {
		log.Error("%v", err)
		return
	}

	err = conn.Subscribe("time", func(msg pubsub.Message) {
		log.Debug("getting time")
		resp := time.Now().String()
		err = conn.Publish(msg.Reply, []byte(resp))
		if err != nil {
			log.Error("%v", err)
		}
		log.Debug("should have returned time")
	})
	if err != nil {
		log.Error("%v", err)
		return
	}

	wait()
	log.Debug("Closing")
	return
}

func wait() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
	fmt.Println()
}
