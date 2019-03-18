package encoding

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Oryon/kvsync/kvs"
	"reflect"
	"strings"
)

var ErrFirstSlash = errors.New("Key must start with /")
var ErrWrongFieldType = errors.New("Provided field is of wrong type")
var ErrWrongFieldName = errors.New("Provided field does not exist")
var errNotImplemented = errors.New("Not implemented")
var errUnsupportedType = errors.New("Object type not supported")

type encodeState struct {
	kvs map[string]string
}

func (state *encodeState) encodeJson(key string, object interface{}) error {
	arr, err := json.Marshal(object)
	if err != nil {
		return err
	}

	if v, ok := state.kvs[key]; ok {
		return fmt.Errorf("Key '%s' is already used by value '%s'", key, v)
	}

	state.kvs[key] = string(arr)
	return nil
}

func appendStructField(dir string, f reflect.StructField) (string, error) {
	tag := f.Tag.Get("kvs")
	if tag == "" {
		dir = dir + f.Name
	} else if tag[:1] == "/" {
		return "", fmt.Errorf("tag must not start with /")
	} else {
		dir = dir + tag
	}
	return dir, nil
}

func serializeMapKey(v reflect.Value) (string, error) {
	arr, err := json.Marshal(v.Interface())
	if err != nil {
		return "<ERROR>", err
	}
	return string(arr), nil
}

func (state *encodeState) encodeStruct(dir string, v reflect.Value) error {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			// Attribute is not exported
			continue
		}

		dir, err := appendStructField(dir, f)
		if err != nil {
			return err
		}

		err = state.encode(dir, v.Field(i).Interface())
		if err != nil {
			return err
		}
	}
	return nil
}

// Parses a map key path and returns the string to be set
// before and after the key string
func parseMapKey(dir string) []string {
	split := strings.Split(dir, "{key}")
	if len(split) == 1 {
		split[0] = "/" + split[0]
		split = append(split, "/")
	}
	return split
}

func (state *encodeState) encodeMap(dir string, v reflect.Value) error {
	split := parseMapKey(dir)
	for _, k := range v.MapKeys() {
		key_string, err := serializeMapKey(k)
		if err != nil {
			return err
		}

		key := split[0] + key_string + split[1]

		value := v.MapIndex(k)
		err = state.encode(key, value.Interface())
		if err != nil {
			return err
		}
	}
	return nil
}

func (state *encodeState) encodePtr(dir string, v reflect.Value) error {
	return state.encode(dir, v.Elem().Interface())
}

func storeAsBlob(root string, object interface{}) bool {
	v := reflect.ValueOf(object)
	switch v.Type().Kind() {
	case reflect.Map:
		if strings.Contains(root, "{key}") {
			return false
		}
	case reflect.Slice:
	case reflect.Array:
		if strings.Contains(root, "{index}") {
			return false
		}
	}

	return root[len(root)-1:] != "/"
}

// Fills the encodeState with keys and values
func (state *encodeState) encode(root string, object interface{}) error {
	// Test if object should be stored recursively or not
	if storeAsBlob(root, object) {
		// Encode the object at the designated key
		if e := state.encodeJson(root, object); e != nil {
			return e
		}
		return nil
	}

	// Recursive encoding only supports certain types

	v := reflect.ValueOf(object)
	switch v.Type().Kind() {
	case reflect.Struct:
		return state.encodeStruct(root, v)
	case reflect.Map:
		return state.encodeMap(root, v)
	case reflect.Slice:
		return errNotImplemented
	case reflect.Array:
		return errNotImplemented
	case reflect.Ptr:
		return state.encodePtr(root, v)
	default:
		return fmt.Errorf("Unsupported type %v", v.Type().Kind())
		return errUnsupportedType
	}

	return errNotImplemented
}

func goDownStruct(key string, v reflect.Value, field interface{}) (string, interface{}, error) {
	name, ok := field.(string)
	if !ok {
		return "", nil, ErrWrongFieldType
	}

	f, ok := v.Type().FieldByName(name)
	if !ok {
		return "", nil, ErrWrongFieldName
	}
	key, err := appendStructField(key, f)
	if err != nil {
		return "", nil, err
	}

	return key, v.FieldByIndex(f.Index).Interface(), nil
}

func goDownMap(dir string, v reflect.Value, field interface{}) (string, interface{}, error) {
	key_type := v.Type().Key()
	k := reflect.ValueOf(field)
	if k.Type() != key_type {
		return "", nil, ErrWrongFieldType
	}

	split := parseMapKey(dir)
	key_string, err := serializeMapKey(k)
	if err != nil {
		return "", nil, err
	}
	key := split[0] + key_string + split[1]
	return key, v.MapIndex(k), nil //TODO: MapIndex will fail if object does not exist
}

// Goes down into the object by one step
func goDown(key string, object interface{}, field interface{}) (string, interface{}, error) {
	v := reflect.ValueOf(object)
	switch v.Type().Kind() {
	case reflect.Struct:
		return goDownStruct(key, v, field)
	case reflect.Map:
		return goDownMap(key, v, field)
	case reflect.Slice:
		return "", nil, errNotImplemented
	case reflect.Array:
		return "", nil, errNotImplemented
	case reflect.Ptr:
		return goDown(key, v.Elem().Interface(), field)
	default:
		return "", nil, fmt.Errorf("Unsupported type %v", v.Type().Kind())
	}
}

// Goes directely down an object
func find(key string, object interface{}, fields ...interface{}) (string, interface{}, error) {
	if len(fields) == 0 {
		// This is the end of the journey, buddy.
		return key, object, nil
	}

	if key[len(key)-1:] != "/" {
		// This object is meant to be encoded as JSON at the given position
		return key, object, nil
	}

	key, object, err := goDown(key, object, fields[0])
	if err != nil {
		return key, object, err
	}

	fields = fields[1:]
	return find(key, object, fields...)
}

// Encode part of the object stored at position key.
// The subfield is identified by a list of fields.
// Structure attributes are identified by name (as a string).
// Slice indexes are identified with integers.
// Map keys are identified by given an object of the same type than the map key.
func Encode(key string, object interface{}, fields ...interface{}) (map[string]string, error) {
	if key[:1] != "/" {
		return nil, ErrFirstSlash
	}

	key, object, err := find(key, object, fields...)
	if err != nil {
		return nil, err
	}

	state := &encodeState{
		kvs: make(map[string]string),
	}
	err = state.encode(key, object)
	if err != nil {
		return nil, err
	}
	return state.kvs, nil
}

// Puts an object into the key-value store
func Set(s kvs.Store, c context.Context, key string, object interface{}) error {
	m, err := Encode(key, object)
	if err != nil {
		return err
	}

	for k, v := range m {
		err = s.Set(c, k, v)
		if err != nil {
			return err
		}
	}
	return nil
}
