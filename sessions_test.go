package sessions_test

import (
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"regexp"
	"testing"

	"golang.org/x/net/context"

	"github.com/mix3/fever-sessions"
	"github.com/mix3/fever/mux"
	"github.com/stretchr/testify/assert"
)

func init() {
	gob.Register(struct{}{})
}

type client struct {
	client *http.Client
}

func newClient(t *testing.T) *client {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &client{&http.Client{Jar: jar}}
}

func (c *client) Get(t *testing.T, url, cookieName string) (*http.Response, string, http.Header) {
	res, err := c.client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res, string(body), res.Header
}

func TestBasic(t *testing.T) {
	store := sessions.NewMemoryStore()
	defer func() {
		err := store.Close()
		if err != nil {
			t.Fatal(err)
		}
	}()
	ss := sessions.New(store, "myapp_session")
	m := mux.New()
	m.Use(ss.Middleware)
	m.Get("/").ThenFunc(func(c context.Context, w http.ResponseWriter, r *http.Request) {
		s := sessions.Session(c)
		if s.Exists("username") {
			fmt.Fprintf(w, "TOP: Hello %s", s.Get("username").(string))
		} else {
			fmt.Fprintf(w, "TOP")
		}
	})
	m.Get("/counter").ThenFunc(func(c context.Context, w http.ResponseWriter, r *http.Request) {
		s := sessions.Session(c)
		v := 1
		if s.Exists("counter") {
			v = s.Get("counter").(int) + 1
		}
		s.Set("counter", v)
		fmt.Fprintf(w, "counter=>%d", v)
	})
	m.Get("/login").ThenFunc(func(c context.Context, w http.ResponseWriter, r *http.Request) {
		s := sessions.Session(c)
		s.Set("username", "foo")
		fmt.Fprintf(w, "LOGIN")
	})
	m.Get("/logout").ThenFunc(func(c context.Context, w http.ResponseWriter, r *http.Request) {
		s := sessions.Session(c)
		if s.Exists("username") {
			s.Expire(true)
		}
		fmt.Fprintf(w, "LOGOUT")
	})
	ts := httptest.NewServer(m)
	defer ts.Close()
	c := newClient(t)
	var sid string
	re := regexp.MustCompile("myapp_session=([a-f0-9]{40})")
	{
		_, body, header := c.Get(t, ts.URL, "myapp_session")
		assert.Equal(t, "TOP", body)
		if ok := assert.Len(t, header["Set-Cookie"], 1); ok {
			assert.Regexp(t, re, header["Set-Cookie"][0])
			sid = re.FindStringSubmatch(header["Set-Cookie"][0])[1]
		}
	}
	{
		_, body, header := c.Get(t, ts.URL+"/login", "myapp_session")
		assert.Equal(t, "LOGIN", body)
		assert.Len(t, header["Set-Cookie"], 0)
	}
	{
		_, body, header := c.Get(t, ts.URL, "myapp_session")
		assert.Equal(t, "TOP: Hello foo", body)
		assert.Len(t, header["Set-Cookie"], 0)
	}
	{
		_, body, header := c.Get(t, ts.URL+"/counter", "myapp_session")
		assert.Equal(t, "counter=>1", body)
		assert.Len(t, header["Set-Cookie"], 0)
	}
	{
		_, body, header := c.Get(t, ts.URL+"/counter", "myapp_session")
		assert.Equal(t, "counter=>2", body)
		assert.Len(t, header["Set-Cookie"], 0)
	}
	{
		_, body, header := c.Get(t, ts.URL+"/logout", "myapp_session")
		assert.Equal(t, "LOGOUT", body)
		if ok := assert.Len(t, header["Set-Cookie"], 1); ok {
			assert.Regexp(t, re, header["Set-Cookie"][0])
			assert.Equal(t, sid, re.FindStringSubmatch(header["Set-Cookie"][0])[1])
		}
	}
	{
		_, body, header := c.Get(t, ts.URL, "myapp_session")
		assert.Equal(t, "TOP", body)
		if ok := assert.Len(t, header["Set-Cookie"], 1); ok {
			assert.Regexp(t, re, header["Set-Cookie"][0])
			assert.NotEqual(t, sid, re.FindStringSubmatch(header["Set-Cookie"][0])[1])
		}
	}
	{
		_, body, header := c.Get(t, ts.URL+"/counter", "myapp_session")
		assert.Equal(t, "counter=>1", body)
		assert.Len(t, header["Set-Cookie"], 0)
	}
}

