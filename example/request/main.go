package main

import (
	"github.com/mateusf777/pubsub/client"
	"log/slog"
	"os"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	//client.SetLogLevel(slog.LevelDebug)
	//core.SetLogLevel(slog.LevelDebug)

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

	slog.Info("now", "data", string(resp.Data))
}
