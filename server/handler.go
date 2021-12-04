package server

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/mateusf777/pubsub/domain"
)

func (s Server) handleConnection(c net.Conn, ps *domain.PubSub) {
	defer func(c net.Conn) {
		err := c.Close()
		if err != nil {
			s.log.Error("%v\n", err)
		}
		s.log.Info("Closed connection %s\n", c.RemoteAddr().String())
	}(c)

	closeHandler := make(chan bool)
	stopTimeout := make(chan bool)

	s.log.Info("Serving %s\n", c.RemoteAddr().String())
	client := c.RemoteAddr().String()

	timeoutReset := make(chan bool)

	buffer := make([]byte, 1024)
	dataCh := make(chan []byte, 100)

	go func() {
		go domain.Read(c, buffer, dataCh)
	Loop:
		for {

			// dispatch
			accumulator := domain.Empty
			for netData := range dataCh {
				timeoutReset <- true

				temp := bytes.TrimSpace(netData)

				if !domain.Equals(accumulator, domain.Empty) {
					temp = domain.Join(accumulator, domain.CRLF, temp)
				}

				var result []byte
				switch {
				case domain.Equals(bytes.ToUpper(temp), domain.OpStop), domain.Equals(temp, domain.ControlC):
					s.log.Info("Closing connection with %s\n", c.RemoteAddr().String())
					stopTimeout <- true
					closeHandler <- true
					break Loop

				case domain.Equals(bytes.ToUpper(temp), domain.OpPing):
					result = domain.Join(domain.OpPong, domain.CRLF)
					break

				case domain.Equals(bytes.ToUpper(temp), domain.OpPong):
					result = domain.OK
					break

				case bytes.HasPrefix(bytes.ToUpper(temp), domain.OpPub):
					// uses accumulator to get next line
					if domain.Equals(bytes.ToUpper(accumulator), domain.Empty) {
						accumulator = temp
						continue
					}
					result = s.handlePub(c, ps, client, temp)

				case bytes.HasPrefix(bytes.ToUpper(temp), domain.OpSub):
					s.log.Debug("sub: %v", temp)
					result = s.handleSub(c, ps, client, temp)

				case bytes.HasPrefix(bytes.ToUpper(temp), domain.OpUnsub):
					result = handleUnsub(ps, client, temp)

				default:
					if domain.Equals(temp, domain.Empty) {
						continue
					}
					result = domain.Join([]byte("-ERR invalid protocol"), domain.CRLF)
				}
				_, err := c.Write(result)
				if err != nil {
					if strings.Contains(err.Error(), "broken pipe") || strings.Contains(err.Error(), "connection reset by peer") {
						continue
					}
					s.log.Error("server handler handleConnection, %v\n", err)
				}

				// reset accumulator
				accumulator = domain.Empty
			}
		}
	}()

	go domain.MonitorTimeout(c, timeoutReset, stopTimeout, closeHandler)

	<-closeHandler
	ps.UnsubAll(client)
	return
}

func (s Server) handlePub(c net.Conn, ps *domain.PubSub, client string, received []byte) []byte {
	// default result
	result := domain.OK

	// parse
	parts := bytes.Split(received, domain.CRLF)
	args := bytes.Split(parts[0], domain.Space)
	msg := parts[1]

	if len(args) < 2 || len(args) > 3 {
		return []byte("-ERR should be PUB <subject> [reply-to]\n")
	}

	opts := make([]domain.PubOpt, 0)
	// subscribe for reply
	if len(args) == 3 {
		reply := args[2]
		err := ps.Subscribe(string(reply), client, func(msg domain.Message) {
			result = domain.Join(domain.OpMsg, domain.Space, []byte(msg.Subject), domain.Space, []byte(msg.Reply), domain.CRLF, msg.Data, domain.CRLF)
			s.log.Debug("pub sending %s", result)
			_, err := c.Write(result)
			if err != nil {
				s.log.Error("server handler handlePub, %v\n", err)
			}

		}, domain.WithMaxMsg(1))
		if err != nil {
			return domain.Join(domain.OpERR, domain.Space, []byte(err.Error()))
		}
		opts = append(opts, domain.WithReply(string(reply)))
	}

	// dispatch
	err := ps.Publish(string(args[1]), msg, opts...)
	if err != nil {
		return domain.Join(domain.OpERR, domain.Space, []byte(err.Error()))
	}

	return result
}

func handleUnsub(ps *domain.PubSub, client string, received []byte) []byte {
	// default result
	result := domain.OK

	// parse
	args := bytes.Split(received, domain.Space)
	if len(args) != 3 {
		return []byte("-ERR should be UNSUB <subject> <id>\n")
	}
	id, _ := strconv.Atoi(string(args[2]))

	// dispatch
	err := ps.Unsubscribe(string(args[1]), client, id)
	if err != nil {
		return domain.Join(domain.OpERR, domain.Space, []byte(err.Error()))
	}
	return result
}

func (s Server) handleSub(c net.Conn, ps *domain.PubSub, client string, received []byte) []byte {
	// default result
	result := domain.OK

	// parse
	args := bytes.Split(received, domain.Space)
	if len(args) < 3 || len(args) > 5 {
		return []byte("-ERR should be SUB <subject> <id> [max-msg] [group]\n")
	}

	id, _ := strconv.Atoi(string(args[2]))
	opts := make([]domain.SubOpt, 0)

	if len(args) == 4 {
		maxMsg, _ := strconv.Atoi(string(args[3]))
		opts = append(opts, domain.WithMaxMsg(maxMsg))
	}
	if len(args) == 5 {
		group := args[4]
		opts = append(opts, domain.WithGroup(string(group)))
	}
	opts = append(opts, domain.WithID(id))

	// dispatch
	err := ps.Subscribe(string(args[1]), client, func(msg domain.Message) {
		err := s.sendMsg(c, id, msg)
		if err != nil {
			s.log.Error("%v\n", err)
		}
	}, opts...)
	if err != nil {
		return domain.Join(domain.OpERR, domain.Space, []byte(err.Error()))
	}

	return result
}

func (s Server) sendMsg(conn net.Conn, id int, msg domain.Message) error {
	result := domain.Join(domain.OpMsg, domain.Space, []byte(msg.Subject), domain.Space, []byte(strconv.Itoa(id)), domain.Space, []byte(msg.Reply), domain.CRLF, msg.Data, domain.CRLF)
	s.log.Debug("%s", result)
	_, err := conn.Write(result)
	if err != nil {
		return fmt.Errorf("server handler sendMsg, %v", err)
	}
	return nil
}
