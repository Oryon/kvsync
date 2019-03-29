package sync

import (
	"context"
	"errors"
	"fmt"
	"github.com/Oryon/kvsync/encoding"
	"github.com/Oryon/kvsync/kvs"
	"reflect"
	"strings"
)

var ErrNoMoreFields = errors.New("No more fields to consume")
var ErrNotAStruct = errors.New("Object is not a structure")
var ErrNotAMap = errors.New("Object is not an array")
var ErrNotAString = errors.New("Object is not a string")
var ErrWrongKeyType = errors.New("Provided key pointer type mismatch")
var ErrNotThisPath = errors.New("The modified object is not on this path")
var ErrNotImplemented = errors.New("This is not implemented")

// SyncEvent is used to notify a change on a watched object
// as well as diving into the changed element of the object.
type SyncEvent struct {
	// The current object in the path
	current_object reflect.Value

	// The list of fields pointing to the modified field from the object.
	fields []interface{}

	// Keep track of potential error
	err error
}

// These callbacks are used to get notified when a synchronized object changed.
type SyncCallback func(eventPath *SyncEvent) error

// Dives into the object
func (se *SyncEvent) Field(name string) *SyncEvent {
	// First dereference pointers
	se = se.derefPointers()
	if se.err != nil {
		return se
	}

	if len(se.fields) == 0 {
		se.err = ErrNoMoreFields
		return se
	}

	// Check if struct
	if se.current_object.Type().Kind() != reflect.Struct {
		se.err = ErrNotAStruct
		return se
	}

	// If not a string, there is a bug in encode functions. So crashing is OK.
	str := se.fields[0].(string)
	if str != name {
		se.err = ErrNotThisPath
		return se
	}

	// Encode cannot be wrong
	se.current_object = se.current_object.FieldByName(str)
	se.fields = se.fields[1:]
	return se
}

// When the change is associated with a an element of a map,
// it might be useful to get the value used as key in this map.
func (se *SyncEvent) Value(key *interface{}) *SyncEvent {
	// First dereference pointers
	se = se.derefPointers()
	if se.err != nil {
		return se
	}

	if len(se.fields) == 0 {
		se.err = ErrNoMoreFields
		return se
	}

	// Check if map
	if se.current_object.Type().Kind() != reflect.Map {
		se.err = ErrNotAMap
		return se
	}

	if key != nil {
		if reflect.TypeOf(*key) != se.current_object.Type().Key() {
			se.err = ErrWrongKeyType
			return se
		}
		*key = se.fields[0]
	}
	se.current_object = se.current_object.MapIndex(reflect.ValueOf(se.fields[0]))
	se.fields = se.fields[1:]
	return se
}

// When the change is associated with a an element of an array,
// this will return the index of the changed element.
func (se *SyncEvent) GetIndex(index *int) *SyncEvent {
	if se.err != nil {
		return se
	}
	// Arrays are not implemented for now
	se.err = ErrNotImplemented
	return se
}

func (se *SyncEvent) Error() error {
	return se.err
}

func (se *SyncEvent) Current() (interface{}, error) {
	if se.err != nil {
		return nil, se.err
	}
	return se.current_object.Interface(), nil
}

func (se *SyncEvent) String() (string, error) {
	if se.err != nil {
		return "", se.err
	}
	str, ok := se.current_object.Interface().(string)
	if !ok {
		return "", ErrNotAString
	}
	return str, nil
}

func (se *SyncEvent) Int() (int, error) {
	if se.err != nil {
		return 0, se.err
	}
	i, ok := se.current_object.Interface().(int)
	if !ok {
		return 0, ErrNotAString
	}
	return i, nil
}

func (se *SyncEvent) derefPointers() *SyncEvent {
	if se == nil {
		return nil
	}

	for se.current_object.Type().Kind() == reflect.Ptr {
		if se.current_object.IsNil() {
			return nil
		}
		se.current_object = se.current_object.Elem()
	}
	return se
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

	// FIXME: Hack for now as delete is not supported
	es := ""
	if e.Value == nil {
		e.Value = &es
	}

	for _, v := range s.objects {
		fields, err := encoding.UpdateKeyObject(v.Object, v.Format, e.Key, *e.Value)
		if err != nil {
			continue
		}
		event := SyncEvent{
			current_object: reflect.ValueOf(v.Object),
			fields:         fields,
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
