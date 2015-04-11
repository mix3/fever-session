package sessions_test

import (
	"testing"

	"github.com/mix3/fever-sessions"
	"github.com/soh335/go-test-redisserver"
	"github.com/stretchr/testify/assert"
)

func TestMemoryStore(t *testing.T) {
	ms := sessions.NewMemoryStore()
	ms.Set("hoge", []byte("fuga"))
	{
		v, _ := ms.Get("hoge")
		assert.Equal(t, []byte("fuga"), v)
	}
	ms.Del("hoge")
	{
		v, _ := ms.Get("hoge")
		assert.Equal(t, []byte(nil), v)
	}
}

func TestRedisStore(t *testing.T) {
	s, err := redistest.NewServer(true, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	rs, err := sessions.NewRedisStore("unix", s.Config["unixsocket"], "")
	if err != nil {
		t.Fatal(err)
	}
	rs.Set("hoge", []byte("fuga"))
	{
		v, _ := rs.Get("hoge")
		assert.Equal(t, []byte("fuga"), v)
	}
	rs.Del("hoge")
	{
		v, _ := rs.Get("hoge")
		assert.Equal(t, []byte(nil), v)
	}
}
