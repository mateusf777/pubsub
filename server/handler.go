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

func handleConnection(c net.Conn, ps *PubSub) {
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

	dataCh := make(chan []byte, 100)

	connReader, err := core.NewConnectionReader(core.ConnectionReaderConfig{
		Conn:     c,
		DataChan: dataCh,
	})
	if err != nil {
		slog.Error("Server.handleConnection", "error", err)
	}

	go func() {
		go connReader.Read()

		// dispatch
		for netData := range dataCh {
			inactivityReset <- true

			data := bytes.TrimSpace(netData)

			var result []byte
			switch {
			case bytes.Equal(bytes.ToUpper(data), core.OpStop), bytes.Equal(data, core.ControlC):
				slog.Info("Closing connection", "remote", c.RemoteAddr().String())
				stopInactivityMonitor <- true
				closeHandler <- true
				break

			case bytes.Equal(bytes.ToUpper(data), core.OpPing):
				result = core.BuildBytes(core.OpPong, core.CRLF)
				break

			case bytes.Equal(bytes.ToUpper(data), core.OpPong):
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
				if bytes.Equal(data, core.Empty) {
					continue
				}
				result = core.BuildBytes([]byte("-ERR invalid protocol"), core.CRLF)
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

	keepAlive, err := core.NewKeepAlive(core.KeepAliveConfig{
		Conn:    c,
		ResetCh: inactivityReset,
		StopCh:  stopInactivityMonitor,
		CloseCh: closeHandler,
	})
	if err != nil {
		slog.Error("Server handler handleConnection", "error", err)
	}

	go keepAlive.Run()

	<-closeHandler
	ps.UnsubAll(client)
	return
}

// handlePub Handles PUB message. See core.OpPub.
func handlePub(c net.Conn, ps *PubSub, client string, received []byte, dataCh chan []byte) []byte {
	// default result
	result := core.OK

	// Get next line
	msg := <-dataCh

	// parse
	args := bytes.Split(received, core.Space)

	if len(args) < 2 || len(args) > 3 {
		return []byte("-ERR should be PUB <subject> [reply-to]\n")
	}

	opts := make([]PubOpt, 0)

	// subscribe for reply
	if len(args) == 3 {
		reply := args[2]
		if err := ps.Subscribe(string(reply), client, func(msg Message) {
			result = core.BuildBytes(core.OpMsg, core.Space, []byte(msg.Subject), core.Space, []byte(msg.Reply), core.CRLF, msg.Data, core.CRLF)
			slog.Debug("pub", "value", result)
			_, err := c.Write(result)
			if err != nil {
				slog.Error("server handler handlePub", "error", err)
			}
		}); err != nil {
			return core.BuildBytes(core.OpERR, core.Space, []byte(err.Error()))
		}
		opts = append(opts, WithReply(string(reply)))
	}

	// dispatch
	ps.Publish(string(args[1]), msg, opts...)

	return result
}

// handleUnsub Handles UNSUB. See core.OpUnsub.
func handleUnsub(ps *PubSub, client string, received []byte) []byte {
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
		return core.BuildBytes(core.OpERR, core.Space, []byte(err.Error()))
	}
	return result
}

// handleSub Handles SUB. See core.OpSub.
func handleSub(c net.Conn, ps *PubSub, client string, received []byte) []byte {
	// default result
	result := core.OK

	// parse
	args := bytes.Split(received, core.Space)
	if len(args) < 3 || len(args) > 4 {
		return []byte("-ERR should be SUB <subject> <id> [group]\n")
	}

	id, _ := strconv.Atoi(string(args[2]))
	opts := make([]SubOpt, 0)

	if len(args) == 4 {
		group := args[3]
		opts = append(opts, WithGroup(string(group)))
	}
	opts = append(opts, WithID(id))

	// dispatch
	err := ps.Subscribe(string(args[1]), client, func(msg Message) {
		err := sendMsg(c, id, msg)
		if err != nil {
			slog.Error("send", "error", err)
		}
	}, opts...)
	if err != nil {
		return core.BuildBytes(core.OpERR, core.Space, []byte(err.Error()))
	}

	return result
}

// sendMsg sends MSG back to client. See core.OpMsg.
func sendMsg(conn net.Conn, id int, msg Message) error {

	subID := strconv.AppendInt([]byte{}, int64(id), 10)
	result := core.BuildBytes(core.OpMsg, core.Space, []byte(msg.Subject), core.Space, subID, core.Space, []byte(msg.Reply), core.CRLF, msg.Data, core.CRLF)
	slog.Debug("MSG message", "result", result)

	_, err := conn.Write(result)
	if err != nil {
		return fmt.Errorf("server handler sendMsg, %v", err)
	}

	return nil
}
