package encoding

import (
	"context"
	"github.com/Oryon/kvsync/kvs"
)

// SyncEvent is used to notify a change on a watched object
// as well as diving into the changed element of the object.
type SyncEvent struct {
	Object interface{}
	Key    string
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
	Key      string
	Object   interface{}
	Callback SyncCallback
}

type Sync struct {
	Sync    kvs.Sync
	objects map[string]SyncObject
}

// Waits until the next change from the storage, updates
// the objects that are being synchronized, calls the callback,
// and then returns.
func (s *Sync) Next(c context.Context) error {
	return nil
}

// Start synchronizing a new object, sending a notification when something changes.
func (s *Sync) AddObject(o SyncObject) error {
	return nil
}
