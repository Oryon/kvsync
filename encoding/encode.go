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

// Mapping funtions between objects and key-value pairs.
package encoding

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

var ErrFirstSlash = errors.New("Key must start with /")
var ErrWrongFieldType = errors.New("Provided field is of wrong type")
var ErrWrongFieldName = errors.New("Provided field does not exist")
var ErrNotImplemented = errors.New("Not implemented")
var ErrUnsupportedType = errors.New("Object type not supported")
var ErrFindPathPastObject = errors.New("Provided path goes past an encoded object")
var ErrFindObjectNotFound = errors.New("Object was not found")
var ErrFindKeyNotFound = errors.New("Key was not found in map")
var ErrFindKeyInvalid = errors.New("Invalid key for this object")
var ErrFindPathNotFound = errors.New("Object not found at specified path")
var ErrFindSetNoExists = errors.New("Cannot set non existent object")
var ErrFindSetWrongType = errors.New("The provided object is of wrong type")
var ErrScalarType = errors.New("Cannot recursively store scalar type")
var ErrTagFirstSlash = errors.New("Structure field tag cannot start with /")
var ErrFindKeyWrongType = errors.New("Provided map key field is of wrong type")
var ErrNotMapIndex = errors.New("Specified object is not a map index")

// State storing keys and values before they get stored for one or multiple objects
type encodeState struct {
	kvs map[string]string
}

// State representing an object as well as its path in some parent opbjects.
// This path is not absolute, and this struct does not keep memory of
// the path root.
type objectPath struct {

	// A value pointing to the current object.
	// It may be non-addressable or contain a Zero value if the object does not exist currently.
	value reflect.Value

	// The type of the current object, used to go through the structure hierarchy even when
	// no value is found.
	vtype reflect.Type

	// The key path used to reach the current object, but not referring to the
	// object path itself (see 'format').
	keypath []string

	// The set of specific fields (attributes names, keys and indexes) used
	// to arrive to this object.
	fields []interface{}

	// The key format to be used to encode children of this object
	// (attributes, map values, array values, etc...).
	// e.g. [], This object is stored as a JSON blob.
	// e.g. [""], Attributes of the struct are stored directly following the struct path.
	// e.g. [ <prefix>... , ""], <prefix> will prefix all attribute paths.
	// e.g. [ <prefix>... , "{key}", <suffix>... ], map values are stored at "prefix/<key>" using <suffix> format.
	// e.g. [ <prefix>... , "{index}", <suffix>... ], array or slice values are stored at "prefix/<index>" using <suffix> format.
	format []string

	// When setting a value, traversing a map will make a value non-addressible.
	// We have to remember which is the last crossed map, such as to make the traversal addressable if necessary.
	lastMapIndirection *objectPath
}

type findOptions struct {
	// Creates the searched object if it does not exists yet.
	Create bool

	// When non-nil, sets the searched object by serializing the string into the searched object.
	SetValue *string

	// If Create and SetValue are set, and the provided string fails unmarshaling,
	// the default value is used instead.
	IgnoreUnmarshalFailure bool

	// When non-nil, sets the searched object with the given value.
	SetObject interface{}

	// Next time a map entry is crossed, it will be made addressable for the rest of the way
	MakeMapAddressable bool
}

// Returns the format
func getStructFieldFormat(f reflect.StructField) ([]string, error) {
	tag := f.Tag.Get("kvs")
	if tag == "" {
		return []string{f.Name}, nil
	} else if tag[:1] == "/" {
		return nil, ErrTagFirstSlash
	} else {
		return strings.Split(tag, "/"), nil
	}
}

func serializeValue(v reflect.Value) (string, error) {
	if v.Type().Kind() == reflect.String {
		return v.Interface().(string), nil
	}

	arr, err := json.Marshal(v.Interface())
	if err != nil {
		return "", err
	}
	return string(arr), nil
}

func unserializeValue(val string, t reflect.Type) (reflect.Value, error) {
	v := reflect.New(t).Elem()
	if t.Kind() == reflect.String {
		v.Set(reflect.ValueOf(val))
		return v, nil
	}

	err := json.Unmarshal([]byte(val), v.Addr().Interface())
	if err != nil {
		return reflect.Zero(t), err
	}
	return v, nil
}

