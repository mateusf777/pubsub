package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mateusf777/pubsub/client"
	"github.com/mateusf777/pubsub/log"
)

func main() {

	for i := 0; i < 2; i++ {
		go send()
	}

	wait()
}

func send() {
	conn, err := client.Connect(":9999")
	if err != nil {
		log.Error("%v", err)
		return
	}
	defer conn.Close()

	count := 0
	ticker := time.NewTicker(time.Microsecond)
	log.Info("start sending")
	for range ticker.C {
		count++
		msg := fmt.Sprintf("this is a longer test with count: %d", count)
		err := conn.Publish("test", []byte(msg))
		if err != nil {
			log.Error("%v", err)
			break
		}
		if count >= 500000 {
			break
		}
	}
	log.Info("finish sending")
}

func wait() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
	fmt.Println()
}
