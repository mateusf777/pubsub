package main

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/mateusf777/pubsub/example/common"

	"github.com/mateusf777/pubsub/domain"

	"github.com/mateusf777/pubsub/client"
)

func main() {

	conn, err := client.Connect(":9999")
	if err != nil {
		slog.Error("client.Connect", "error", err)
		return
	}
	defer conn.Close()

	count := 0
	err = conn.Subscribe("test", func(msg *client.Message) {
		count++
		if count >= (common.Routines * common.Messages) {
			slog.Info("received", "count", count)
		}
	})
	if err != nil {
		slog.Error("Subscribe", "error", err)
		return
	}

	err = conn.Subscribe("count", func(msg *client.Message) {
		resp := strconv.Itoa(count)
		err = msg.Respond([]byte(resp))
		if err != nil {
			slog.Error("Subscribe", "error", err)
		}
		slog.Debug("should have returned count")
	})
	if err != nil {
		slog.Error("Subscribe", "error", err)
		return
	}

	err = conn.Subscribe("time", func(msg *client.Message) {
		slog.Debug("getting time")
		resp := time.Now().String()
		err = msg.Respond([]byte(resp))
		if err != nil {
			slog.Error("Subscribe", "error", err)
		}
		slog.Debug("should have returned time")
	})
	if err != nil {
		slog.Error("Subscribe", "error", err)
		return
	}

	domain.Wait()
	slog.Debug("Closing")
}
