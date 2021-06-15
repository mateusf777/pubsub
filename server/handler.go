package server

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
	"strings"

	psnet "github.com/mateusf777/pubsub/net"

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
		go psnet.Read(c, buffer, dataCh)
	Loop:
		for {

			// dispatch
			accumulator := psnet.Empty
			for netData := range dataCh {
				timeoutReset <- true

				temp := bytes.TrimSpace(netData)

				if bytes.Compare(accumulator, psnet.Empty) != 0 {
					temp = domain.Join(accumulator, psnet.CRLF, temp)
				}

				var result []byte
				switch {
				case bytes.Compare(bytes.ToUpper(temp), psnet.OpStop) == 0, bytes.Compare(temp, psnet.ControlC) == 0:
					s.log.Info("Closing connection with %s\n", c.RemoteAddr().String())
					stopTimeout <- true
					closeHandler <- true
					break Loop

				case bytes.Compare(bytes.ToUpper(temp), psnet.OpPing) == 0:
					result = domain.Join(psnet.OpPong, psnet.CRLF)
					break

				case bytes.Compare(bytes.ToUpper(temp), psnet.OpPong) == 0:
					result = psnet.OK
					break

				case bytes.HasPrefix(bytes.ToUpper(temp), psnet.OpPub):
					// uses accumulator to get next line
					if bytes.Compare(bytes.ToUpper(accumulator), psnet.Empty) == 0 {
						accumulator = temp
						continue
					}
					result = s.handlePub(c, ps, client, temp)

				case bytes.HasPrefix(bytes.ToUpper(temp), psnet.OpSub):
					s.log.Debug("sub: %v", temp)
					result = s.handleSub(c, ps, client, temp)

				case bytes.HasPrefix(bytes.ToUpper(temp), psnet.OpUnsub):
					result = handleUnsub(ps, client, temp)

				default:
					if bytes.Compare(temp, psnet.Empty) == 0 {
						continue
					}
					result = domain.Join([]byte("-ERR invalid protocol"), psnet.CRLF)
				}
				_, err := c.Write(result)
				if err != nil {
					if strings.Contains(err.Error(), "broken pipe") || strings.Contains(err.Error(), "connection reset by peer") {
						continue
					}
					s.log.Error("server handler handleConnection, %v\n", err)
				}

				// reset accumulator
				accumulator = psnet.Empty
			}
		}
	}()

	go psnet.MonitorTimeout(c, timeoutReset, stopTimeout, closeHandler)

	<-closeHandler
	ps.UnsubAll(client)
	return
}

func (s Server) handlePub(c net.Conn, ps *domain.PubSub, client string, received []byte) []byte {
	// default result
	result := psnet.OK

	// parse
	parts := bytes.Split(received, psnet.CRLF)
	args := bytes.Split(parts[0], psnet.Space)
	msg := parts[1]

	if len(args) < 2 || len(args) > 3 {
		return []byte("-ERR should be PUB <subject> [reply-to]\n")
	}

	opts := make([]domain.PubOpt, 0)
	// subscribe for reply
	if len(args) == 3 {
		reply := args[2]
		err := ps.Subscribe(string(reply), client, func(msg domain.Message) {
			result = domain.Join(psnet.OpMsg, psnet.Space, []byte(msg.Subject), psnet.Space, []byte(msg.Reply), psnet.CRLF, msg.Data, psnet.CRLF)
			s.log.Debug("pub sending %s", result)
			_, err := c.Write(result)
			if err != nil {
				s.log.Error("server handler handlePub, %v\n", err)
			}

		}, domain.WithMaxMsg(1))
		if err != nil {
			return domain.Join(psnet.OpERR, psnet.Space, []byte(err.Error()))
		}
		opts = append(opts, domain.WithReply(string(reply)))
	}

	// dispatch
	err := ps.Publish(string(args[1]), msg, opts...)
	if err != nil {
		return domain.Join(psnet.OpERR, psnet.Space, []byte(err.Error()))
	}

	return result
}

func handleUnsub(ps *domain.PubSub, client string, received []byte) []byte {
	// default result
	result := psnet.OK

	// parse
	args := bytes.Split(received, psnet.Space)
	if len(args) != 3 {
		return []byte("-ERR should be UNSUB <subject> <id>\n")
	}
	id, _ := strconv.Atoi(string(args[2]))

	// dispatch
	err := ps.Unsubscribe(string(args[1]), client, id)
	if err != nil {
		return domain.Join(psnet.OpERR, psnet.Space, []byte(err.Error()))
	}
	return result
}

func (s Server) handleSub(c net.Conn, ps *domain.PubSub, client string, received []byte) []byte {
	// default result
	result := psnet.OK

	// parse
	args := bytes.Split(received, psnet.Space)
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
		return domain.Join(psnet.OpERR, psnet.Space, []byte(err.Error()))
	}

	return result
}

func (s Server) sendMsg(conn net.Conn, id int, msg domain.Message) error {
	result := domain.Join(psnet.OpMsg, psnet.Space, []byte(msg.Subject), psnet.Space, []byte(strconv.Itoa(id)), psnet.Space, []byte(msg.Reply), psnet.CRLF, msg.Data, psnet.CRLF)
	s.log.Debug("%s", result)
	_, err := conn.Write(result)
	if err != nil {
		return fmt.Errorf("server handler sendMsg, %v", err)
	}
	return nil
}
