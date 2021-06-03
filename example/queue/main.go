package main

import (
	"sync"
	"time"

	"github.com/mateusf777/pubsub/client"
	"github.com/mateusf777/pubsub/log"
)

const (
	queue    = 3
	messages = 15
)

func main() {
	log.SetLevel(log.INFO)

	conn, err := client.Connect(":9999")
	if err != nil {
		log.Error("%v", err)
		return
	}
	defer conn.Close()

	log.Info("Launching %d queue subscribers", queue)

	mu := sync.Mutex{}
	count := 0
	for i := 0; i < queue; i++ {
		n := i
		err = conn.QueueSubscribe("echo", "queue", func(msg *client.Message) {
			mu.Lock()
			count++
			mu.Unlock()
			log.Info("Queue %d, received: %s", n, msg.Data)
		})
		if err != nil {
			log.Error("Queue %d, error: %v", n, err)
			return
		}
		log.Info("Queue %d, successfully subscribed", n)
	}

	log.Info("Publishing %d messages...", messages*queue)
	for i := 0; i < messages*queue; i++ {
		err = conn.Publish("echo", []byte("message to echo"))
		if err != nil {
			log.Error("could not publish, %v", err)
			continue
		}
	}
	log.Info("Finished publishing messages")

	// Wait for all messages
	ticker := time.NewTicker(100 * time.Millisecond)
	timeout := time.NewTimer(10 * time.Minute)
Wait:
	for {
		select {
		case <-ticker.C:
			if count >= messages*queue {
				log.Info("Received all messages")
				break Wait
			}
		case <-timeout.C:
			log.Info("Timeout! only received %d messages", count)
			return
		}
	}
}
