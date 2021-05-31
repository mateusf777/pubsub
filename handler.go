package main

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

const ttl = 5 * time.Minute

const (
	opStop  = "STOP"
	opPub   = "PUB"
	opSub   = "SUB"
	opUnsub = "UNSUB"
	opPong  = "PONG"
	opPing  = "PING"
)

const closeErr = "use of closed network connection"

func HandleConnection(c net.Conn, ps *PubSub) {
	defer func(c net.Conn) {
		err := c.Close()
		if err != nil {
			Log(ERR, "%v\n", err)
		}
		Log(INFO, "Closed connection %s\n", c.RemoteAddr().String())
	}(c)

	//
	closeHandler := make(chan bool)
	stopTimeout := make(chan bool)

	Log(INFO, "Serving %s\n", c.RemoteAddr().String())
	client := c.RemoteAddr().String()

	timeout := time.NewTicker(ttl)
	timeoutCount := 0

	go func() {
	Loop:
		for {
			netData, err := bufio.NewReader(c).ReadString('\n')
			if err != nil {
				if strings.Contains(err.Error(), closeErr) {
					return
				}
				Log(ERR, "%v\n", err)
				return
			}

			Log(DEBUG, "Resetting timeout...")
			timeout.Reset(ttl)
			timeoutCount = 0

			temp := strings.TrimSpace(netData)
			var result string
			switch {
			case strings.ToUpper(temp) == opStop:
				Log(INFO, "Closing connection with %s\n", c.RemoteAddr().String())
				stopTimeout <- true
				closeHandler <- true
				break Loop

			case strings.ToUpper(temp) == opPing:
				result = opPong + "\n"
				break

			case strings.ToUpper(temp) == opPong:
				result = "+OK\n"
				break

			case strings.HasPrefix(strings.ToUpper(temp), opPub):
				result = handlePub(c, ps, client, temp)

			case strings.HasPrefix(strings.ToUpper(temp), opSub):
				result = handleSub(c, ps, client, temp)

			case strings.HasPrefix(strings.ToUpper(temp), opUnsub):
				result = handleUnsub(ps, client, temp)

			default:
				result = "-ERR invalid protocol message\n"
			}
			_, err = c.Write([]byte(result))
			if err != nil {
				Log(ERR, "%v\n", err)
			}
		}
	}()

	go func() {
	Timeout:
		for {
			select {
			case <-stopTimeout:
				Log(DEBUG, "Stop timeout process %s\n", c.RemoteAddr().String())
				break Timeout

			case <-timeout.C:
				timeoutCount++
				if timeoutCount > 2 {
					Log(INFO, "Timeout %s\n", c.RemoteAddr().String())
					closeHandler <- true
					break Timeout
				}

				Log(DEBUG, "Sending timeout...")
				_, err := c.Write([]byte(opPing + "\n"))
				if err != nil {
					Log(ERR, "%v\n", err)
				}

			}
		}
	}()

	<-closeHandler
	return
}

func handlePub(c net.Conn, ps *PubSub, client string, received string) string {
	result := "+OK\n"
	args := strings.Split(received, " ")
	if len(args) < 2 || len(args) > 3 {
		return "-ERR should be PUB <subject> [reply-to]\n"
	}
	msg, err := bufio.NewReader(c).ReadString('\n')
	if err != nil {
		return fmt.Sprintf("-ERR %v\n", err)
	}

	opts := make([]PubOpt, 0)
	if len(args) == 3 {
		reply := args[2]
		err := ps.Subscribe(reply, client, -1, func(msg Message) {

			result = fmt.Sprintf("MSG %s %s\r\n%v\r\n", msg.Subject, msg.Reply, msg.Value)

			_, err := c.Write([]byte(result))
			if err != nil {
				Log(ERR, "%v\n", err)
			}

		}, WithMaxMsg(1))
		if err != nil {
			return fmt.Sprintf("-ERR %v\n", err)
		}
		opts = append(opts, WithReply(reply))
	}

	err = ps.Publish(args[1], msg[:len(msg)-2], opts...)
	if err != nil {
		return fmt.Sprintf("-ERR %v\n", err)
	}

	return result
}

func handleUnsub(ps *PubSub, client string, received string) string {
	result := "+OK\n"

	args := strings.Split(received, " ")
	if len(args) != 3 {
		return "-ERR should be UNSUB <subject> <id>\n"
	}
	id, _ := strconv.Atoi(args[2])

	err := ps.Unsubscribe(args[1], client, id)
	if err != nil {
		return fmt.Sprintf("-ERR %v\n", err)
	}
	return result
}

func handleSub(c net.Conn, ps *PubSub, client string, received string) string {
	result := "+OK\n"

	args := strings.Split(received, " ")
	if len(args) < 3 || len(args) > 5 {
		return "-ERR should be SUB <subject> <id> [max-msg] [group]\n"
	}

	id, _ := strconv.Atoi(args[2])
	opts := make([]SubOpt, 0)

	if len(args) == 4 {
		maxMsg, _ := strconv.Atoi(args[3])
		opts = append(opts, WithMaxMsg(maxMsg))
	}
	if len(args) == 5 {
		group := args[4]
		opts = append(opts, WithGroup(group))
	}

	err := ps.Subscribe(args[1], client, id, func(msg Message) {
		result = fmt.Sprintf("MSG %s %d %s\r\n%v\r\n", msg.Subject, id, msg.Reply, msg.Value)
		_, err := c.Write([]byte(result))
		if err != nil {
			Log(ERR, "%v\n", err)
		}
	}, opts...)
	if err != nil {
		return fmt.Sprintf("-ERR %v\n", err)
	}

	return result
}
