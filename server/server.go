package server

import (
	"net"
	"strings"

	psnet "github.com/mateusf777/pubsub/net"

	"github.com/mateusf777/pubsub/domain"
	"github.com/mateusf777/pubsub/log"
)

// Run stars to listen in the given address
// Ex: server.Run("localhost:9999")
func Run(address string) {
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

// Starts a concurrent handler for each connection
func acceptClients(l net.Listener) {
	ps := domain.NewPubSub()
	defer ps.Stop()

	for {
		c, err := l.Accept()
		if err != nil {
			// Todo: better handle connection to stop hiding errors like this
			if strings.Contains(err.Error(), psnet.CloseErr) {
				return
			}
			log.Error("%v\n", err)
			return
		}

		go handleConnection(c, ps)
	}
}
