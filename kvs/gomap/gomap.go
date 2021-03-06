// Copyright (c) 2019 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Generic kvs interface implementation using a golang map.
package gomap

import (
	"context"
	"fmt"
	"github.com/Oryon/kvsync/kvs"
	"strings"
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

func (m *Gomap) Lock() {
	m.mutex.Lock()
}

func (m *Gomap) Unlock() {
	m.mutex.Unlock()
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

	found := false

	if key[len(key)-1] == '/' {
		var us []kvs.Update

		for k, v := range m.gomap {
			if strings.HasPrefix(k, key) {
				s := string(v)
				u := kvs.Update{
					Key:      k,
					Value:    nil,
					Previous: &s,
				}
				us = append(us, u)
				found = true
			}
		}
		if !found {
			return fmt.Errorf("Key '%s' is not in map", key)
		}
		u := kvs.Update{
			Key:      key,
			Value:    nil,
			Previous: &key,
		}
		m.queue = append(m.queue, u)

		for _, u := range us {
			delete(m.gomap, u.Key)
		}

	} else {
		s, ok := m.gomap[key]
		if !ok {
			return fmt.Errorf("Key '%s' is not in map", key)
		}
		u := kvs.Update{
			Key:      key,
			Value:    nil,
			Previous: &s,
		}
		delete(m.gomap, u.Key)
		m.queue = append(m.queue, u)
	}

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

func (m *Gomap) Get(c context.Context, key string) (string, error) {
	v, ok := m.gomap[key]
	if !ok {
		return "", kvs.ErrNoSuchKey
	}
	return v, nil
}

func (m *Gomap) GetBackingMap() map[string]string {
	return m.gomap
}