func TestEmpty(t *testing.T) {
	store := sessions.NewMemoryStore()
	defer func() {
		err := store.Close()
		if err != nil {
			t.Fatal(err)
		}
	}()
	ss := sessions.New(store, "myapp_session")
	ss.NoKeepEmpty = true
	m := mux.New()
	m.Use(ss.Middleware)
	m.Get("/").ThenFunc(func(c context.Context, w http.ResponseWriter, r *http.Request) {
		s := sessions.Session(c)
		if s.Exists("username") {
			fmt.Fprintf(w, "TOP: Hello %s", s.Get("username").(string))
		} else {
			fmt.Fprintf(w, "TOP")
		}
	})
	m.Get("/counter").ThenFunc(func(c context.Context, w http.ResponseWriter, r *http.Request) {
		s := sessions.Session(c)
		v := 1
		if s.Exists("counter") {
			v = s.Get("counter").(int) + 1
		}
		s.Set("counter", v)
		fmt.Fprintf(w, "counter=>%d", v)
	})
	m.Get("/login").ThenFunc(func(c context.Context, w http.ResponseWriter, r *http.Request) {
		s := sessions.Session(c)
		s.Set("username", "foo")
		fmt.Fprintf(w, "LOGIN")
	})
	m.Get("/logout").ThenFunc(func(c context.Context, w http.ResponseWriter, r *http.Request) {
		s := sessions.Session(c)
		if s.Exists("username") {
			s.Expire(true)
		}
		fmt.Fprintf(w, "LOGOUT")
	})
	ts := httptest.NewServer(m)
	defer ts.Close()
	c := newClient(t)
	var sid string
	re := regexp.MustCompile("myapp_session=([a-f0-9]{40})")
	{
		_, body, header := c.Get(t, ts.URL, "myapp_session")
		assert.Equal(t, "TOP", body)
		assert.Len(t, header["Set-Cookie"], 0)
	}
	{
		_, body, header := c.Get(t, ts.URL+"/login", "myapp_session")
		assert.Equal(t, "LOGIN", body)
		if ok := assert.Len(t, header["Set-Cookie"], 1); ok {
			assert.Regexp(t, re, header["Set-Cookie"][0])
			sid = re.FindStringSubmatch(header["Set-Cookie"][0])[1]
		}
	}
	{
		_, body, header := c.Get(t, ts.URL, "myapp_session")
		assert.Equal(t, "TOP: Hello foo", body)
		assert.Len(t, header["Set-Cookie"], 0)
	}
	{
		_, body, header := c.Get(t, ts.URL+"/counter", "myapp_session")
		assert.Equal(t, "counter=>1", body)
		assert.Len(t, header["Set-Cookie"], 0)
	}
	{
		_, body, header := c.Get(t, ts.URL+"/counter", "myapp_session")
		assert.Equal(t, "counter=>2", body)
		assert.Len(t, header["Set-Cookie"], 0)
	}
	{
		_, body, header := c.Get(t, ts.URL+"/logout", "myapp_session")
		assert.Equal(t, "LOGOUT", body)
		if ok := assert.Len(t, header["Set-Cookie"], 1); ok {
			assert.Regexp(t, re, header["Set-Cookie"][0])
			assert.Equal(t, sid, re.FindStringSubmatch(header["Set-Cookie"][0])[1])
		}
	}
	{
		_, body, header := c.Get(t, ts.URL, "myapp_session")
		assert.Equal(t, "TOP", body)
		assert.Len(t, header["Set-Cookie"], 0)
	}
	{
		_, body, header := c.Get(t, ts.URL+"/counter", "myapp_session")
		assert.Equal(t, "counter=>1", body)
		if ok := assert.Len(t, header["Set-Cookie"], 1); ok {
			assert.Regexp(t, re, header["Set-Cookie"][0])
			assert.NotEqual(t, sid, re.FindStringSubmatch(header["Set-Cookie"][0])[1])
		}
	}
}

