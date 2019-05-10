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

// Functions to synchronize objects with a kvs.
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
var ErrNotAnInt = errors.New("Object is not an integer")
var ErrNotABool = errors.New("Object is not a bool")
var ErrKeyMustPtr = errors.New("Provided key must be a pointer")
var ErrWrongKeyType = errors.New("Provided key pointer type mismatch")
var ErrNotThisPath = errors.New("The modified object is not on this path")
var ErrNotImplemented = errors.New("This is not implemented")
var ErrNilPointer = errors.New("Reached nil pointer")
var ErrIsDelete = errors.New("Object is being deleted")

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
func (se SyncEvent) Field(name string) SyncEvent {
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
func (se SyncEvent) Value(key interface{}) SyncEvent {
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
		k := reflect.ValueOf(key)
		if k.Type().Kind() != reflect.Ptr {
			se.err = ErrKeyMustPtr
			return se
		}
		if k.Type().Elem() != se.current_object.Type().Key() {
			se.err = ErrWrongKeyType
			return se
		}
		k.Elem().Set(reflect.ValueOf(se.fields[0]))
	}
	se.current_object = se.current_object.MapIndex(reflect.ValueOf(se.fields[0]))
	se.fields = se.fields[1:]
	return se
}

// When the change is associated with a an element of an array,
// this will return the index of the changed element.
func (se SyncEvent) GetIndex(index *int) SyncEvent {
	if se.err != nil {
		return se
	}
	// Arrays are not implemented for now
	se.err = ErrNotImplemented
	return se
}

// Returns whether the currently considered object was actually deleted.
func (se SyncEvent) IsDeleted(deleted *bool) SyncEvent {
	if !se.current_object.IsValid() && len(se.fields) == 0 {
		*deleted = true
		return se
	}
	return se
}

func (se SyncEvent) Error() error {
	return se.err
}

func (se SyncEvent) Current() (interface{}, error) {
	if se.err != nil {
		return nil, se.err
	}
	if !se.current_object.IsValid() {
		se.err = ErrIsDelete
		return nil, se.err
	}

	return se.current_object.Interface(), nil
}

func (se SyncEvent) String() (string, error) {
	if se.err != nil {
		return "", se.err
	}
	str, ok := se.current_object.Interface().(string)
	if !ok {
		return "", ErrNotAString
	}
	return str, nil
}

func (se SyncEvent) Int() (int, error) {
	if se.err != nil {
		return 0, se.err
	}
	kind := se.current_object.Kind()
	ok := kind == reflect.Int || kind == reflect.Int8 || kind == reflect.Int16 || kind == reflect.Int32 || kind == reflect.Int64
	if ok {
		i := int(se.current_object.Int())
		return i, nil
	} else {
		return 0, ErrNotAnInt
	}
}

func (se SyncEvent) Bool() (bool, error) {
	if se.err != nil {
		return false, se.err
	}
	b, ok := se.current_object.Interface().(bool)
	if !ok {
		return false, ErrNotABool
	}
	return b, nil
}

func (se SyncEvent) derefPointers() SyncEvent {
	if !se.current_object.IsValid() {
		se.err = ErrIsDelete
		return se
	}

	for se.current_object.Type().Kind() == reflect.Ptr {
		if se.current_object.IsNil() {
			se.err = ErrNilPointer
			return se
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

	if e.Value == nil {
		// First try to remove as map object
		for _, v := range s.objects {
			k := e.Key
			if e.Key[len(e.Key)-1] == '/' {
				k = e.Key[:len(e.Key)-1]
			}
			fields, err := encoding.DeleteKeyObject(v.Object, v.Format, k)
			if err != nil {
				if err == encoding.ErrFindObjectNotFound {
					return err
				}
				continue
			}
			event := SyncEvent{
				current_object: reflect.ValueOf(v.Object),
				fields:         fields,
			}
			v.Callback(&event)
			return nil
		}
	}

	// This is a hack since some objects cannot be deleted properly for now
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
