package server

import (
	"bytes"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"

	"github.com/mateusf777/pubsub/domain"
)

func (s Server) handleConnection(c net.Conn, ps *domain.PubSub) {
	defer func(c net.Conn) {
		err := c.Close()
		if err != nil {
			slog.Error("Servier.handleConnection", "error", err)
		}
		slog.Info("Closed connection", "remote", c.RemoteAddr().String())
	}(c)

	closeHandler := make(chan bool)
	stopTimeout := make(chan bool)

	slog.Info("Serving", "remote", c.RemoteAddr().String())
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
					slog.Info("Closing connection", "remote", c.RemoteAddr().String())
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
					slog.Debug("sub", "value", temp)
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
					slog.Error("server handler handleConnection", "error", err)
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
			slog.Debug("pub", "value", result)
			_, err := c.Write(result)
			if err != nil {
				slog.Error("server handler handlePub", "error", err)
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
			slog.Error("send", "error", err)
		}
	}, opts...)
	if err != nil {
		return domain.Join(domain.OpERR, domain.Space, []byte(err.Error()))
	}

	return result
}

func (s Server) sendMsg(conn net.Conn, id int, msg domain.Message) error {
	result := domain.Join(domain.OpMsg, domain.Space, []byte(msg.Subject), domain.Space, []byte(strconv.Itoa(id)), domain.Space, []byte(msg.Reply), domain.CRLF, msg.Data, domain.CRLF)
	slog.Debug("join", "result", result)
	_, err := conn.Write(result)
	if err != nil {
		return fmt.Errorf("server handler sendMsg, %v", err)
	}
	return nil
}
