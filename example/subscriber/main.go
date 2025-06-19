package main

import (
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/mateusf777/pubsub/client"

	"github.com/mateusf777/pubsub/example/common"
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

	count := 0
	subID, err := conn.Subscribe("test", func(msg *client.Message) {
		count++
		if count >= (common.Routines * common.Messages) {
			slog.Info("received", "count", count)
		}
	})
	if err != nil {
		slog.Error("Subscribe", "error", err)
		return
	}
	defer conn.Unsubscribe("test", subID)

	subID2, err := conn.Subscribe("count", func(msg *client.Message) {
		resp := strconv.Itoa(count)
		if err := conn.Reply(msg, []byte(resp)); err != nil {
			slog.Error("Subscribe", "error", err)
		}
		slog.Debug("should have returned count")
	})
	if err != nil {
		slog.Error("Subscribe", "error", err)
		return
	}
	defer conn.Unsubscribe("count", subID2)

	subID3, err := conn.Subscribe("time", func(msg *client.Message) {
		slog.Debug("getting time", "message", msg)
		resp := time.Now().String()
		if err := conn.Reply(msg, []byte(resp)); err != nil {
			slog.Error("Subscribe", "error", err)
		}
		slog.Debug("should have returned time")
	})
	if err != nil {
		slog.Error("Subscribe", "error", err)
		return
	}
	defer conn.Unsubscribe("time", subID3)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
	slog.Debug("Closing")
}