func serializeMapKey(v reflect.Value) (string, error) {
	if v.Type().Kind() == reflect.String {
		return v.Interface().(string), nil
	}

	arr, err := json.Marshal(v.Interface())
	if err != nil {
		return "<ERROR>", err
	}
	return string(arr), nil
}

func unserializeMapKey(s string, t reflect.Type) (reflect.Value, error) {
	v := reflect.New(t).Elem()
	if t.Kind() == reflect.String {
		v.Set(reflect.ValueOf(s))
		return v, nil
	}

	err := json.Unmarshal([]byte(s), v.Addr().Interface())
	if err != nil {
		return reflect.Zero(t), err
	}
	return v, nil
}

func (state *encodeState) encodeStruct(o objectPath) error {
	v := o.value
	for i := 0; i < v.NumField(); i++ {
		f := v.Type().Field(i)
		if f.PkgPath != "" {
			// Attribute is not exported
			continue
		}

		format, err := getStructFieldFormat(f)
		if err != nil {
			return err
		}

		o.value = v.Field(i)
		o.format = format

		err = state.encode(o)
		if err != nil {
			return err
		}
	}
	return nil
}

func (state *encodeState) encodeMap(o objectPath) error {
	if len(o.format) == 0 || o.format[0] != "{key}" {
		return fmt.Errorf("Map format must contain a '{key}' element")
	}
	o.format = o.format[1:] //Remove "{key}" from format

	v := o.value
	for _, k := range v.MapKeys() {
		key_string, err := serializeMapKey(k)
		if err != nil {
			return err
		}

		o.value = reflect.Indirect(v.MapIndex(k))
		o.keypath = append(o.keypath, key_string)
		err = state.encode(o)
		if err != nil {
			return err
		}
		o.keypath = o.keypath[:len(o.keypath)-1]
	}
	return nil
}

func (state *encodeState) encodeJson(o objectPath) error {
	key := strings.Join(o.keypath, "/")
	if v, ok := state.kvs[key]; ok {
		return fmt.Errorf("Key '%s' is already used by value '%s'", key, v)
	}

	val, err := serializeValue(o.value)
	if err != nil {
		return err
	}

	state.kvs[key] = val
	return nil
}

func (state *encodeState) encode(o objectPath) error {
	if o.value.Type().Kind() == reflect.Ptr {
		o.value = o.value.Elem()
		return state.encode(o)
	}

	for len(o.format) != 0 {
		if o.format[0] == "" || o.format[0] == "{key}" || o.format[0] == "{index}" {
			break
		}
		o.keypath = append(o.keypath, o.format[0])
		o.format = o.format[1:]
	}

	if len(o.format) == 0 {
		// This element is stored as blob
		return state.encodeJson(o)
	}

	switch o.value.Type().Kind() {
	case reflect.Struct:
		return state.encodeStruct(o)
	case reflect.Map:
		return state.encodeMap(o)
	case reflect.Slice:
		return ErrNotImplemented
	case reflect.Array:
		return ErrNotImplemented
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Invalid, reflect.UnsafePointer:
		return ErrUnsupportedType
	default:
		return ErrScalarType
	}
}

