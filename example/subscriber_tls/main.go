package main

import (
	"crypto/tls"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/mateusf777/pubsub/client"

	"github.com/mateusf777/pubsub/example/common"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	clientCert := os.Getenv("PUBSUB_TLS_CLIENT_CERT")
	clientKey := os.Getenv("PUBSUB_TLS_CLIENT_KEY")

	cert, err := tls.LoadX509KeyPair(clientCert, clientKey)
	if err != nil {
		slog.Error("failed to load client cert/key", "error", err)
		return
	}

	conn, err := client.Connect(":9443", client.WithTLSConfig(&tls.Config{
		ServerName:   "simpleappz.org",
		Certificates: []tls.Certificate{cert},
	}))
	if err != nil {
		slog.Error("connection", "error", err)
		return
	}
	defer conn.Close()

	count := 0
	subID, err := conn.Subscribe("test", func(msg *client.Message) {
		if count == 0 {
			slog.Info("Subscriber started receiving messages")
		}
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
	defer func() {
		slog.Info("received", "count", count)
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
	slog.Debug("Closing")
}
