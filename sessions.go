package sessions

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/gob"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/codegangsta/negroni"
	"github.com/mix3/fever"
	"golang.org/x/net/context"
)

var defaultFlashKey = "_flash"
var defaultCookieName = "sessions"

type sessionValues map[string]interface{}

func init() {
	gob.Register(sessionValues{})
	gob.Register([]interface{}{})
}

var contextSessionKey = struct{}{}

type session struct {
	ss *Sessions

	sid      string
	values   sessionValues
	isNew    bool
	changeId bool
	expire   bool
	noStore  bool
	written  bool

	// cookie setting
	Path     string
	Domain   string
	Expires  time.Time
	Secure   bool
	HttpOnly bool
}

func (s *session) Get(key string) interface{} {
	if v, ok := s.values[key]; ok {
		return v
	}
	return nil
}

func (s *session) Exists(key string) bool {
	_, ok := s.values[key]
	return ok
}

func (s *session) Set(key string, val interface{}) {
	s.values[key] = val
	s.written = true
}

func (s *session) Del(key string) {
	delete(s.values, key)
	s.written = true
}

func (s *session) AddFlash(val interface{}, vars ...string) {
	key := defaultFlashKey
	if 0 < len(vars) {
		key = vars[0]
	}
	var flashes []interface{}
	if v, ok := s.values[key]; ok {
		flashes = v.([]interface{})
	}
	s.values[key] = append(flashes, val)
	s.written = true
}

func (s *session) Flashes(vars ...string) []interface{} {
	key := defaultFlashKey
	if 0 < len(vars) {
		key = vars[0]
	}
	var flashes []interface{}
	if v, ok := s.values[key]; ok {
		delete(s.values, key)
		flashes = v.([]interface{})
		s.written = true
	}
	return flashes
}

func (s *session) NoStore(v ...bool) bool {
	if 0 < len(v) {
		s.noStore = v[0]
	}
	return s.noStore
}

func (s *session) ChangeId(v ...bool) bool {
	if 0 < len(v) {
		s.changeId = v[0]
	}
	return s.changeId
}

func (s *session) Expire(v ...bool) bool {
	if 0 < len(v) {
		s.expire = v[0]
	}
	return s.expire
}

func (s *session) HasKey() bool {
	if 0 < len(s.values) {
		return true
	}
	return false
}

func (s *session) sidRegenerate() {
	s.sid = s.ss.SidGenerator()
}

func (s *session) needStore() bool {
	if s.noStore {
		return false
	}

	if (s.isNew && !s.ss.NoKeepEmpty && !s.HasKey()) ||
		s.written ||
		s.expire ||
		s.changeId {
		return true
	}

	return false
}

func (s *session) store() error {
	if !s.needStore() {
		return nil
	}

	if s.expire {
		return s.ss.Store.Del(s.sid)
	}

	if s.changeId {
		err := s.ss.Store.Del(s.sid)
		if err != nil {
			return err
		}
		s.sidRegenerate()
	}

	b, err := s.ss.Encode(s.values)
	if err != nil {
		return err
	}

	err = s.ss.Store.Set(s.sid, b)
	if err != nil {
		return err
	}

	return nil
}

func (s *session) needSetCookie() bool {
	if (s.isNew && !s.ss.NoKeepEmpty && !s.HasKey()) ||
		(s.isNew && s.written) ||
		s.expire ||
		s.changeId {
		return true
	}
	return false
}

func (s *session) setCookie(w http.ResponseWriter) {
	if !s.needSetCookie() {
		return
	}

	cookie := &http.Cookie{
		Name:     s.ss.CookieName,
		Value:    s.sid,
		Path:     s.Path,
		Domain:   s.Domain,
		Expires:  s.Expires,
		Secure:   s.Secure,
		HttpOnly: s.HttpOnly,
	}
	if s.expire {
		cookie.Expires = time.Now()
	}

	http.SetCookie(w, cookie)
}

func sidGenerator() string {
	h := sha1.New()
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return ""
	}
	h.Write(b)
	return fmt.Sprintf("%x", h.Sum(nil))
}

var re = regexp.MustCompile(`\A[0-9a-f]{40}\z`)

func sidValidator(sid string) bool {
	return re.MatchString(sid)
}

type Sessions struct {
	Store        Store
	CookieName   string
	NoKeepEmpty  bool
	Path         string
	Domain       string
	Expires      time.Time
	Secure       bool
	HttpOnly     bool
	SidGenerator func() string
	SidValidator func(sid string) bool
}

func New(store Store, vars ...string) *Sessions {
	cookieName := defaultCookieName
	if 0 < len(vars) {
		cookieName = vars[0]
	}
	return &Sessions{
		Store:      store,
		CookieName: cookieName,
	}
}

func (ss *Sessions) getSessionValues(r *http.Request) (string, sessionValues, error) {
	cookie, _ := r.Cookie(ss.CookieName)
	if cookie == nil {
		return "", sessionValues{}, nil
	}
	sid := cookie.Value
	if !ss.SidValidator(sid) {
		return "", sessionValues{}, nil
	}
	encodedValue, err := ss.Store.Get(sid)
	if err != nil {
		return "", sessionValues{}, err
	}
	if len(encodedValue) == 0 {
		return "", sessionValues{}, nil
	}
	decodedValue, err := ss.Decode(encodedValue)
	if err != nil {
		return "", sessionValues{}, err
	}
	return sid, decodedValue, nil
}

func (ss *Sessions) Middleware(h fever.Handler) fever.Handler {
	if ss.SidGenerator == nil {
		ss.SidGenerator = sidGenerator
	}
	if ss.SidValidator == nil {
		ss.SidValidator = sidValidator
	}
	return fever.HandlerFunc(func(c context.Context, w http.ResponseWriter, r *http.Request) {
		nw, ok := w.(negroni.ResponseWriter)
		if !ok {
			nw = negroni.NewResponseWriter(w)
		}
		sid, values, err := ss.getSessionValues(r)
		if err != nil {
			panic(err)
		}
		isNew := false
		if sid == "" {
			sid = ss.SidGenerator()
			isNew = true
		}
		s := &session{
			ss:       ss,
			sid:      sid,
			values:   values,
			isNew:    isNew,
			Path:     ss.Path,
			Domain:   ss.Domain,
			Expires:  ss.Expires,
			Secure:   ss.Secure,
			HttpOnly: ss.HttpOnly,
		}
		c = context.WithValue(c, contextSessionKey, s)
		nw.Before(func(w negroni.ResponseWriter) {
			err := ss.finalize(w, s)
			if err != nil {
				panic(err)
			}
		})
		h.ServeHTTP(c, nw, r)
	})
}

func (ss *Sessions) finalize(w http.ResponseWriter, s *session) error {
	err := s.store()
	if err != nil {
		return err
	}
	s.setCookie(w)
	return nil
}

func (ss *Sessions) Encode(val sessionValues) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(&val)
	if err != nil {
		return []byte{}, err
	}
	return buf.Bytes(), nil
}

func (ss *Sessions) Decode(b []byte) (sessionValues, error) {
	buf := bytes.NewBuffer(b)
	dec := gob.NewDecoder(buf)
	var val sessionValues
	err := dec.Decode(&val)
	if err != nil {
		return sessionValues{}, err
	}
	return val, nil
}

func Session(c context.Context) *session {
	return c.Value(contextSessionKey).(*session)
}