func findByFieldsMap(o objectPath, fields []interface{}, opt findOptions) (objectPath, error) {
	o2 := o
	o.lastMapIndirection = &o2

	if len(o.format) == 0 || o.format[0] != "{key}" {
		return o, fmt.Errorf("Map format must contain a '{key}' element")
	}
	o.format = o.format[1:] //Remove "{key}" from format

	key_type := o.vtype.Key()
	key := reflect.ValueOf(fields[0])
	if key.Type() != key_type {
		return o, ErrFindKeyWrongType
	}

	keystr, err := serializeMapKey(key)
	if err != nil {
		return o, err
	}

	m := o.value
	if o.value.IsValid() {

		if o.value.IsNil() && opt.Create {
			if !o.value.CanSet() {
				return findByFieldsRevertAddressable(o, fields, opt)
			}
			n := reflect.MakeMap(o.vtype) // Create new map
			o.value.Set(n)                // Set the pointer value to the current value
			m = o.value
		}

		// Set object to inner object
		// Note: Use of indirect here is probably weird.
		// It is used to dereference pointers whenever the map stores pointers,
		// such that the object is addressible.
		// But it would also work if it does not.
		val := o.value.MapIndex(key)

		if val.IsValid() {
			o.value = val
		} else if opt.Create {
			val = reflect.New(o.vtype.Elem())    // Get pointer to a new value
			o.value.SetMapIndex(key, val.Elem()) // Set the value in the map
			o.value = o.value.MapIndex(key)      // Get the value
		} else {
			o.value = val
		}
	}

	o.vtype = o.vtype.Elem()                     // Get type of the element
	o.keypath = append(o.keypath, keystr)        // Add object key to keypath
	o.fields = append(o.fields, key.Interface()) // Set field to key object

	if opt.MakeMapAddressable {
		// Note that MakeMapAddressable requires the value to exist. We do not check here.
		val := reflect.New(o.vtype)
		val.Elem().Set(o.value) // Make a copy of the current value

		o.value = val.Elem()
		opt.MakeMapAddressable = false
		o, err = findByFields(o, fields[1:], opt) //Iterate on the addressable value
		if err != nil {
			return o, err
		}
		m.SetMapIndex(key, val.Elem()) // Set the addressable value in the map
		return o, err
	} else {
		// Iterate within the object
		return findByFields(o, fields[1:], opt)
	}
}

func findByFieldsStruct(o objectPath, fields []interface{}, opt findOptions) (objectPath, error) {

	name, ok := fields[0].(string)
	if !ok {
		return o, ErrWrongFieldType
	}
	fields = fields[1:]

	f, ok := o.vtype.FieldByName(name)
	if !ok {
		return o, ErrWrongFieldName
	}

	format, err := getStructFieldFormat(f)
	if err != nil {
		return o, err
	}

	if o.value.IsValid() {
		o.value = o.value.FieldByIndex(f.Index)
	}
	o.vtype = o.vtype.FieldByIndex(f.Index).Type
	o.format = format
	o.fields = append(o.fields, name)

	return findByFields(o, fields, opt)
}

func findByFieldsPtr(o objectPath, fields []interface{}, opt findOptions) (objectPath, error) {
	if o.value.IsValid() { // Value represents an actual pointer
		if o.value.Elem().IsValid() {
			// Pointer contains a valide value
			o.value = o.value.Elem() // Dereference
		} else if opt.Create {
			// Create object
			if !o.value.CanSet() {
				// But can't set !
				return findByFieldsRevertAddressable(o, fields, opt) // Revert to last addressable
			}
			n := reflect.New(o.vtype.Elem()) // Get pointer to a new value
			o.value.Set(n)                   // Set the pointer value to the current value
			o.value = o.value.Elem()         // Dereference
		} else {
			o.value = o.value.Elem() // Just dereference
		}
	}

	o.vtype = o.vtype.Elem() // Dereference type
	return findByFields(o, fields, opt)
}

func findByFieldsFormat(o objectPath, fields []interface{}) (objectPath, []interface{}, error) {
	for len(o.format) != 0 {
		if o.format[0] == "" && len(o.format) == 1 {
			// This object is supposed to be encoded within the given key path
			break
		} else if o.format[0] == "{key}" || o.format[0] == "{index}" {
			//We stop here and can format a map or list element
			break
		} else {
			// Just stack up the format in the keypath
			o.keypath = append(o.keypath, o.format[0])
			o.format = o.format[1:]
		}
	}
	return o, fields, nil
}

func findByFieldsRevertAddressable(o objectPath, fields []interface{}, opt findOptions) (objectPath, error) {
	if o.lastMapIndirection == nil {
		return o, fmt.Errorf("Object is not addressable")
	}

	fields = append(o.fields[len(o.lastMapIndirection.fields):], fields...) // Reconstruct the fields before they were consumed
	o = *o.lastMapIndirection
	opt.MakeMapAddressable = true
	return findByFieldsMap(o, fields, opt)
}

