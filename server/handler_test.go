package server

import (
	"errors"
	"io"
	"net"
	"testing"

	"github.com/mateusf777/pubsub/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var expectedRemote = &net.IPAddr{IP: []byte{127, 0, 0, 1}}

func TestConnHandler_Run(t *testing.T) {
	type fields struct {
		connHandler func(m *MockConnectionHandler)
		pubSub      func(m *MockPubSubConn)
		remote      string
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name: "Run",
			fields: fields{
				connHandler: func(m *MockConnectionHandler) {
					m.On("Handle").Return().Once()
					m.On("Close").Return().Once()
				},
				pubSub: func(m *MockPubSubConn) {
					m.On("UnsubAll", expectedRemote.String()).Return().Once()
				},
				remote: expectedRemote.String(),
			},
		},
	}
	for _, tt := range tests {
		mockConnectionHandler := NewMockConnectionHandler(t)
		mockPubSubConn := NewMockPubSubConn(t)

		if tt.fields.connHandler != nil {
			tt.fields.connHandler(mockConnectionHandler)
		}

		if tt.fields.pubSub != nil {
			tt.fields.pubSub(mockPubSubConn)
		}

		t.Run(tt.name, func(t *testing.T) {
			s := &ConnHandler{
				connHandler: mockConnectionHandler,
				pubSub:      mockPubSubConn,
				remote:      tt.fields.remote,
			}
			s.Run()
		})
	}
}

