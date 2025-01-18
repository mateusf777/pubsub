package main

import (
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/mateusf777/pubsub/client"
)

const (
	queue    = 3
	messages = 15
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	slog.SetLogLoggerLevel(slog.LevelInfo)

	conn, err := client.Connect(":9999")
	if err != nil {
		slog.Error("client.Connect", "error", err)
		return
	}
	defer conn.Close()

	slog.Info("Launching subscribers", "queue", queue)

	mu := sync.Mutex{}
	count := 0
	for i := 0; i < queue; i++ {
		n := i
		err = conn.QueueSubscribe("echo", "queue", func(msg *client.Message) {
			mu.Lock()
			count++
			mu.Unlock()
			slog.Info("Queue, received", "queue", n, "msg.Data", msg.Data)
		})
		if err != nil {
			slog.Error("Queue", "queue", n, "error", err)
			return
		}
		slog.Info("Queue, successfully subscribed", "queue", n)
	}

	slog.Info("Publishing messages...", "value", messages*queue)
	for i := 0; i < messages*queue; i++ {
		err = conn.Publish("echo", []byte("message to echo"))
		if err != nil {
			slog.Error("could not publish", "error", err)
			continue
		}
	}
	slog.Info("Finished publishing messages")

	// Wait for all messages
	ticker := time.NewTicker(100 * time.Millisecond)
	timeout := time.NewTimer(10 * time.Minute)
Wait:
	for {
		select {
		case <-ticker.C:
			if count >= messages*queue {
				slog.Info("Received all messages")
				break Wait
			}
		case <-timeout.C:
			slog.Info("Timeout! only received messages", "count", count)
			return
		}
	}
}
