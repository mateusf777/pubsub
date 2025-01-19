package server

import (
	"bytes"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"

	"github.com/mateusf777/pubsub/core"
)

func handleConnection(c net.Conn, ps *core.PubSub) {
	defer func(c net.Conn) {
		err := c.Close()
		if err != nil {
			slog.Error("Server.handleConnection", "error", err)
		}
		slog.Info("Closed connection", "remote", c.RemoteAddr().String())
	}(c)

	closeHandler := make(chan bool)
	inactivityReset := make(chan bool)
	stopInactivityMonitor := make(chan bool)

	slog.Info("Serving", "remote", c.RemoteAddr().String())
	client := c.RemoteAddr().String()

	buffer := make([]byte, 1024)
	dataCh := make(chan []byte, 100)

	go func() {
		go core.Read(c, buffer, dataCh)

		// dispatch
		for netData := range dataCh {
			inactivityReset <- true

			data := bytes.TrimSpace(netData)

			var result []byte
			switch {
			case core.Equals(bytes.ToUpper(data), core.OpStop), core.Equals(data, core.ControlC):
				slog.Info("Closing connection", "remote", c.RemoteAddr().String())
				stopInactivityMonitor <- true
				closeHandler <- true
				break

			case core.Equals(bytes.ToUpper(data), core.OpPing):
				result = bytes.Join([][]byte{core.OpPong, core.CRLF}, nil)
				break

			case core.Equals(bytes.ToUpper(data), core.OpPong):
				result = core.OK
				break

			case bytes.HasPrefix(bytes.ToUpper(data), core.OpPub):
				result = handlePub(c, ps, client, data, dataCh)

			case bytes.HasPrefix(bytes.ToUpper(data), core.OpSub):
				slog.Debug("sub", "value", data)
				result = handleSub(c, ps, client, data)

			case bytes.HasPrefix(bytes.ToUpper(data), core.OpUnsub):
				result = handleUnsub(ps, client, data)

			default:
				if core.Equals(data, core.Empty) {
					continue
				}
				result = bytes.Join([][]byte{[]byte("-ERR invalid protocol"), core.CRLF}, nil)
			}

			_, err := c.Write(result)
			if err != nil {
				if strings.Contains(err.Error(), "broken pipe") || strings.Contains(err.Error(), "connection reset by peer") {
					continue
				}
				slog.Error("server handler handleConnection", "error", err)
			}

		}
	}()

	go core.MonitorInactivity(c, inactivityReset, stopInactivityMonitor, closeHandler)

	<-closeHandler
	ps.UnsubAll(client)
	return
}

func handlePub(c net.Conn, ps *core.PubSub, client string, received []byte, dataCh chan []byte) []byte {
	// default result
	result := core.OK

	// Get next line
	msg := <-dataCh

	// parse
	args := bytes.Split(received, core.Space)

	if len(args) < 2 || len(args) > 3 {
		return []byte("-ERR should be PUB <subject> [reply-to]\n")
	}

	opts := make([]core.PubOpt, 0)

	// subscribe for reply
	if len(args) == 3 {
		reply := args[2]
		err := ps.Subscribe(string(reply), client, func(msg core.Message) {
			result = bytes.Join([][]byte{core.OpMsg, core.Space, []byte(msg.Subject), core.Space, []byte(msg.Reply), core.CRLF, msg.Data, core.CRLF}, nil)
			slog.Debug("pub", "value", result)
			_, err := c.Write(result)
			if err != nil {
				slog.Error("server handler handlePub", "error", err)
			}

		}, core.WithMaxMsg(1))
		if err != nil {
			return bytes.Join([][]byte{core.OpERR, core.Space, []byte(err.Error())}, nil)
		}
		opts = append(opts, core.WithReply(string(reply)))
	}

	// dispatch
	err := ps.Publish(string(args[1]), msg, opts...)
	if err != nil {
		return bytes.Join([][]byte{core.OpERR, core.Space, []byte(err.Error())}, nil)
	}

	return result
}

func handleUnsub(ps *core.PubSub, client string, received []byte) []byte {
	// default result
	result := core.OK

	// parse
	args := bytes.Split(received, core.Space)
	if len(args) != 3 {
		return []byte("-ERR should be UNSUB <subject> <id>\n")
	}
	id, _ := strconv.Atoi(string(args[2]))

	// dispatch
	err := ps.Unsubscribe(string(args[1]), client, id)
	if err != nil {
		return bytes.Join([][]byte{core.OpERR, core.Space, []byte(err.Error())}, nil)
	}
	return result
}

func handleSub(c net.Conn, ps *core.PubSub, client string, received []byte) []byte {
	// default result
	result := core.OK

	// parse
	args := bytes.Split(received, core.Space)
	if len(args) < 3 || len(args) > 5 {
		return []byte("-ERR should be SUB <subject> <id> [max-msg] [group]\n")
	}

	id, _ := strconv.Atoi(string(args[2]))
	opts := make([]core.SubOpt, 0)

	if len(args) == 4 {
		maxMsg, _ := strconv.Atoi(string(args[3]))
		opts = append(opts, core.WithMaxMsg(maxMsg))
	}
	if len(args) == 5 {
		group := args[4]
		opts = append(opts, core.WithGroup(string(group)))
	}
	opts = append(opts, core.WithID(id))

	// dispatch
	err := ps.Subscribe(string(args[1]), client, func(msg core.Message) {
		err := sendMsg(c, id, msg)
		if err != nil {
			slog.Error("send", "error", err)
		}
	}, opts...)
	if err != nil {
		return bytes.Join([][]byte{core.OpERR, core.Space, []byte(err.Error())}, nil)
	}

	return result
}

func sendMsg(conn net.Conn, id int, msg core.Message) error {
	result := bytes.Join([][]byte{core.OpMsg, core.Space, []byte(msg.Subject), core.Space, []byte(strconv.Itoa(id)), core.Space, []byte(msg.Reply), core.CRLF, msg.Data, core.CRLF}, nil)
	slog.Debug("join", "result", result)
	_, err := conn.Write(result)
	if err != nil {
		return fmt.Errorf("server handler sendMsg, %v", err)
	}
	return nil
}
