package client

import (
	"github.com/mateusf777/pubsub/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"io"
	"testing"
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