func findByFieldsSetMaybe(o objectPath, fields []interface{}, opt findOptions) (objectPath, error) {
	// If no set is needed, return ok
	if opt.SetObject == nil && opt.SetValue == nil {
		return o, nil
	}

	// Can only set if the value exists (opt.Create should be set if intent is to create too)
	if !o.value.IsValid() {
		return o, ErrFindSetNoExists
	}

	// If object cannot be set, try to rollback
	if !o.value.CanSet() {
		return findByFieldsRevertAddressable(o, fields, opt)
	}

	var value reflect.Value
	var err error
	// If set by string, parse the string
	if opt.SetObject == nil {
		value, err = unserializeValue(*opt.SetValue, o.vtype)
		if err != nil {
			if opt.IgnoreUnmarshalFailure {
				value = reflect.New(o.vtype)
			} else {
				return o, err
			}
		}
	} else {
		value = reflect.ValueOf(opt.SetObject)
	}

	// Check the type
	if value.Type() != o.vtype {
		return o, ErrFindSetWrongType
	}

	// Set the value
	o.value.Set(value)
	return o, nil
}

// Goes directely down an object
func findByFields(o objectPath, fields []interface{}, opt findOptions) (objectPath, error) {
	// First we always dereference pointers, even though the value may become invalid
	if o.vtype.Kind() == reflect.Ptr {
		return findByFieldsPtr(o, fields, opt)
	}

	o, fields, err := findByFieldsFormat(o, fields)
	if err != nil {
		return o, err
	}

	// Now we have removed all leading objects

	if len(fields) == 0 {
		// This is the end of the journey, buddy.
		return findByFieldsSetMaybe(o, fields, opt)
	}

	if len(o.format) == 0 {
		// The object is supposed to be encoded as a blob
		// NOTE: It would make sense to check if fields correspond to an inner object, and possibly
		// return it with the reduced key.
		// For now let's just return an error.
		if len(fields) != 0 {
			return o, ErrFindPathPastObject
		}
	}

	switch o.vtype.Kind() {
	case reflect.Struct:
		return findByFieldsStruct(o, fields, opt)
	case reflect.Map:
		return findByFieldsMap(o, fields, opt)
	case reflect.Slice:
		return o, ErrNotImplemented
	case reflect.Array:
		return o, ErrNotImplemented
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Invalid, reflect.UnsafePointer:
		return o, ErrUnsupportedType
	default:
		return o, ErrScalarType
	}
}

// Finds a sub-object based on the path of successive fields.
//
// Returns the found object, its path, and possibly an error.
func FindByFields(object interface{}, format string, fields []interface{}) (interface{}, string, error) {
	o := objectPath{
		value:  reflect.ValueOf(object),
		vtype:  reflect.TypeOf(object),
		format: strings.Split(format, "/"),
	}

	o, err := findByFields(o, fields, findOptions{})
	if err != nil {
		return nil, "", err
	}

	if !o.value.IsValid() {
		return nil, "", ErrFindKeyNotFound
	}

	if !o.value.CanAddr() {
		// Returning a copy if the object is non-addressable
		return o.value.Interface(), strings.Join(append(o.keypath, o.format...), "/"), nil
	}

	// If the value is addressable, return a pointer
	return o.value.Addr().Interface(), strings.Join(append(o.keypath, o.format...), "/"), nil
}

// Encode part of the object stored at position key.
// The subfield is identified by a list of fields.
// Structure attributes are identified by name (as a string).
// Slice indexes are identified with integers.
// Map keys are identified by given an object of the same type than the map key.
func Encode(format string, object interface{}, fields ...interface{}) (map[string]string, error) {

	formatpath := strings.Split(format, "/")

	o := objectPath{
		value:   reflect.ValueOf(object),
		vtype:   reflect.TypeOf(object),
		format:  formatpath,
		keypath: []string{},
	}

	o, err := findByFields(o, fields, findOptions{})
	if err != nil {
		return nil, err
	}
	if !o.value.IsValid() {
		return nil, ErrFindObjectNotFound
	}

	state := &encodeState{
		kvs: make(map[string]string),
	}
	err = state.encode(o)
	if err != nil {
		return nil, err
	}

	return state.kvs, nil
}

