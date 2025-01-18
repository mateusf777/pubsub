package main

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/mateusf777/pubsub/client"
	"github.com/mateusf777/pubsub/example/common"
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
	conn, err := client.Connect(":9999")
	if err != nil {
		slog.Error("send", "error", err)
		return
	}
	defer wg.Done()
	defer func() {
		conn.Drain()
		slog.Info("Connection closed")
	}()

	slog.Info("start sending")
	for i := 0; i < common.Messages; i++ {
		msg := fmt.Sprintf("this is a longer test with count: %d", i)
		err := conn.Publish("test", []byte(msg))
		if err != nil {
			slog.Error("send Publish", "error", err)
			return
		}
	}
	slog.Info("finish sending")
}
