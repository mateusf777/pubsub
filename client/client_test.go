package client

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/mateusf777/pubsub/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestClient_Close(t *testing.T) {
	mockConnHandler := NewMockConnectionHandler(t)
	mockConnHandler.On("Close").Return().Once()

	client := &Client{
		connHandler: mockConnHandler,
	}

	client.Close()
}

func TestClient_Publish(t *testing.T) {
	type args struct {
		subject string
		msg     []byte
	}
	tests := []struct {
		name    string
		reader  func(r *io.PipeReader)
		args    args
		wantErr bool
	}{
		{
			name: "Publish",
			reader: func(r *io.PipeReader) {
				buf := make([]byte, 1024)
				n, _ := r.Read(buf)
				assert.Equal(t, core.BuildBytes(core.OpPub, core.Space, []byte("test"), core.CRLF, []byte("test"), core.CRLF), buf[:n])
			},
			args: args{
				subject: "test",
				msg:     []byte("test"),
			},
			wantErr: false,
		},
		{
			name: "Publish with error",
			reader: func(r *io.PipeReader) {
				_ = r.Close()
			},
			args: args{
				subject: "test",
				msg:     []byte("test"),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		testReader, testWriter := io.Pipe()

		if tt.reader != nil {
			go tt.reader(testReader)
		}

		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				writer: testWriter,
			}
			if err := c.Publish(tt.args.subject, tt.args.msg); (err != nil) != tt.wantErr {
				t.Errorf("Publish() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClient_Subscribe(t *testing.T) {
	type fields struct {
		reader func(r *io.PipeReader)
		router func(m *Mockrouter)
	}
	type args struct {
		subject string
		handler Handler
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    int
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "Subscribe",
			fields: fields{
				reader: func(r *io.PipeReader) {
					buf := make([]byte, 1024)
					n, _ := r.Read(buf)
					assert.Equal(t, core.BuildBytes(core.OpSub, core.Space, []byte("test"), core.Space, []byte("1"), core.CRLF), buf[:n])
				},
				router: func(m *Mockrouter) {
					m.On("addSubHandler", mock.MatchedBy(func(h Handler) bool {
						msg := &Message{Subject: "test", Data: []byte("test")}
						h(msg)
						return true
					})).Return(1).Once()
				},
			},
			args: args{
				subject: "test",
				handler: func(message *Message) {
					assert.Equal(t, message.Subject, "test")
					assert.Equal(t, message.Data, []byte("test"))
				},
			},
			want:    1,
			wantErr: assert.NoError,
		},
		{
			name: "Subscribe with error",
			fields: fields{
				reader: func(r *io.PipeReader) {
					_ = r.Close()
				},
				router: func(m *Mockrouter) {
					m.On("addSubHandler", mock.Anything).Return(1).Once()
				},
			},
			want:    -1,
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testReader, testWriter := io.Pipe()
			mockRouter := NewMockrouter(t)

			if tt.fields.reader != nil {
				go tt.fields.reader(testReader)
			}

			if tt.fields.router != nil {
				tt.fields.router(mockRouter)
			}

			c := &Client{
				writer: testWriter,
				router: mockRouter,
			}
			got, err := c.Subscribe(tt.args.subject, tt.args.handler)
			if !tt.wantErr(t, err) {
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_Unsubscribe(t *testing.T) {
	type fields struct {
		reader func(r *io.PipeReader)
		router func(m *Mockrouter)
	}
	type args struct {
		subject      string
		subscriberID int
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "Unsubscribe",
			fields: fields{
				reader: func(r *io.PipeReader) {
					buf := make([]byte, 1024)
					n, _ := r.Read(buf)
					assert.Equal(t, core.BuildBytes(core.OpUnsub, core.Space, []byte("subject"), core.Space, []byte("1"), core.CRLF), buf[:n])
				},
				router: func(m *Mockrouter) {
					m.On("removeSubHandler", 1).Return().Once()
				},
			},
			args: args{
				subject:      "subject",
				subscriberID: 1,
			},
			wantErr: assert.NoError,
		},
		{
			name: "Unsubscribe with error",
			fields: fields{
				reader: func(r *io.PipeReader) {
					_ = r.Close()
				},
				router: func(m *Mockrouter) {
					m.On("removeSubHandler", 1).Return().Once()
				},
			},
			args: args{
				subject:      "subject",
				subscriberID: 1,
			},
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testReader, testWriter := io.Pipe()
			mockRouter := NewMockrouter(t)

			if tt.fields.reader != nil {
				go tt.fields.reader(testReader)
			}

			if tt.fields.router != nil {
				tt.fields.router(mockRouter)
			}

			c := &Client{
				writer: testWriter,
				router: mockRouter,
			}

			tt.wantErr(t, c.Unsubscribe(tt.args.subject, tt.args.subscriberID), fmt.Sprintf("Unsubscribe(%v, %v)", tt.args.subject, tt.args.subscriberID))
		})
	}
}

func TestClient_QueueSubscribe(t *testing.T) {
	type fields struct {
		reader func(r *io.PipeReader)
		router func(m *Mockrouter)
	}
	type args struct {
		subject string
		queue   string
		handler Handler
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    int
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "QueueSubscribe",
			fields: fields{
				reader: func(r *io.PipeReader) {
					buf := make([]byte, 1024)
					n, _ := r.Read(buf)
					assert.Equal(t, core.BuildBytes(core.OpSub, core.Space, []byte("test"), core.Space, []byte("1"), core.Space, []byte("test"), core.CRLF), buf[:n])
				},
				router: func(m *Mockrouter) {
					m.On("addSubHandler", mock.MatchedBy(func(h Handler) bool {
						msg := &Message{Subject: "test", Data: []byte("test")}
						h(msg)
						return true
					})).Return(1).Once()
				},
			},
			args: args{
				subject: "test",
				queue:   "test",
				handler: func(message *Message) {
					assert.Equal(t, message.Subject, "test")
					assert.Equal(t, message.Data, []byte("test"))
				},
			},
			want:    1,
			wantErr: assert.NoError,
		},
		{
			name: "QueueSubscribe with error",
			fields: fields{
				reader: func(r *io.PipeReader) {
					_ = r.Close()
				},
				router: func(m *Mockrouter) {
					m.On("addSubHandler", mock.Anything).Return(1).Once()
				},
			},
			want:    -1,
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testReader, testWriter := io.Pipe()
			mockRouter := NewMockrouter(t)

			if tt.fields.reader != nil {
				go tt.fields.reader(testReader)
			}

			if tt.fields.router != nil {
				tt.fields.router(mockRouter)
			}

			c := &Client{
				writer: testWriter,
				router: mockRouter,
			}
			got, err := c.QueueSubscribe(tt.args.subject, tt.args.queue, tt.args.handler)
			if !tt.wantErr(t, err) {
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_Request(t *testing.T) {
	ctxTimeout, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	type fields struct {
		reader    func(r *io.PipeReader)
		router    func(m *Mockrouter)
		generator func(m *MockuniqueGenerator)
	}
	type args struct {
		subject string
		msg     []byte
		ctx     context.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *Message
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "Request",
			fields: fields{
				reader: func(r *io.PipeReader) {
					buf := make([]byte, 1024)
					n, _ := r.Read(buf)
					assert.Equal(t, core.BuildBytes(core.OpSub, core.Space, []byte("REPLY.1"), core.Space, []byte("1"), core.CRLF), buf[:n])

					n, _ = r.Read(buf)
					assert.Equal(t, core.BuildBytes(core.OpPub, core.Space, []byte("test"), core.Space, []byte("REPLY.1"), core.CRLF, []byte("test"), core.CRLF), buf[:n])

					n, _ = r.Read(buf)
					assert.Equal(t, core.BuildBytes(core.OpUnsub, core.Space, []byte("REPLY.1"), core.Space, []byte("1"), core.CRLF), buf[:n])
				},
				router: func(m *Mockrouter) {
					m.On("addSubHandler", mock.MatchedBy(func(h Handler) bool {
						go time.AfterFunc(50*time.Millisecond, func() {
							msg := &Message{Subject: "test", Data: []byte("test")}
							h(msg)
						})
						return true
					})).Return(1).Once()
					m.On("removeSubHandler", 1).Return().Once()
				},
				generator: func(m *MockuniqueGenerator) {
					m.On("nextUnique").Return("1").Once()
				},
			},
			args: args{
				subject: "test",
				msg:     []byte("test"),
			},
			want: &Message{
				Subject: "test",
				Data:    []byte("test"),
			},
			wantErr: assert.NoError,
		},
		{
			name: "Request with subscribe error",
			fields: fields{
				reader: func(r *io.PipeReader) {
					_ = r.Close()
				},
				router: func(m *Mockrouter) {
					m.On("addSubHandler", mock.Anything).Return(1).Once()
				},
				generator: func(m *MockuniqueGenerator) {
					m.On("nextUnique").Return("1").Once()
				},
			},
			args: args{
				subject: "test",
				msg:     []byte("test"),
			},
			want:    nil,
			wantErr: assert.Error,
		},
		{
			name: "Request with write error",
			fields: fields{
				reader: func(r *io.PipeReader) {
					buf := make([]byte, 1024)
					n, _ := r.Read(buf)
					assert.Equal(t, core.BuildBytes(core.OpSub, core.Space, []byte("REPLY.1"), core.Space, []byte("1"), core.CRLF), buf[:n])

					_ = r.Close()
				},
				router: func(m *Mockrouter) {
					m.On("addSubHandler", mock.Anything).Return(1).Once()
				},
				generator: func(m *MockuniqueGenerator) {
					m.On("nextUnique").Return("1").Once()
				},
			},
			args: args{
				subject: "test",
				msg:     []byte("test"),
			},
			want:    nil,
			wantErr: assert.Error,
		},
		{
			name: "Request with unsubscribe error",
			fields: fields{
				reader: func(r *io.PipeReader) {
					buf := make([]byte, 1024)
					n, _ := r.Read(buf)
					assert.Equal(t, core.BuildBytes(core.OpSub, core.Space, []byte("REPLY.1"), core.Space, []byte("1"), core.CRLF), buf[:n])

					n, _ = r.Read(buf)
					assert.Equal(t, core.BuildBytes(core.OpPub, core.Space, []byte("test"), core.Space, []byte("REPLY.1"), core.CRLF, []byte("test"), core.CRLF), buf[:n])

					_ = r.Close()
				},
				router: func(m *Mockrouter) {
					m.On("addSubHandler", mock.MatchedBy(func(h Handler) bool {
						go time.AfterFunc(50*time.Millisecond, func() {
							msg := &Message{Subject: "test", Data: []byte("test")}
							h(msg)
						})
						return true
					})).Return(1).Once()
					m.On("removeSubHandler", 1).Return().Once()
				},
				generator: func(m *MockuniqueGenerator) {
					m.On("nextUnique").Return("1").Once()
				},
			},
			args: args{
				subject: "test",
				msg:     []byte("test"),
			},
			want:    nil,
			wantErr: assert.Error,
		},
		{
			name: "Request with context timeout",
			fields: fields{
				reader: func(r *io.PipeReader) {
					buf := make([]byte, 1024)
					n, _ := r.Read(buf)
					assert.Equal(t, core.BuildBytes(core.OpSub, core.Space, []byte("REPLY.1"), core.Space, []byte("1"), core.CRLF), buf[:n])

					n, _ = r.Read(buf)
					assert.Equal(t, core.BuildBytes(core.OpPub, core.Space, []byte("test"), core.Space, []byte("REPLY.1"), core.CRLF, []byte("test"), core.CRLF), buf[:n])
				},
				router: func(m *Mockrouter) {
					m.On("addSubHandler", mock.MatchedBy(func(h Handler) bool {
						return true
					})).Return(1).Once()
				},
				generator: func(m *MockuniqueGenerator) {
					m.On("nextUnique").Return("1").Once()
				},
			},
			args: args{
				subject: "test",
				msg:     []byte("test"),
				ctx:     ctxTimeout,
			},
			want:    nil,
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testReader, testWriter := io.Pipe()
			mockRouter := NewMockrouter(t)
			mockGenerator := NewMockuniqueGenerator(t)

			if tt.fields.reader != nil {
				go tt.fields.reader(testReader)
			}

			if tt.fields.router != nil {
				tt.fields.router(mockRouter)
			}

			if tt.fields.generator != nil {
				tt.fields.generator(mockGenerator)
			}

			c := &Client{
				writer:    testWriter,
				router:    mockRouter,
				generator: mockGenerator,
			}

			var (
				got *Message
				err error
			)

			if tt.args.ctx != nil {
				got, err = c.RequestWithCtx(tt.args.ctx, tt.args.subject, tt.args.msg)
			} else {
				got, err = c.Request(tt.args.subject, tt.args.msg)
			}
			if !tt.wantErr(t, err, fmt.Sprintf("Request(%v, %v)", tt.args.subject, tt.args.msg)) {
				return
			}
			assert.Equalf(t, tt.want, got, "Request(%v, %v)", tt.args.subject, tt.args.msg)
		})
	}
}

func TestClient_Reply(t *testing.T) {
	type fields struct {
		reader func(r *io.PipeReader)
	}
	type args struct {
		msg  *Message
		data []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "Reply",
			fields: fields{
				reader: func(r *io.PipeReader) {
					buf := make([]byte, 1024)
					n, _ := r.Read(buf)
					assert.Equal(t, core.BuildBytes(core.OpPub, core.Space, []byte("test"), core.CRLF, []byte("test"), core.CRLF), buf[:n])
				},
			},
			args: args{
				msg: &Message{
					Reply: "test",
				},
				data: []byte("test"),
			},
			wantErr: assert.NoError,
		},
		{
			name: "Reply with no reply",
			args: args{
				msg:  &Message{},
				data: []byte("test"),
			},
			wantErr: assert.NoError,
		},
		{
			name: "Reply with publish error",
			fields: fields{
				reader: func(r *io.PipeReader) {
					_ = r.Close()
				},
			},
			args: args{
				msg: &Message{
					Reply: "test",
				},
				data: []byte("test"),
			},
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			testReader, testWriter := io.Pipe()
			mockRouter := NewMockrouter(t)

			if tt.fields.reader != nil {
				go tt.fields.reader(testReader)
			}

			c := &Client{
				writer: testWriter,
				router: mockRouter,
			}
			tt.wantErr(t, c.Reply(tt.args.msg, tt.args.data), fmt.Sprintf("Reply(%v, %v)", tt.args.msg, tt.args.data))
		})
	}
}