// Find sub-object from struct per its key
// Returns the found object, the consumed key path
func findByKeyOneStruct(o objectPath, path []string, opt findOptions) (objectPath, error) {
	if len(o.format) != 1 && o.format[0] != "" {
		return o, fmt.Errorf("Struct object expect [\"\"] format")
	}

	v := o.value
	t := o.vtype
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			// Attribute is not exported
			continue
		}

		format, err := getStructFieldFormat(f)
		if err != nil {
			return o, err
		}

		if v.IsValid() {
			o.value = v.Field(i) // Get field if value exists
		}
		o.vtype = f.Type // Get attribute type
		o.format = format

		// First see if the format corresponds
		o2, path2, err := findByKeyFormat(o, path)
		if err == nil {
			// We can fully look in there
			o2.fields = append(o2.fields, f.Name)
			return findByKey(o2, path2, opt)
		}
		// Let's continue searching
	}
	return o, ErrFindPathNotFound
}

// Finds a sub-object inside a map with the provided object format (e.g. {key}, {key}/, {key}/name).
func findByKeyOneMap(o objectPath, path []string, opt findOptions) (objectPath, error) {

	if o.value.IsValid() && o.value.IsNil() && opt.Create && !o.value.CanSet() {
		// Create MAP if necessary
		return findByKeyRevertAddressable(o, path, opt)
	}

	o2 := o
	o.lastMapIndirection = &o2

	if len(o.format) == 0 || o.format[0] != "{key}" {
		return o, fmt.Errorf("Map format must contain a '{key}' element")
	}
	o.format = o.format[1:] // Consume {key} format

	// Consume key
	keyvalue, err := unserializeMapKey(path[0], o.vtype.Key())
	if err != nil {
		return o, err
	}

	m := o.value
	if o.value.IsValid() {

		if o.value.IsNil() && opt.Create {
			n := reflect.MakeMap(o.vtype) // Create new map
			o.value.Set(n)                // Set the pointer value to the current value
			m = o.value
		}

		// Set object to inner object
		// Note: Use of indirect here is probably weird.
		// It is used to dereference pointers whenever the map stores pointers,
		// such that the object is addressible.
		// But it would also work if it does not.
		val := o.value.MapIndex(keyvalue)

		if val.IsValid() {
			o.value = val
		} else if opt.Create {
			val = reflect.New(o.vtype.Elem())         // Get pointer to a new value
			o.value.SetMapIndex(keyvalue, val.Elem()) // Set the value in the map
			o.value = o.value.MapIndex(keyvalue)      // Get the value
		} else {
			o.value = val
		}
	}

	o.fields = append(o.fields, keyvalue.Interface()) // Set field to key object
	o.vtype = o.vtype.Elem()                          // Get the map value type
	o.keypath = append(o.keypath, path[0])            // Add object key to keypath

	if opt.MakeMapAddressable {
		// Note that MakeMapAddressable requires the value to exist. We do not check here.
		val := reflect.New(o.vtype)
		val.Elem().Set(o.value) // Make a copy of the current value

		o.value = val.Elem()
		opt.MakeMapAddressable = false
		o, err := findByKey(o, path[1:], opt) //Iterate on the addressable value
		if err != nil {
			return o, err
		}
		m.SetMapIndex(keyvalue, val.Elem()) // Set the addressable value in the map
		return o, err
	} else {
		// Iterate within the object
		return findByKey(o, path[1:], opt)
	}
}

func findByKeyPtr(o objectPath, path []string, opt findOptions) (objectPath, error) {
	if o.value.IsValid() {

		if o.value.Elem().IsValid() {
			o.value = o.value.Elem()
		} else if opt.Create {
			if !o.value.CanSet() {
				return findByKeyRevertAddressable(o, path, opt)
			}
			n := reflect.New(o.vtype.Elem()) // Get pointer to a new value
			o.value.Set(n)                   // Set the pointer value to the current value
			o.value = o.value.Elem()         // Dereference
		} else {
			o.value = o.value.Elem()
		}
	}
	o.vtype = o.vtype.Elem()
	return findByKey(o, path, opt)
}