func TestSession(t *testing.T) {
	store := sessions.NewMemoryStore()
	defer func() {
		err := store.Close()
		if err != nil {
			t.Fatal(err)
		}
	}()
	ss1 := sessions.New(store, "myapp_session")
	ss2 := sessions.New(store, "myapp_session")
	m := mux.New()
	m.Get("/hoge").Use(ss1.Middleware).ThenFunc(func(c context.Context, w http.ResponseWriter, r *http.Request) {
		s := sessions.Session(c)
		v := 1
		if ok := s.Exists("counter"); ok {
			v = s.Get("counter").(int) + 1
		}
		s.Set("counter", v)
		fmt.Fprintf(w, "counter=>%d", v)
	})
	m.Get("/fuga").Use(ss2.Middleware).ThenFunc(func(c context.Context, w http.ResponseWriter, r *http.Request) {
		s := sessions.Session(c)
		v := 1
		if ok := s.Exists("counter"); ok {
			v = s.Get("counter").(int) + 1
		}
		s.Set("counter", v)
		fmt.Fprintf(w, "counter=>%d", v)
	})
	ts := httptest.NewServer(m)
	defer ts.Close()
	c := newClient(t)
	re := regexp.MustCompile("myapp_session=([a-f0-9]{40})")
	{
		_, body, header := c.Get(t, ts.URL+"/hoge", "myapp_session")
		assert.Equal(t, "counter=>1", body)
		if ok := assert.Len(t, header["Set-Cookie"], 1); ok {
			assert.Regexp(t, re, header["Set-Cookie"][0])
		}
	}
	{
		_, body, header := c.Get(t, ts.URL+"/hoge", "myapp_session")
		assert.Equal(t, "counter=>2", body)
		assert.Len(t, header["Set-Cookie"], 0)
	}
	{
		_, body, header := c.Get(t, ts.URL+"/fuga", "myapp_session")
		assert.Equal(t, "counter=>3", body)
		assert.Len(t, header["Set-Cookie"], 0)
	}
	{
		_, body, header := c.Get(t, ts.URL+"/hoge", "myapp_session")
		assert.Equal(t, "counter=>4", body)
		assert.Len(t, header["Set-Cookie"], 0)
	}
}

func TestFlash(t *testing.T) {
	store := sessions.NewMemoryStore()
	defer func() {
		err := store.Close()
		if err != nil {
			t.Fatal(err)
		}
	}()
	ss := sessions.New(store, "myapp_session")
	m := mux.New()
	m.Use(ss.Middleware)
	m.Get("/").ThenFunc(func(c context.Context, w http.ResponseWriter, r *http.Request) {
		s := sessions.Session(c)
		if flashes := s.Flashes(); 0 < len(flashes) {
			fmt.Fprintf(w, "Flashes %v %v", flashes[0], flashes[1])
		} else {
			s.AddFlash("hoge")
			s.AddFlash("fuga")
			fmt.Fprintf(w, "AddFlash")
		}
	})
	ts := httptest.NewServer(m)
	defer ts.Close()
	c := newClient(t)
	re := regexp.MustCompile("myapp_session=([a-f0-9]{40})")
	{
		_, body, header := c.Get(t, ts.URL, "myapp_session")
		assert.Equal(t, "AddFlash", body)
		if ok := assert.Len(t, header["Set-Cookie"], 1); ok {
			assert.Regexp(t, re, header["Set-Cookie"][0])
		}
	}
	{
		_, body, header := c.Get(t, ts.URL, "myapp_session")
		assert.Equal(t, "Flashes hoge fuga", body)
		assert.Len(t, header["Set-Cookie"], 0)
	}
	{
		_, body, header := c.Get(t, ts.URL, "myapp_session")
		assert.Equal(t, "AddFlash", body)
		assert.Len(t, header["Set-Cookie"], 0)
	}
	{
		_, body, header := c.Get(t, ts.URL, "myapp_session")
		assert.Equal(t, "Flashes hoge fuga", body)
		assert.Len(t, header["Set-Cookie"], 0)
	}
}

