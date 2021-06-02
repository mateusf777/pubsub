package server

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	psnet "github.com/mateusf777/pubsub/net"

	"github.com/mateusf777/pubsub/log"
	"github.com/mateusf777/pubsub/pubsub"
)

func Start(address string) {
	l, err := net.Listen("tcp4", address)
	if err != nil {
		log.Error("%v\n", err)
		return
	}
	defer l.Close()

	go acceptClients(l)

	log.Info("PubSub accepting connections at %s", address)
	wait()
}

func wait() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
	fmt.Println()
	log.Info("PubSub stop")
}

func acceptClients(l net.Listener) {
	ps := pubsub.NewPubSub()
	defer ps.Stop()

	for {
		c, err := l.Accept()
		if err != nil {
			if strings.Contains(err.Error(), psnet.CloseErr) {
				return
			}
			log.Error("%v\n", err)
			return
		}

		go handleConnection(c, ps)
	}
}