func findByKeyFormat(o objectPath, path []string) (objectPath, []string, error) {
	for len(o.format) != 0 {
		if o.format[0] == "" && len(o.format) == 1 {
			// This object is supposed to be encoded within the given key path
			break
		} else if o.format[0] == "{key}" || o.format[0] == "{index}" {
			//We stop here and can format a map, array or slice element
			break
		} else if len(path) == 0 {
			// We are going to stop now
			break
		} else if o.format[0] != path[0] {
			// Provided path does not match the expected format
			return o, path, ErrFindPathNotFound
		} else {
			// Pile-up key and continue
			o.keypath = append(o.keypath, path[0])
			path = path[1:]
			o.format = o.format[1:]
		}
	}
	return o, path, nil
}

// When some object must be changed but is not addressable, we revert to the last addressable object
// and restart while asking for the rest of the process to be addressable.
func findByKeyRevertAddressable(o objectPath, path []string, opt findOptions) (objectPath, error) {
	if o.lastMapIndirection == nil {
		return o, fmt.Errorf("Object is not addressable")
	}

	path = append(o.keypath[len(o.lastMapIndirection.keypath):], path...) // Reconstruct the keypath before it was consumed
	o = *o.lastMapIndirection
	opt.MakeMapAddressable = true
	return findByKeyOneMap(o, path, opt)
}

func findByKeySetMaybe(o objectPath, path []string, opt findOptions) (objectPath, error) {
	// If no set is needed, return ok
	if opt.SetObject == nil && opt.SetValue == nil {
		return o, nil
	}

	// Can only set if the value exists (opt.Create should be set if intent is to create too)
	if !o.value.IsValid() {
		return o, ErrFindSetNoExists
	}

	// If object cannot be set, try to rollback
	if !o.value.CanSet() {
		return findByKeyRevertAddressable(o, path, opt)
	}

	var value reflect.Value
	var err error
	// If set by string, parse the string
	if opt.SetObject == nil {
		value, err = unserializeValue(*opt.SetValue, o.vtype)
		if err != nil {
			if opt.IgnoreUnmarshalFailure {
				value = reflect.New(o.vtype).Elem()
			} else {
				return o, err
			}
		}
	} else {
		value = reflect.ValueOf(opt.SetObject).Elem()
	}

	// Check the type
	if value.Type() != o.vtype {
		return o, ErrFindSetWrongType
	}

	// Set the value
	o.value.Set(value)
	return o, nil
}

func findByKey(o objectPath, path []string, opt findOptions) (objectPath, error) {
	if o.vtype.Kind() == reflect.Ptr {
		// Let's first dereference (Before actually parsing the keys)
		return findByKeyPtr(o, path, opt)
	}

	// Go through format prefixing element (before "", "{key}" or "{index}")
	o, path, err := findByKeyFormat(o, path)
	if err != nil {
		return o, err
	}

	if len(o.format) == 0 {
		// The object is supposed to be encoded as a blob
		if len(path) != 0 {
			// Path is too specific and therefore does not correspond to an encoded object.
			return o, ErrFindPathPastObject
		}
		return findByKeySetMaybe(o, path, opt)
	}

	if len(path) == 0 || (path[0] == "" && len(path) != 1) {
		// We reached the end of the requested path but the object expects more.
		return o, ErrFindKeyInvalid
	}

	if path[0] == "" {
		return findByKeySetMaybe(o, path, opt)
	}

	switch o.vtype.Kind() {
	case reflect.Struct:
		return findByKeyOneStruct(o, path, opt)
	case reflect.Map:
		return findByKeyOneMap(o, path, opt)
	case reflect.Slice:
		return o, ErrNotImplemented
	case reflect.Array:
		return o, ErrNotImplemented
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Invalid, reflect.UnsafePointer:
		return o, ErrUnsupportedType
	default:
		return o, ErrScalarType
	}
}

