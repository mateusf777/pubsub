package server

import (
	"net"
	"strings"

	psnet "github.com/mateusf777/pubsub/net"

	"github.com/mateusf777/pubsub/domain"
	"github.com/mateusf777/pubsub/log"
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
	psnet.Wait()
	log.Info("Stopping PubSub")
}

func acceptClients(l net.Listener) {
	ps := domain.NewPubSub()
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
