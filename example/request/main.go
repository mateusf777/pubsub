package main

import (
	"log/slog"
	"os"

	"github.com/mateusf777/pubsub/client"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	conn, err := client.Connect(":9999")
	if err != nil {
		slog.Error("client.Connect", "error", err)
		return
	}
	defer conn.Close()

	c := conn.GetClient()

	slog.Info("request time")
	resp, err := c.Request("time", nil)
	if err != nil {
		slog.Error("Request", "error", err)
		return
	}

	slog.Info("now", "data", string(resp.Data))
}
