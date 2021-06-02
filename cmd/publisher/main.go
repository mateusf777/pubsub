package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mateusf777/pubsub/client"
	"github.com/mateusf777/pubsub/log"
)

func main() {
	log.SetLevel(log.INFO)

	for i := 0; i < 8; i++ {
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
	log.Info("start sending")
	for {
		count++
		msg := fmt.Sprintf("this is a longer test with count: %d", count)
		err := conn.Publish("test", []byte(msg))
		if err != nil {
			log.Error("%v", err)
			break
		}
		if count >= 1000000 {
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
