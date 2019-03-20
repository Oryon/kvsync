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
var errNotImplemented = errors.New("Not implemented")
var errUnsupportedType = errors.New("Object type not supported")
var errFindPathPastObject = errors.New("Provided path goes past an encoded object")
var errFindKeyNotFound = errors.New("Key was not found in map")
var errFindPathNotFound = errors.New("Object not found at specified path")
var errNotAddressible = errors.New("Requested object is not addressible")

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

func getStructFieldKey(f reflect.StructField) (string, error) {
	tag := f.Tag.Get("kvs")
	if tag == "" {
		return f.Name, nil
	} else if tag[:1] == "/" {
		return "<ERROR>", fmt.Errorf("tag must not start with /")
	} else {
		return tag, nil
	}
}

func appendStructField(dir string, f reflect.StructField) (string, error) {
	key, err := getStructFieldKey(f)
	return dir + key, err
}

func serializeMapKey(v reflect.Value) (string, error) {
	arr, err := json.Marshal(v.Interface())
	if err != nil {
		fmt.Printf("Error %v\n", err)
		return "<ERROR>", err
	}
	return string(arr), nil
}

func unserializeMapKey(s string, t reflect.Type) (reflect.Value, error) {
	v := reflect.New(t).Elem()
	err := json.Unmarshal([]byte(s), v.Addr().Interface())
	if err != nil {
		fmt.Printf("Error %s %v %v\n", s, t, err)
		return reflect.Zero(t), err
	}
	return v, nil
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

// State used to parse an object.
type objectPath struct {

	// A value pointing to the current object.
	value reflect.Value

	// The key path used to reach the current object, but not referring to the
	// object path itself (see 'format').
	keypath []string

	// The set of specific fields (attributes names, keys and indexes) used
	// to arrive to this object.
	fields []interface{}

	// The key format to be used by the encoded object.
	// e.g. [ "name" ],  [ "name", "" ],
	// e.g. for a map [ "{key}", "name" , "" ], [ "{key}" ], [ "{index}", "" ]
	// e.g. for a list of maps [ "{index}", "name" , "{key}", "" ]
	format []string
}

// Find sub-object from struct per its key
// Returns the found object, the consumed key path
func findByKeyOneStruct(o objectPath, path []string) (objectPath, []string, error) {
	if len(o.format) != 1 && o.format[0] != "" {
		return o, path, fmt.Errorf("Struct object expect [\"\"] format")
	}

	v := o.value
	for i := 0; i < v.Type().NumField(); i++ {
		f := v.Type().Field(i)

		k, err := getStructFieldKey(f)
		if err != nil {
			return o, path, err
		}

		o.value = v.Field(i)
		o.format = strings.Split(k, "/")

		// First see if the format corresponds
		o2, path2, err := findByKeyFormat(o, path)
		if err == nil {
			// We can fully look in there
			o2.fields = append(o2.fields, f.Name)
			return findByKey(o2, path2)
		}
		// Let's continue searching
	}
	return o, path, errFindPathNotFound
}

// Finds a sub-object inside a map with the provided object format (e.g. {key}, {key}/, {key}/name).
func findByKeyOneMap(o objectPath, path []string) (objectPath, []string, error) {

	if len(o.format) == 0 || o.format[0] != "{key}" {
		//TODO: Replace this by a check in caller
		return o, path, fmt.Errorf("Map format must contain a '{key}' element")
	}
	o.format = o.format[1:] // Consume {key} format

	// Consume key
	t := o.value.Type()

	keyvalue, err := unserializeMapKey(path[0], t.Key())
	if err != nil {
		return o, path, err
	}

	v := o.value.MapIndex(keyvalue)
	if !v.IsValid() {
		//TODO: Add possibility to create the value
		return o, path, errFindKeyNotFound
	}

	o.keypath = append(o.keypath, path[0]) // Add object key to keypath

	// Set object to inner object
	// Note: Use of indirect here is probably weird.
	// It is used to dereference pointers whenever the map stores pointers,
	// such that the object is addressible.
	// But it would also work if it does not.
	o.value = reflect.Indirect(o.value.MapIndex(keyvalue))

	// Set field to key object
	o.fields = append(o.fields, keyvalue.Interface())

	// Iterate within the object
	return findByKey(o, path[1:])
}

func findByKeyFormat(o objectPath, path []string) (objectPath, []string, error) {
	for len(o.format) != 0 {
		if o.format[0] == "" {
			// This object is supposed to be encoded within the given key path
			if len(o.format) != 1 {
				// "" key element must be last
				return o, nil, fmt.Errorf("Format contains an intermediate space")
			}
			break
		} else if o.format[0] == "{key}" {
			//We stop here and can format a map element
			break
		} else if o.format[0] == "{index}" {
			//We stop here and can format a slice or array element
			break
		} else if len(path) == 0 || o.format[0] != path[0] {
			// Provided path does not match the expected format
			return o, path, errFindPathNotFound
		} else {
			// Pile-up key and continue
			o.keypath = append(o.keypath, path[0])
			path = path[1:]
			o.format = o.format[1:]
		}
	}
	return o, path, nil
}

func findByKey(o objectPath, path []string) (objectPath, []string, error) {
	// First go through formatting elements
	o, path, err := findByKeyFormat(o, path)
	if err != nil {
		return o, nil, err
	}

	if len(o.format) == 0 {
		// The object is supposed to be encoded as a blob
		if len(path) != 0 {
			return o, nil, errFindPathPastObject
		}
	}

	if len(path) == 0 || (len(path) == 1 && path[0] == "") {
		// We reached the end of the requested path but there still is some format.
		return o, path, nil
	}

	switch o.value.Type().Kind() {
	case reflect.Struct:
		return findByKeyOneStruct(o, path)
	case reflect.Map:
		return findByKeyOneMap(o, path)
	case reflect.Slice:
		return o, nil, errNotImplemented
	case reflect.Array:
		return o, nil, errNotImplemented
	case reflect.Ptr:
		o.value = o.value.Elem()
		return findByKey(o, path)
	default:
		return o, nil, fmt.Errorf("Unsupported type %v", o.value.Type().Kind())
	}
}

func FindByKey(o interface{}, format string, subpath string) (interface{}, []interface{}, error) {
	op := objectPath{
		value:  reflect.ValueOf(o),
		format: strings.Split(format, "/"),
	}
	op, _, err := findByKey(op, strings.Split(subpath, "/"))
	if err != nil {
		return nil, nil, err
	}
	if !op.value.CanAddr() {
		//NOTE: Another option would be to return a copy of the value
		return nil, nil, errNotAddressible
	}

	return op.value.Addr().Interface(), op.fields, nil
}