func TestChangeId(t *testing.T) {
	store := sessions.NewMemoryStore()
	defer func() {
		err := store.Close()
		if err != nil {
			t.Fatal(err)
		}
	}()
	ss := sessions.New(store, "myapp_session")
	m := mux.New()
	m.Use(ss.Middleware)
	m.Get("/").ThenFunc(func(c context.Context, w http.ResponseWriter, r *http.Request) {
		s := sessions.Session(c)
		if ok := s.Exists("username"); ok {
			fmt.Fprintf(w, "TOP Hello: %s", s.Get("username").(string))
		} else {
			fmt.Fprintf(w, "TOP")
		}
	})
	m.Get("/login").ThenFunc(func(c context.Context, w http.ResponseWriter, r *http.Request) {
		s := sessions.Session(c)
		if ok := s.Exists("username"); ok {
			http.Redirect(w, r, "/", http.StatusFound)
		} else {
			s.ChangeId(true)
			s.Set("username", "foo")
			fmt.Fprintf(w, "LOGIN")
		}
	})
	m.Get("/logout").ThenFunc(func(c context.Context, w http.ResponseWriter, r *http.Request) {
		s := sessions.Session(c)
		s.Expire(true)
		fmt.Fprintf(w, "LOGOUT")
	})
	ts := httptest.NewServer(m)
	defer ts.Close()
	c := newClient(t)
	var sid string
	re := regexp.MustCompile("myapp_session=([a-f0-9]{40})")
	{
		_, body, header := c.Get(t, ts.URL, "myapp_session")
		assert.Equal(t, "TOP", body)
		if ok := assert.Len(t, header["Set-Cookie"], 1); ok {
			assert.Regexp(t, re, header["Set-Cookie"][0])
			sid = re.FindStringSubmatch(header["Set-Cookie"][0])[1]
		}
	}
	{
		_, body, header := c.Get(t, ts.URL+"/login", "myapp_session")
		assert.Equal(t, "LOGIN", body)
		if ok := assert.Len(t, header["Set-Cookie"], 1); ok {
			assert.Regexp(t, re, header["Set-Cookie"][0])
			assert.NotEqual(t, sid, re.FindStringSubmatch(header["Set-Cookie"][0])[1])
			sid = re.FindStringSubmatch(header["Set-Cookie"][0])[1]
		}
	}
	{
		_, body, header := c.Get(t, ts.URL, "myapp_session")
		assert.Equal(t, "TOP Hello: foo", body)
		assert.Len(t, header["Set-Cookie"], 0)
	}
	{
		_, body, header := c.Get(t, ts.URL+"/login", "myapp_session")
		assert.Equal(t, "TOP Hello: foo", body)
		assert.Len(t, header["Set-Cookie"], 0)
	}
	{
		_, body, header := c.Get(t, ts.URL+"/logout", "myapp_session")
		assert.Equal(t, "LOGOUT", body)
		if ok := assert.Len(t, header["Set-Cookie"], 1); ok {
			assert.Regexp(t, re, header["Set-Cookie"][0])
			assert.Equal(t, sid, re.FindStringSubmatch(header["Set-Cookie"][0])[1])
		}
	}
	{
		_, body, header := c.Get(t, ts.URL+"/login", "myapp_session")
		assert.Equal(t, "LOGIN", body)
		if ok := assert.Len(t, header["Set-Cookie"], 1); ok {
			assert.Regexp(t, re, header["Set-Cookie"][0])
			assert.NotEqual(t, sid, re.FindStringSubmatch(header["Set-Cookie"][0])[1])
		}
	}
}
