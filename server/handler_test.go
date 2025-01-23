package server

import (
	"errors"
	"github.com/mateusf777/pubsub/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"net"
	"sync"
	"testing"
	"time"
)

func TestNewConnectionHandler(t *testing.T) {

	dataCh := make(chan []byte)
	resetCh := make(chan bool)
	stopCh := make(chan bool)
	closeCh := make(chan bool)

	mockConn := NewMockClientConn(t)
	mockPubSub := NewMockPubSubConn(t)
	mockConnReader := NewMockConnReader(t)
	mockMsgProc := NewMockMessageProcessor(t)
	mockKeepAlive := NewMockKeepAlive(t)

	type args struct {
		cfg ConnectionHandlerConfig
	}
	tests := []struct {
		name    string
		args    args
		want    *ConnectionHandler
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "Create ConnectionHandler",
			args: args{
				cfg: ConnectionHandlerConfig{
					Conn:            mockConn,
					PubSub:          mockPubSub,
					ConnReader:      mockConnReader,
					MsgProc:         mockMsgProc,
					KeepAlive:       mockKeepAlive,
					Data:            dataCh,
					ResetInactivity: resetCh,
					StopKeepAlive:   stopCh,
					CloseHandler:    closeCh,
				},
			},
			want: &ConnectionHandler{
				conn:            mockConn,
				pubSub:          mockPubSub,
				connReader:      mockConnReader,
				msgProc:         mockMsgProc,
				keepAlive:       mockKeepAlive,
				data:            dataCh,
				resetInactivity: resetCh,
				stopKeepAlive:   stopCh,
				closeHandler:    closeCh,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewConnectionHandler(tt.args.cfg)
			assert.Equalf(t, tt.want, got, "NewConnectionHandler(%v)", tt.args.cfg)
		})
	}
}

func TestConnectionHandler_Handle(t *testing.T) {

	expectedAddr := &net.IPAddr{IP: []byte{127, 0, 0, 1}}
	expectedClient := expectedAddr.String()

	type fields struct {
		conn            func(m *MockClientConn)
		pubSub          func(m *MockPubSubConn)
		connReader      func(m *MockConnReader)
		msgProc         func(m *MockMessageProcessor)
		keepAlive       func(m *MockKeepAlive)
		data            chan []byte
		resetInactivity chan bool
		stopKeepAlive   chan bool
		closeHandler    chan bool
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name: "Handle",
			fields: fields{
				conn: func(m *MockClientConn) {
					m.On("RemoteAddr").Return(expectedAddr).Twice()
					m.On("Close").Return(nil).Once()
				},
				pubSub: func(m *MockPubSubConn) {
					m.On("UnsubAll", expectedClient).Return().Once()
				},
				connReader: func(m *MockConnReader) {
					m.On("Read").Return().Once()
				},
				msgProc: func(m *MockMessageProcessor) {
					m.On("Process").Return().Once()
				},
				keepAlive: func(m *MockKeepAlive) {
					m.On("Run").Return().Once()
				},
				data:            make(chan []byte),
				resetInactivity: make(chan bool),
				stopKeepAlive:   make(chan bool),
				closeHandler:    make(chan bool),
			},
		},
		{
			name: "Handle with Connection Close error",
			fields: fields{
				conn: func(m *MockClientConn) {
					m.On("RemoteAddr").Return(expectedAddr).Twice()
					m.On("Close").Return(errors.New("error")).Once()
				},
				pubSub: func(m *MockPubSubConn) {
					m.On("UnsubAll", expectedClient).Return().Once()
				},
				connReader: func(m *MockConnReader) {
					m.On("Read").Return().Once()
				},
				msgProc: func(m *MockMessageProcessor) {
					m.On("Process").Return().Once()
				},
				keepAlive: func(m *MockKeepAlive) {
					m.On("Run").Return().Once()
				},
				data:            make(chan []byte),
				resetInactivity: make(chan bool),
				stopKeepAlive:   make(chan bool),
				closeHandler:    make(chan bool),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := NewMockClientConn(t)
			mockPubSub := NewMockPubSubConn(t)
			mockConnReader := NewMockConnReader(t)
			mockMsgProc := NewMockMessageProcessor(t)
			mockKeepAlive := NewMockKeepAlive(t)

			if tt.fields.conn != nil {
				tt.fields.conn(mockConn)
			}

			if tt.fields.pubSub != nil {
				tt.fields.pubSub(mockPubSub)
			}

			if tt.fields.connReader != nil {
				tt.fields.connReader(mockConnReader)
			}

			if tt.fields.msgProc != nil {
				tt.fields.msgProc(mockMsgProc)
			}

			if tt.fields.keepAlive != nil {
				tt.fields.keepAlive(mockKeepAlive)
			}

			h := &ConnectionHandler{
				conn:            mockConn,
				pubSub:          mockPubSub,
				connReader:      mockConnReader,
				msgProc:         mockMsgProc,
				keepAlive:       mockKeepAlive,
				data:            tt.fields.data,
				resetInactivity: tt.fields.resetInactivity,
				stopKeepAlive:   tt.fields.stopKeepAlive,
				closeHandler:    tt.fields.closeHandler,
			}
			wg := sync.WaitGroup{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				h.Handle()
			}()
			h.closeHandler <- true
			wg.Wait()
		})
	}
}

