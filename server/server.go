package server

import (
	"net"
	"strings"

	psnet "github.com/mateusf777/pubsub/net"

	"github.com/mateusf777/pubsub/domain"
	"github.com/mateusf777/pubsub/log"
)

type Server struct {
	log log.Logger
}

func New() Server {
	return Server{
		log: log.New().WithContext("pubsub server"),
	}
}

func (s *Server) SetLogLevel(level log.Level) {
	s.log.Level = level
}

// Run stars to listen in the given address
// Ex: server.Run("localhost:9999")
func (s Server) Run(address string) {
	l, err := net.Listen("tcp4", address)
	if err != nil {
		s.log.Error("%v\n", err)
		return
	}
	defer l.Close()

	go s.acceptClients(l)

	s.log.Info("PubSub accepting connections at %s", address)
	psnet.Wait()
	s.log.Info("Stopping PubSub")
}

// Starts a concurrent handler for each connection
func (s Server) acceptClients(l net.Listener) {
	ps := domain.NewPubSub()
	defer ps.Stop()
	ps.SetLogLevel(s.log.Level)

	for {
		c, err := l.Accept()
		if err != nil {
			// Todo: better handle connection to stop hiding errors like this
			if strings.Contains(err.Error(), psnet.CloseErr) {
				return
			}
			s.log.Error("%v\n", err)
			return
		}

		go s.handleConnection(c, ps)
	}
}
