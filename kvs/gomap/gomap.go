package gomap

import (
	"context"
	"fmt"
	"github.com/Oryon/kvsync/kvs"
	"sync"
)

type Gomap struct {
	gomap   map[string]string
	mutex   sync.Mutex
	channel chan int
	queue   []kvs.Update
}

func CreateFromExistingMap(gomap map[string]string) *Gomap {
	m := &Gomap{}
	m.gomap = make(map[string]string)
	m.mutex = sync.Mutex{}
	m.channel = make(chan int, 1)
	for k, v := range gomap {
		u := kvs.Update{
			Key:      k,
			Value:    &v,
			Previous: nil,
		}
		m.queue = append(m.queue, u)
	}
	return m
}

func Create() *Gomap {
	return CreateFromExistingMap(make(map[string]string))
}

func (m *Gomap) Set(c context.Context, key string, value string) error {
	u := kvs.Update{
		Key:      key,
		Value:    &value,
		Previous: nil,
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()
	s, ok := m.gomap[key]
	if ok {
		u.Previous = &s
	}

	m.gomap[key] = value

	m.queue = append(m.queue, u)

	select {
	case m.channel <- 2: // Put 2 in the channel unless it is full
	default:
	}
	return nil
}

func (m *Gomap) Delete(c context.Context, key string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	s, ok := m.gomap[key]
	if !ok {
		return fmt.Errorf("Key '%s' is not in map", key)
	}

	u := kvs.Update{
		Key:      key,
		Value:    nil,
		Previous: &s,
	}

	delete(m.gomap, key)

	m.queue = append(m.queue, u)
	return nil
}

func (m *Gomap) Next(c context.Context) (*kvs.Update, error) {

	for {
		m.mutex.Lock()
		if len(m.queue) != 0 {
			u := m.queue[0]
			m.queue = m.queue[1:]
			m.mutex.Unlock()
			return &u, nil
		}
		m.mutex.Unlock()

		// Wait until notification or context is done
		select {
		case <-m.channel:
		case <-c.Done():
			return nil, c.Err()
		}
	}

	return nil, fmt.Errorf("Next not implemented")
}

func (m *Gomap) Get(c context.Context, key string) (string, bool) {
	v, ok := m.gomap[key]
	return v, ok
}