func Test_messageProcessor_Process(t *testing.T) {
	expectedAddr := &net.IPAddr{IP: []byte{127, 0, 0, 1}}

	type fields struct {
		conn            func(m *MockClientConn)
		pubSub          func(m *MockPubSubConn)
		data            chan []byte
		resetInactivity chan bool
		stopKeepAlive   chan bool
		closeHandler    chan bool
	}
	tests := []struct {
		name     string
		fields   fields
		message  []byte
		message2 []byte
	}{
		{
			name: "Process PING",
			fields: fields{
				conn: func(m *MockClientConn) {
					m.On("RemoteAddr").Return(expectedAddr).Once()
					m.On("Write", core.BuildBytes(core.OpPong, core.CRLF)).Return(0, nil)
				},
				data:            make(chan []byte),
				resetInactivity: make(chan bool),
				stopKeepAlive:   make(chan bool),
				closeHandler:    make(chan bool),
			},
			message: core.Ping,
		},
		{
			name: "Process PONG",
			fields: fields{
				conn: func(m *MockClientConn) {
					m.On("RemoteAddr").Return(expectedAddr).Once()
					m.On("Write", core.OK).Return(0, nil)
				},
				data:            make(chan []byte),
				resetInactivity: make(chan bool),
				stopKeepAlive:   make(chan bool),
				closeHandler:    make(chan bool),
			},
			message: core.BuildBytes(core.OpPong, core.CRLF),
		},
		{
			name: "Process PUB",
			fields: fields{
				conn: func(m *MockClientConn) {
					m.On("RemoteAddr").Return(expectedAddr).Once()
					m.On("Write", core.OK).Return(0, nil)
				},
				pubSub: func(m *MockPubSubConn) {
					m.On("Publish", "test", []byte("test\r\n")).Return().Once()
				},
				data:            make(chan []byte),
				resetInactivity: make(chan bool),
				stopKeepAlive:   make(chan bool),
				closeHandler:    make(chan bool),
			},
			message:  core.BuildBytes(core.OpPub, core.Space, []byte("test"), core.CRLF),
			message2: core.BuildBytes([]byte("test"), core.CRLF),
		},
		{
			name: "Process PUB with reply",
			fields: fields{
				conn: func(m *MockClientConn) {
					m.On("RemoteAddr").Return(expectedAddr).Once()
					m.On("Write", core.OK).Return(0, nil)
				},
				pubSub: func(m *MockPubSubConn) {
					m.On("Publish", "test", []byte("test\r\n"), mock.MatchedBy(func(opt PubOpt) bool {
						var msg Message
						opt(&msg)
						return msg.Reply == "reply"
					})).Return().Once()
				},
				data:            make(chan []byte),
				resetInactivity: make(chan bool),
				stopKeepAlive:   make(chan bool),
				closeHandler:    make(chan bool),
			},
			message:  core.BuildBytes(core.OpPub, core.Space, []byte("test"), core.Space, []byte("reply"), core.CRLF),
			message2: core.BuildBytes([]byte("test"), core.CRLF),
		},
		{
			name: "Process invalid PUB message",
			fields: fields{
				conn: func(m *MockClientConn) {
					m.On("RemoteAddr").Return(expectedAddr).Once()
					m.On("Write", []byte("-ERR should be PUB <subject> [reply-to]   \n")).Return(0, nil)
				},
				data:            make(chan []byte),
				resetInactivity: make(chan bool),
				stopKeepAlive:   make(chan bool),
				closeHandler:    make(chan bool),
			},
			message: core.BuildBytes(core.OpPub, core.Space, core.CRLF),
		},
		{
			name: "Process UNSUB",
			fields: fields{
				conn: func(m *MockClientConn) {
					m.On("RemoteAddr").Return(expectedAddr).Once()
					m.On("Write", core.OK).Return(0, nil)
				},
				pubSub: func(m *MockPubSubConn) {
					m.On("Unsubscribe", "test", "127.0.0.1", 1).Return(nil).Once()
				},
				data:            make(chan []byte),
				resetInactivity: make(chan bool),
				stopKeepAlive:   make(chan bool),
				closeHandler:    make(chan bool),
			},
			message: core.BuildBytes(core.OpUnsub, core.Space, []byte("test"), core.Space, []byte("1"), core.CRLF),
		},
		{
			name: "Process invalid UNSUB",
			fields: fields{
				conn: func(m *MockClientConn) {
					m.On("RemoteAddr").Return(expectedAddr).Once()
					m.On("Write", []byte("-ERR should be UNSUB <subject> <id>\n")).Return(0, nil)
				},
				data:            make(chan []byte),
				resetInactivity: make(chan bool),
				stopKeepAlive:   make(chan bool),
				closeHandler:    make(chan bool),
			},
			message: core.BuildBytes(core.OpUnsub, core.Space, core.CRLF),
		},
		{
			name: "Process UNSUB with error",
			fields: fields{
				conn: func(m *MockClientConn) {
					m.On("RemoteAddr").Return(expectedAddr).Once()
					m.On("Write", core.BuildBytes(core.OpERR, core.Space, []byte("error"))).Return(0, nil)
				},
				pubSub: func(m *MockPubSubConn) {
					m.On("Unsubscribe", "test", "127.0.0.1", 1).Return(errors.New("error")).Once()
				},
				data:            make(chan []byte),
				resetInactivity: make(chan bool),
				stopKeepAlive:   make(chan bool),
				closeHandler:    make(chan bool),
			},
			message: core.BuildBytes(core.OpUnsub, core.Space, []byte("test"), core.Space, []byte("1"), core.CRLF),
		},
		{
			name: "Process SUB",
			fields: fields{
				conn: func(m *MockClientConn) {
					m.On("RemoteAddr").Return(expectedAddr).Once()
					m.On("Write", core.OK).Return(0, nil)
				},
				pubSub: func(m *MockPubSubConn) {
					m.On("Subscribe", "test", "127.0.0.1",
						mock.MatchedBy(func(handler Handler) bool {
							return handler != nil
						}),
						mock.MatchedBy(func(opt SubOpt) bool {
							var h HandlerSubject
							opt(&h)
							return h.id == 1
						})).Return(nil).Once()
				},
				data:            make(chan []byte),
				resetInactivity: make(chan bool),
				stopKeepAlive:   make(chan bool),
				closeHandler:    make(chan bool),
			},
			message: core.BuildBytes(core.OpSub, core.Space, []byte("test"), core.Space, []byte("1"), core.CRLF),
		},
		{
			name: "Process SUB with group",
			fields: fields{
				conn: func(m *MockClientConn) {
					m.On("RemoteAddr").Return(expectedAddr).Once()
					m.On("Write", core.OK).Return(0, nil)
				},
				pubSub: func(m *MockPubSubConn) {
					m.On("Subscribe", "test", "127.0.0.1",
						mock.MatchedBy(func(handler Handler) bool {
							return handler != nil
						}),
						mock.MatchedBy(func(opt SubOpt) bool {
							var h HandlerSubject
							opt(&h)
							return h.group == "test"
						}),
						mock.MatchedBy(func(opt SubOpt) bool {
							var h HandlerSubject
							opt(&h)
							return h.id == 1
						}),
					).Return(nil).Once()
				},
				data:            make(chan []byte),
				resetInactivity: make(chan bool),
				stopKeepAlive:   make(chan bool),
				closeHandler:    make(chan bool),
			},
			message: core.BuildBytes(core.OpSub, core.Space, []byte("test"), core.Space, []byte("1"), core.Space, []byte("test"), core.CRLF),
		},
		{
			name: "Process invalid SUB",
			fields: fields{
				conn: func(m *MockClientConn) {
					m.On("RemoteAddr").Return(expectedAddr).Once()
					m.On("Write", []byte("-ERR should be SUB <subject> <id> [group]\n")).Return(0, nil)
				},
				data:            make(chan []byte),
				resetInactivity: make(chan bool),
				stopKeepAlive:   make(chan bool),
				closeHandler:    make(chan bool),
			},
			message: core.BuildBytes(core.OpSub, core.Space, core.CRLF),
		},
		{
			name: "Process SUB with error",
			fields: fields{
				conn: func(m *MockClientConn) {
					m.On("RemoteAddr").Return(expectedAddr).Once()
					m.On("Write", core.BuildBytes(core.OpERR, core.Space, []byte("error"))).Return(0, nil)
				},
				pubSub: func(m *MockPubSubConn) {
					m.On("Subscribe", "test", "127.0.0.1",
						mock.MatchedBy(func(handler Handler) bool {
							return handler != nil
						}),
						mock.MatchedBy(func(opt SubOpt) bool {
							var h HandlerSubject
							opt(&h)
							return h.id == 1
						})).Return(errors.New("error")).Once()
				},
				data:            make(chan []byte),
				resetInactivity: make(chan bool),
				stopKeepAlive:   make(chan bool),
				closeHandler:    make(chan bool),
			},
			message: core.BuildBytes(core.OpSub, core.Space, []byte("test"), core.Space, []byte("1"), core.CRLF),
		},
		{
			name: "Process empty message",
			fields: fields{
				conn: func(m *MockClientConn) {
					m.On("RemoteAddr").Return(expectedAddr).Once()
				},
				data:            make(chan []byte),
				resetInactivity: make(chan bool),
				stopKeepAlive:   make(chan bool),
				closeHandler:    make(chan bool),
			},
			message: core.Empty,
		},
		{
			name: "Process UNKNOWN",
			fields: fields{
				conn: func(m *MockClientConn) {
					m.On("RemoteAddr").Return(expectedAddr).Once()
					m.On("Write", core.BuildBytes([]byte("-ERR invalid protocol"), core.CRLF)).Return(0, nil)
				},
				data:            make(chan []byte),
				resetInactivity: make(chan bool),
				stopKeepAlive:   make(chan bool),
				closeHandler:    make(chan bool),
			},
			message: []byte("UNKNOWN"),
		},
		{
			name: "Process message with write broken pipe error",
			fields: fields{
				conn: func(m *MockClientConn) {
					m.On("RemoteAddr").Return(expectedAddr).Once()
					m.On("Write", core.BuildBytes(core.OpPong, core.CRLF)).Return(0, errors.New("broken pipe"))
				},
				data:            make(chan []byte),
				resetInactivity: make(chan bool),
				stopKeepAlive:   make(chan bool),
				closeHandler:    make(chan bool),
			},
			message: core.Ping,
		},
		{
			name: "Process message with write reset error",
			fields: fields{
				conn: func(m *MockClientConn) {
					m.On("RemoteAddr").Return(expectedAddr).Once()
					m.On("Write", core.BuildBytes(core.OpPong, core.CRLF)).Return(0, errors.New("connection reset by peer"))
				},
				data:            make(chan []byte),
				resetInactivity: make(chan bool),
				stopKeepAlive:   make(chan bool),
				closeHandler:    make(chan bool),
			},
			message: core.Ping,
		},
		{
			name: "Process message with write error",
			fields: fields{
				conn: func(m *MockClientConn) {
					m.On("RemoteAddr").Return(expectedAddr).Once()
					m.On("Write", core.BuildBytes(core.OpPong, core.CRLF)).Return(0, errors.New("error"))
				},
				data:            make(chan []byte),
				resetInactivity: make(chan bool),
				stopKeepAlive:   make(chan bool),
				closeHandler:    make(chan bool),
			},
			message: core.Ping,
		},
		{
			name: "Process STOP",
			fields: fields{
				conn: func(m *MockClientConn) {
					m.On("RemoteAddr").Return(expectedAddr).Once()
				},
				data:            make(chan []byte),
				resetInactivity: make(chan bool),
				stopKeepAlive:   make(chan bool),
				closeHandler:    make(chan bool),
			},
			message: core.OpStop,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClientConn := NewMockClientConn(t)
			mockPubSub := NewMockPubSubConn(t)

			if tt.fields.conn != nil {
				tt.fields.conn(mockClientConn)
			}

			if tt.fields.pubSub != nil {
				tt.fields.pubSub(mockPubSub)
			}

			m := &messageProcessor{
				conn:            mockClientConn,
				pubSub:          mockPubSub,
				data:            tt.fields.data,
				resetInactivity: tt.fields.resetInactivity,
				stopKeepAlive:   tt.fields.stopKeepAlive,
				closeHandler:    tt.fields.closeHandler,
			}

			wg := sync.WaitGroup{}
			wg.Add(1)
			go func() {
				defer wg.Done()

				m.Process()
			}()

			//wg.Add(1)
			go func() {
				//	defer wg.Done()
				//loop:
				for {
					select {
					case <-m.resetInactivity:

					case <-m.stopKeepAlive:

					case <-m.closeHandler:

					default:

					}
				}
			}()

			tt.fields.data <- tt.message
			if tt.message2 != nil {
				tt.fields.data <- tt.message2
			}
			time.Sleep(10 * time.Millisecond)
			close(m.data)
			wg.Wait()
		})
	}
}

