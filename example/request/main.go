package main

import (
	"log/slog"

	"github.com/mateusf777/pubsub/client"
)

func main() {

	conn, err := client.Connect(":9999")
	if err != nil {
		slog.Error("client.Connect", "error", err)
		return
	}
	defer conn.Close()

	slog.Info("request time")
	resp, err := conn.Request("time", nil)
	if err != nil {
		slog.Error("Request", "error", err)
		return
	}

	slog.Info("now", "data", resp.Data)
}
