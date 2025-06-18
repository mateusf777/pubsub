package client

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_msgRouter_addSubHandler(t *testing.T) {
	type fields struct {
		subHandlers map[int]Handler
	}
	type args struct {
		handler Handler
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   int
	}{
		{
			name: "addSubHandler",
			fields: fields{
				subHandlers: make(map[int]Handler),
			},
			args: args{
				handler: func(message *Message) {},
			},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := &msgRouter{
				subHandlers: tt.fields.subHandlers,
			}
			assert.Equal(t, tt.want, ps.addSubHandler(tt.args.handler))
		})
	}
}

func Test_msgRouter_removeSubHandler(t *testing.T) {
	type fields struct {
		subHandlers map[int]Handler
	}
	type args struct {
		subscriberID int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "removeSubHandler",
			fields: fields{
				subHandlers: map[int]Handler{
					1: func(message *Message) {},
				},
			},
			args: args{
				subscriberID: 1,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := &msgRouter{
				subHandlers: tt.fields.subHandlers,
			}
			ps.removeSubHandler(tt.args.subscriberID)
			assert.Equal(t, 0, len(tt.fields.subHandlers))
		})
	}
}

func Test_msgRouter_route(t *testing.T) {
	type fields struct {
		subHandlers map[int]Handler
	}
	type args struct {
		msg          *Message
		subscriberID int
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "route",
			fields: fields{
				subHandlers: map[int]Handler{
					1: func(msg *Message) {
						assert.Equal(t, &Message{
							Subject: "test",
							Data:    []byte("test"),
						}, msg)
					},
				},
			},
			args: args{
				msg: &Message{
					Subject: "test",
					Data:    []byte("test"),
				},
				subscriberID: 1,
			},
			wantErr: assert.NoError,
		},
		{
			name: "route with error",
			fields: fields{
				subHandlers: map[int]Handler{},
			},
			args:    args{msg: &Message{}, subscriberID: 1},
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := &msgRouter{
				subHandlers: tt.fields.subHandlers,
			}
			tt.wantErr(t, ps.route(tt.args.msg, tt.args.subscriberID), fmt.Sprintf("route(%v, %v)", tt.args.msg, tt.args.subscriberID))
		})
	}
}

func Test_newMsgRouter(t *testing.T) {
	tests := []struct {
		name string
		want *msgRouter
	}{
		{
			name: "newMsgRouter",
			want: &msgRouter{subHandlers: make(map[int]Handler)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, newMsgRouter(), "newMsgRouter()")
		})
	}
}
