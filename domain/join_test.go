package domain

import (
	"fmt"
	"testing"
)

func TestJoin(t *testing.T) {

	a := "test a"
	b := "test b"
	c := "test c"
	expected := fmt.Sprintf("%s %s %s", a, b, c)

	r := Join([]byte(a), Space, []byte(b), Space, []byte(c))

	if string(r) != expected {
		t.Error("error")
	}

	t.Log(string(r))

}
