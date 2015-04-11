package sessions

import (
	"io"
	"sync"

	"github.com/garyburd/redigo/redis"
)

type Store interface {
	io.Closer
	Get(key string) ([]byte, error)
	Set(key string, val []byte) error
	Del(key string) error
}

type MemoryStore struct {
	sync.RWMutex
	values map[string][]byte
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{values: make(map[string][]byte)}
}

var DefaultMemoryStore = NewMemoryStore()

func (ms *MemoryStore) Close() error {
	ms.values = nil
	return nil
}

func (ms *MemoryStore) Get(key string) ([]byte, error) {
	ms.RLock()
	defer ms.RUnlock()
	if v, ok := ms.values[key]; ok {
		return v, nil
	}
	return []byte(nil), nil
}

func (ms *MemoryStore) Set(key string, val []byte) error {
	ms.Lock()
	defer ms.Unlock()
	ms.values[key] = val
	return nil
}

func (ms *MemoryStore) Del(key string) error {
	ms.Lock()
	defer ms.Unlock()
	delete(ms.values, key)
	return nil
}

type RedisStore struct {
	conn redis.Conn
}

func NewRedisStore(network, address, password string) (*RedisStore, error) {
	c, err := redis.Dial(network, address)
	if err != nil {
		return nil, err
	}
	if password != "" {
		if _, err := c.Do("AUTH", password); err != nil {
			c.Close()
			return nil, err
		}
	}
	return &RedisStore{conn: c}, nil
}

func (rs *RedisStore) Close() error {
	rs.conn.Close()
	return nil
}

func (rs *RedisStore) Get(key string) ([]byte, error) {
	b, err := redis.Bytes(rs.conn.Do("GET", key))
	if err != nil {
		if err == redis.ErrNil {
			return []byte(nil), nil
		}
		return b, err
	}
	return b, nil
}

func (rs *RedisStore) Set(key string, val []byte) error {
	_, err := rs.conn.Do("SET", key, val)
	return err
}

func (rs *RedisStore) Del(key string) error {
	_, err := rs.conn.Do("DEL", key)
	return err
}
