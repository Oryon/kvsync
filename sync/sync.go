package sync

import (
	"context"
	"fmt"
	"github.com/Oryon/kvsync/encoding"
	"github.com/Oryon/kvsync/kvs"
	"strings"
)

// SyncEvent is used to notify a change on a watched object
// as well as diving into the changed element of the object.
type SyncEvent struct {

	// The modified object
	Object interface{}

	// The list of fields pointing to the modified field from the object.
	fields []interface{}
}

// These callbacks are used to get notified when a synchronized object changed.
type SyncCallback func(eventPath *SyncEvent) error

// Dives into the object
func (se *SyncEvent) Subobject(path string) *SyncEvent {
	if se == nil {
		return nil
	}
	return nil
}

// When the change is associated with a an element of a map,
// it might be useful to get the value used as key in this map.
func (se *SyncEvent) GetKey(key interface{}) *SyncEvent {
	if se == nil {
		return nil
	}
	return nil
}

// When the change is associated with a an element of an array,
// this will return the index of the changed element.
func (se *SyncEvent) GetIndex(index *int) *SyncEvent {
	if se == nil {
		return nil
	}
	return nil
}

type SyncObject struct {
	Format   string
	Object   interface{}
	Callback SyncCallback
}

type Sync struct {
	Sync     kvs.Sync
	objects  map[int]SyncObject
	next_key int
}

// Waits until the next change from the storage, updates
// the objects that are being synchronized, calls the callback,
// and then returns.
func (s *Sync) Next(c context.Context) error {
	s.initIfNot()

	e, err := s.Sync.Next(c)
	if err != nil {
		return err
	}

	for _, v := range s.objects {
		o, fields, err := encoding.FindByKey(v.Object, v.Format, e.Key)
		if err != nil {
			continue
		}
		event := SyncEvent{
			Object: o,
			fields: fields,
		}
		v.Callback(&event)
	}

	return nil
}

// Start synchronizing a new object, sending a notification when something changes.
func (s *Sync) SyncObject(o SyncObject) error {
	s.initIfNot()

	for _, v := range s.objects {
		if prefixCollision(o.Format, v.Format) {
			return fmt.Errorf("Cannot watch objects in overlaping key spaces.")
		}
	}

	s.objects[s.next_key] = o
	s.next_key++ //FIXME: This will not work after loop.

	//TODO: Register watcher on KVS

	return nil
}

// Start synchronizing a new object, sending a notification when something changes.
func (s *Sync) UnsyncObject(key string) error {
	s.initIfNot()
	for k, v := range s.objects {
		if v.Format == key {
			delete(s.objects, k)
			//TODO: Unregister watcher on KVS
			return nil
		}
	}

	return fmt.Errorf("Key '%s' not found in listeners", key)
}

func (s *Sync) initIfNot() {
	if s.objects == nil {
		s.objects = make(map[int]SyncObject)
	}
}

func prefixCollision(key1, key2 string) bool {
	s1 := strings.Split(key1, "/")
	s2 := strings.Split(key2, "/")
	if len(s1) < len(s2) {
		s1, s2 = s2, s1 //swap works in go
	}

	for i := range s2 {
		if s1[i] != s2[i] {
			return false
		}
	}
	return true
}
