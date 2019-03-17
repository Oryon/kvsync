package encoding

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Oryon/kvsync/kvs"
	"reflect"
)

var ErrFirstSlash = errors.New("Key must start with /")
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

func (state *encodeState) encodeStruct(dir string, v reflect.Value) error {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			// Attribute is not exported
			continue
		}

		tag := f.Tag.Get("kvs")
		key := dir
		if tag == "" {
			key = key + f.Name
		} else if tag[:1] == "/" {
			return fmt.Errorf("tag must not start with /")
		} else {
			key = key + tag
		}

		err := state.encode(key, v.Field(i).Interface())
		if err != nil {
			return err
		}
	}
	return nil
}

func (state *encodeState) encodePtr(dir string, v reflect.Value) error {
	return state.encode(dir, v.Elem().Interface())
}

// Fills the encodeState with keys and values
func (state *encodeState) encode(root string, object interface{}) error {
	// Test if object should be stored recursively or not
	if root[len(root)-1:] != "/" {
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
		return errNotImplemented
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

func Encode(key string, object interface{}) (map[string]string, error) {
	if key[:1] != "/" {
		return nil, ErrFirstSlash
	}

	state := &encodeState{
		kvs: make(map[string]string),
	}
	err := state.encode(key, object)
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
