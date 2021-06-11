package domain

import (
	"fmt"
	"testing"

	"github.com/mateusf777/pubsub/net"
)

func TestJoin(t *testing.T) {

	a := "test a"
	b := "test b"
	c := "test c"
	expected := fmt.Sprintf("%s %s %s", a, b, c)

	r := Join([]byte(a), net.Space, []byte(b), net.Space, []byte(c))

	if string(r) != expected {
		t.Error("error")
	}

	t.Log(string(r))

}