// FindByKey returns a sub-object by following the provided path.
//
// 'format' is the provided object key formatting string,
// equivalent to the attribute 'kvs' tags from struct fields.
// For most types, providing a format is optional.
//
// Note that the provided path should include the format, or specific values
// used by the format. For instance, if the format is "here/{key}/there/", then
// the path should start with "here/<some-key-value>/there/".
func FindByKey(o interface{}, format string, path string) (interface{}, []interface{}, error) {
	op := objectPath{
		value:  reflect.ValueOf(o),
		vtype:  reflect.TypeOf(o),
		format: strings.Split(format, "/"),
	}
	op, err := findByKey(op, strings.Split(path, "/"), findOptions{})
	if err != nil {
		return nil, nil, err
	}

	if !op.value.IsValid() {
		return nil, nil, ErrFindKeyNotFound
	}

	if !op.value.CanAddr() {
		// If the value is not addressable, return a copy
		return op.value.Interface(), op.fields, nil
	}

	return op.value.Addr().Interface(), op.fields, nil
}

// Update transforms a (key,value) into an actually modified object.
//
// Given an object and its format, as well as a (key, value) pair (where key is relative to the object),
// Update modifies the object, returns the field path to the modified sub-object.
func UpdateKeyObject(object interface{}, format string, keypath string, value string) ([]interface{}, error) {
	o := objectPath{
		value:  reflect.ValueOf(object),
		vtype:  reflect.TypeOf(object),
		format: strings.Split(format, "/"),
	}
	opt := findOptions{
		Create:                 true,
		SetValue:               &value,
		IgnoreUnmarshalFailure: true,
	}
	o, err := findByKey(o, strings.Split(keypath, "/"), opt)
	if err != nil {
		return nil, err
	}

	return o.fields, nil
}

func DeleteKeyObject(object interface{}, format string, keypath string) ([]interface{}, error) {
	o := objectPath{
		value:  reflect.ValueOf(object),
		vtype:  reflect.TypeOf(object),
		format: strings.Split(format, "/"),
	}

	opt := findOptions{}
	path := strings.Split(keypath, "/")

	o, err := findByKey(o, path, opt)
	if err != nil && err != ErrFindKeyInvalid {
		// Getting ErrFindKeyInvalid means the key does not represent an encoded value, which is ok in this case
		return nil, err
	}

	err, _ = DeleteByFields(object, format, o.fields...)
	return o.fields, err
}

func SetByFields(object interface{}, format string, value interface{}, fields ...interface{}) error {
	o := objectPath{
		value:  reflect.ValueOf(object),
		vtype:  reflect.TypeOf(object),
		format: strings.Split(format, "/"),
	}

	opt := findOptions{
		Create:    true,
		SetObject: value,
	}
	_, err := findByFields(o, fields, opt)
	if err != nil {
		return err
	}

	return nil
}

// Deletes an element from a map, which means the last element from the fields
// list must be a key, and the previous fields must reference a map object.
// Returns an error, or nil and the format string of the removed object
func DeleteByFields(object interface{}, format string, fields ...interface{}) (error, string) {
	if len(fields) < 1 {
		return ErrNotMapIndex, ""
	}

	o := objectPath{
		value:  reflect.ValueOf(object),
		vtype:  reflect.TypeOf(object),
		format: strings.Split(format, "/"),
	}

	opt := findOptions{}
	o, err := findByFields(o, fields[0:len(fields)-1], opt)
	if err != nil {
		return err, ""
	}

	if o.vtype.Kind() != reflect.Map {
		return ErrNotMapIndex, ""
	}

	o2, err := findByFields(o, fields[len(fields)-1:], opt)
	if err != nil {
		return err, ""
	}

	if !o2.value.IsValid() {
		return ErrFindObjectNotFound, ""
	}

	key := reflect.ValueOf(fields[len(fields)-1])
	o.value.SetMapIndex(key, reflect.ValueOf(nil))

	keypath := strings.Join(o2.keypath, "/")
	if len(o2.format) != 0 { //More subkeys
		keypath = keypath + "/"
	}

	return nil, keypath
}
