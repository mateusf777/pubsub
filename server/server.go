package server

import (
	"net"
	"strings"

	"github.com/mateusf777/pubsub/domain"
	"github.com/mateusf777/pubsub/log"
)

type Server struct {
	log log.Logger
}

type Opt func(s *Server)

func LogLevel(level log.Level) Opt {
	return func(s *Server) {
		s.log.Level = level
	}
}

func New(opts ...Opt) Server {
	s := Server{
		log: log.New().WithContext("pubsub server"),
	}

	for _, opt := range opts {
		opt(&s)
	}

	return s
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
	domain.Wait()
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
			if strings.Contains(err.Error(), domain.CloseErr) {
				return
			}
			s.log.Error("%v\n", err)
			return
		}

		go s.handleConnection(c, ps)
	}
}