func Test_subscriberHandler(t *testing.T) {
	type args struct {
		conn func(m *MockClientConn)
		sid  int
	}
	tests := []struct {
		name string
		args args
		msg  Message
	}{
		{
			name: "subscriberHandler",
			args: args{
				conn: func(m *MockClientConn) {
					m.On("Write", core.BuildBytes(core.OpMsg, core.Space, []byte("test"), core.Space, []byte("1"), core.Space, []byte(""), core.CRLF, []byte("test"), core.CRLF)).Return(0, nil).Once()
				},
				sid: 1,
			},
			msg: Message{
				Subject: "test",
				Data:    []byte("test"),
			},
		},
		{
			name: "subscriberHandler with error",
			args: args{
				conn: func(m *MockClientConn) {
					m.On("Write", core.BuildBytes(core.OpMsg, core.Space, []byte("test"), core.Space, []byte("1"), core.Space, []byte(""), core.CRLF, []byte("test"), core.CRLF)).Return(0, errors.New("error")).Once()
				},
				sid: 1,
			},
			msg: Message{
				Subject: "test",
				Data:    []byte("test"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := NewMockClientConn(t)

			if tt.args.conn != nil {
				tt.args.conn(mockConn)
			}

			handler := subscriberHandler(mockConn, tt.args.sid)
			assert.NotNil(t, handler)
			handler(tt.msg)

		})
	}
}