func TestMessageHandler(t *testing.T) {
	type args struct {
		pubSub func(m *MockPubSubConn)
		reader func(reader *io.PipeReader)
		dataCh func(data chan []byte)
		close  func(close chan struct{})
		raw    []byte
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "STOP",
			args: args{
				close: func(close chan struct{}) {
					<-close
				},
				raw: core.OpStop,
			},
		},
		{
			name: "ControlC",
			args: args{
				close: func(close chan struct{}) {
					<-close
				},
				raw: core.ControlC,
			},
		},
		{
			name: "PING",
			args: args{
				raw: core.OpPing,
				reader: func(reader *io.PipeReader) {
					buf := make([]byte, 1024)
					n, _ := reader.Read(buf)
					assert.Equal(t, core.BuildBytes(core.OpPong, core.CRLF), buf[:n])
				},
			},
		},
		{
			name: "PONG",
			args: args{
				raw: core.OpPong,
				reader: func(reader *io.PipeReader) {
					buf := make([]byte, 1024)
					n, _ := reader.Read(buf)
					assert.Equal(t, core.OK, buf[:n])
				},
			},
		},
		{
			name: "PUB",
			args: args{
				raw: core.BuildBytes(core.OpPub, core.Space, []byte("test")),
				dataCh: func(data chan []byte) {
					data <- []byte("test")
				},
				pubSub: func(m *MockPubSubConn) {
					m.On("Publish", "test", []byte("test")).Return().Once()
				},
				reader: func(reader *io.PipeReader) {
					buf := make([]byte, 1024)
					n, _ := reader.Read(buf)
					assert.Equal(t, core.OK, buf[:n])
				},
			},
		},
		{
			name: "PUB with reply",
			args: args{
				raw: core.BuildBytes(core.OpPub, core.Space, []byte("test"), core.Space, []byte("test")),
				dataCh: func(data chan []byte) {
					data <- []byte("test")
				},
				pubSub: func(m *MockPubSubConn) {
					m.On("Publish", "test", []byte("test"), mock.MatchedBy(func(opt PubOpt) bool {
						var msg Message
						opt(&msg)
						return msg.Reply == "test"
					})).Return().Once()
				},
				reader: func(reader *io.PipeReader) {
					buf := make([]byte, 1024)
					n, _ := reader.Read(buf)
					assert.Equal(t, core.OK, buf[:n])
				},
			},
		},
		{
			name: "PUB malformed",
			args: args{
				raw: core.OpPub,
				dataCh: func(data chan []byte) {
					data <- []byte("test")
				},
				reader: func(reader *io.PipeReader) {
					buf := make([]byte, 1024)
					n, _ := reader.Read(buf)
					assert.Equal(t, []byte("-ERR should be PUB <subject> [reply-to]   \n"), buf[:n])
				},
			},
		},
		{
			name: "UNSUB",
			args: args{
				raw: core.BuildBytes(core.OpUnsub, core.Space, []byte("test"), core.Space, []byte("1")),
				pubSub: func(m *MockPubSubConn) {
					m.On("Unsubscribe", "test", expectedRemote.String(), 1).Return(nil).Once()
				},
				reader: func(reader *io.PipeReader) {
					buf := make([]byte, 1024)
					n, _ := reader.Read(buf)
					assert.Equal(t, core.OK, buf[:n])
				},
			},
		},
		{
			name: "UNSUB with error",
			args: args{
				raw: core.BuildBytes(core.OpUnsub, core.Space, []byte("test"), core.Space, []byte("1")),
				pubSub: func(m *MockPubSubConn) {
					m.On("Unsubscribe", "test", expectedRemote.String(), 1).Return(errors.New("error")).Once()
				},
				reader: func(reader *io.PipeReader) {
					buf := make([]byte, 1024)
					n, _ := reader.Read(buf)
					assert.Equal(t, core.BuildBytes(core.OpERR, core.Space, []byte("error")), buf[:n])
				},
			},
		},
		{
			name: "UNSUB malformed",
			args: args{
				raw: core.OpUnsub,
				reader: func(reader *io.PipeReader) {
					buf := make([]byte, 1024)
					n, _ := reader.Read(buf)
					assert.Equal(t, []byte("-ERR should be UNSUB <subject> <id>\n"), buf[:n])
				},
			},
		},
		{
			name: "SUB",
			args: args{
				raw: core.BuildBytes(core.OpSub, core.Space, []byte("test"), core.Space, []byte("1")),
				pubSub: func(m *MockPubSubConn) {
					m.On("Subscribe", "test", expectedRemote.String(),
						mock.MatchedBy(func(handler Handler) bool {
							return handler != nil
						}),
						mock.MatchedBy(func(opt SubOpt) bool {
							var hs HandlerSubject
							opt(&hs)
							return hs.id == 1
						})).Return(nil).Once()
				},
				reader: func(reader *io.PipeReader) {
					buf := make([]byte, 1024)
					n, _ := reader.Read(buf)
					assert.Equal(t, core.OK, buf[:n])
				},
			},
		},
		{
			name: "SUB with group",
			args: args{
				raw: core.BuildBytes(core.OpSub, core.Space, []byte("test"), core.Space, []byte("1"), core.Space, []byte("test")),
				pubSub: func(m *MockPubSubConn) {
					m.On("Subscribe", "test", expectedRemote.String(),
						mock.MatchedBy(func(handler Handler) bool {
							return handler != nil
						}),
						mock.MatchedBy(func(opt SubOpt) bool {
							var hs HandlerSubject
							opt(&hs)
							return hs.group == "test"
						}),
						mock.MatchedBy(func(opt SubOpt) bool {
							var hs HandlerSubject
							opt(&hs)
							return hs.id == 1
						}),
					).Return(nil).Once()
				},
				reader: func(reader *io.PipeReader) {
					buf := make([]byte, 1024)
					n, _ := reader.Read(buf)
					assert.Equal(t, core.OK, buf[:n])
				},
			},
		},
		{
			name: "SUB with error",
			args: args{
				raw: core.BuildBytes(core.OpSub, core.Space, []byte("test"), core.Space, []byte("1")),
				pubSub: func(m *MockPubSubConn) {
					m.On("Subscribe", "test", expectedRemote.String(),
						mock.MatchedBy(func(handler Handler) bool {
							return handler != nil
						}),
						mock.MatchedBy(func(opt SubOpt) bool {
							var hs HandlerSubject
							opt(&hs)
							return hs.id == 1
						}),
					).Return(errors.New("error")).Once()
				},
				reader: func(reader *io.PipeReader) {
					buf := make([]byte, 1024)
					n, _ := reader.Read(buf)
					assert.Equal(t, core.BuildBytes(core.OpERR, core.Space, []byte("error")), buf[:n])
				},
			},
		},
		{
			name: "SUB malformed",
			args: args{
				raw: core.OpSub,
				reader: func(reader *io.PipeReader) {
					buf := make([]byte, 1024)
					n, _ := reader.Read(buf)
					assert.Equal(t, []byte("-ERR should be SUB <subject> <id> [group]\n"), buf[:n])
				},
			},
		},
		{
			name: "UNKNOWN",
			args: args{
				raw: []byte("UNKNOWN"),
				reader: func(reader *io.PipeReader) {
					buf := make([]byte, 1024)
					n, _ := reader.Read(buf)
					assert.Equal(t, core.BuildBytes([]byte("-ERR invalid protocol"), core.CRLF), buf[:n])
				},
			},
		},
		{
			name: "Broken pipe",
			args: args{
				raw: []byte("UNKNOWN"),
				reader: func(reader *io.PipeReader) {
					_ = reader.CloseWithError(errors.New("broken pipe"))
				},
			},
		},
		{
			name: "Write error",
			args: args{
				raw: []byte("UNKNOWN"),
				reader: func(reader *io.PipeReader) {
					_ = reader.Close()
				},
			},
		},
		{
			name: "Empty",
			args: args{
				raw: core.Empty,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPubSubConn := NewMockPubSubConn(t)
			testReader, testWriter := io.Pipe()
			dataCh := make(chan []byte)
			closeCh := make(chan struct{})

			if tt.args.pubSub != nil {
				tt.args.pubSub(mockPubSubConn)
			}

			if tt.args.reader != nil {
				go tt.args.reader(testReader)
			}

			if tt.args.dataCh != nil {
				go tt.args.dataCh(dataCh)
			}

			if tt.args.close != nil {
				go tt.args.close(closeCh)
			}

			mh := MessageHandler(mockPubSubConn, expectedRemote.String())

			mh(testWriter, tt.args.raw, dataCh, closeCh)

		})
	}
}

func Test_subscriberHandler(t *testing.T) {
	tests := []struct {
		name   string
		msg    Message
		reader func(reader *io.PipeReader)
	}{
		{
			name: "subscriberHandler",
			msg: Message{
				Subject: "test",
				Reply:   "test",
				Data:    []byte("test"),
			},
			reader: func(reader *io.PipeReader) {
				buf := make([]byte, 1024)
				n, _ := reader.Read(buf)
				assert.Equal(t, core.BuildBytes(core.OpMsg, core.Space, []byte("test"), core.Space, []byte("1"), core.Space, []byte("test"), core.CRLF, []byte("test"), core.CRLF), buf[:n])
			},
		},
		{
			name: "subscriberHandler with write error",
			msg: Message{
				Subject: "test",
				Reply:   "test",
				Data:    []byte("test"),
			},
			reader: func(reader *io.PipeReader) {
				_ = reader.Close()
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testReader, testWriter := io.Pipe()

			if tt.reader != nil {
				go tt.reader(testReader)
			}

			handler := subscriberHandler(testWriter, 1)
			handler(tt.msg)
		})
	}
}
