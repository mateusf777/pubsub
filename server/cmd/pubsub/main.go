package main

import (
	"log/slog"
	"os"

	"github.com/mateusf777/pubsub/core"
	"github.com/mateusf777/pubsub/server"
)

// ATTENTION: This server has NO SECURITY whatsoever
// If you expose it you're on your own.
const defaultAddress = "127.0.0.1:9999"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	slog.SetLogLoggerLevel(slog.LevelInfo)

	address := os.Getenv("PUBSUB_ADDRESS")
	if address == string(core.Empty) {
		address = defaultAddress
	}

	s := &server.Server{}
	s.Run(address)
}
