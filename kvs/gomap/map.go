package gomap

import (
	"context"
	"fmt"
	"github.com/Oryon/kvsync/kvs"
	"sync"
)

type Gomap struct {
	gomap map[string]string
	cond  *sync.Cond

	// State already returned by Next.
	// This is quite memory inefficient, maybe there is room for
	// optimizations.
	synced map[string]string
}

func CreateFromExistingMap(gomap map[string]string) *Gomap {
	m := &Gomap{}
	m.gomap = make(map[string]string)
	m.cond = sync.NewCond(&sync.Mutex{})
	return m
}

func Create() *Gomap {
	return CreateFromExistingMap(make(map[string]string))
}

func (m *Gomap) Set(c context.Context, key string, value string) error {
	m.gomap[key] = value
	return nil
}

func (m *Gomap) Delete(c context.Context, key string) error {
	_, ok := m.gomap[key]
	if !ok {
		return fmt.Errorf("Key '%s' is not in map", key)
	}
	delete(m.gomap, key)
	return nil
}

func (m *Gomap) Next(c context.Context) (*kvs.Update, error) {
	//u = &kvs.Update{}
	if m.synced == nil {
		m.synced = make(map[string]string)
	}

	//TODO

	return nil, fmt.Errorf("Next not implemented")
}
