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
var errFindKeyInvalid = errors.New("Invalid key for this object")
var errFindPathNotFound = errors.New("Object not found at specified path")
var errNotAddressable = errors.New("Requested object is not addressable")

// State storing keys and values before they get stored for one or multiple objects
type encodeState struct {
	kvs map[string]string
}

// State representing an object as well as its path in some parent opbjects.
// This path is not absolute, and this struct does not keep memory of
// the path root.
type objectPath struct {

	// A value pointing to the current object.
	// Careful, it may be non addressable.
	value reflect.Value

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

func serializeValue(v reflect.Value) (string, error) {
	arr, err := json.Marshal(v.Interface())
	if err != nil {
		return "", err
	}
	return string(arr), nil
}

func unserializeValue(val string, t reflect.Type) (reflect.Value, error) {
	v := reflect.New(t).Elem()
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

		k, err := getStructFieldKey(f)
		if err != nil {
			return err
		}

		o.value = v.Field(i)
		o.format = strings.Split(k, "/")

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
		return errNotImplemented
	case reflect.Array:
		return errNotImplemented
	default:
		return fmt.Errorf("Unsupported type %v", o.value.Kind())
		return errUnsupportedType
	}
}

func findByFieldsMap(o objectPath, fields []interface{}) (objectPath, []interface{}, error) {
	if len(o.format) == 0 || o.format[0] != "{key}" {
		return o, fields, fmt.Errorf("Map format must contain a '{key}' element")
	}
	o.format = o.format[1:] //Remove "{key}" from format

	key_type := o.value.Type().Key()
	key := reflect.ValueOf(fields[0])
	if key.Type() != key_type {
		return o, nil, ErrWrongFieldType
	}

	keystr, err := serializeMapKey(key)
	if err != nil {
		return o, nil, err
	}

	v := o.value.MapIndex(key)
	if !v.IsValid() {
		//TODO: Add possibility to create the value
		return o, fields, errFindKeyNotFound
	}

	o.keypath = append(o.keypath, keystr)        // Add object key to keypath
	o.fields = append(o.fields, key.Interface()) // Set field to key object

	// Set object to inner object
	// Note: Use of indirect here is probably weird.
	// It is used to dereference pointers whenever the map stores pointers,
	// such that the object is addressible.
	// But it would also work if it does not.
	o.value = reflect.Indirect(v)

	return findByFields(o, fields)
}

func findByFieldsStruct(o objectPath, fields []interface{}) (objectPath, []interface{}, error) {
	name, ok := fields[0].(string)
	if !ok {
		return o, nil, ErrWrongFieldType
	}
	fields = fields[1:]

	f, ok := o.value.Type().FieldByName(name)
	if !ok {
		return o, nil, ErrWrongFieldName
	}

	format, err := getStructFieldKey(f)
	if err != nil {
		return o, fields, err
	}

	o.value = o.value.FieldByIndex(f.Index)
	o.format = strings.Split(format, "/")
	o.fields = append(o.fields, name)

	return findByFields(o, fields)
}

// Goes directely down an object
func findByFields(o objectPath, fields []interface{}) (objectPath, []interface{}, error) {
	for len(o.format) != 0 {
		if o.format[0] == "" {
			// This object is supposed to be encoded within the given key path
			if len(o.format) != 1 {
				// "" key element must be last
				return o, nil, fmt.Errorf("Format contains an intermediate space")
			}
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

	// Now we have removed all leading objects

	if len(fields) == 0 {
		// This is the end of the journey, buddy.
		return o, fields, nil
	}

	if len(o.format) == 0 {
		// The object is supposed to be encoded as a blob
		// NOTE: It would make sense to check if fields correspond to an inner object, and possibly
		// return it with the reduced key.
		// For now let's just return an error.
		if len(fields) != 0 {
			return o, nil, errFindPathPastObject
		}
	}

	if len(fields) == 0 {
		// We found the object
		return o, fields, nil
	}

	switch o.value.Type().Kind() {
	case reflect.Struct:
		return findByFieldsStruct(o, fields)
	case reflect.Map:
		return findByFieldsMap(o, fields)
	case reflect.Slice:
		return o, nil, errNotImplemented
	case reflect.Array:
		return o, nil, errNotImplemented
	case reflect.Ptr:
		o.value = o.value.Elem()
		return findByFields(o, fields)
	default:
		return o, nil, fmt.Errorf("Unsupported type %v", o.value.Type().Kind())
	}
}

// Finds a sub-object based on the path of successive fields.
//
// Returns the found object, its path, and possibly an error.
func FindByFields(object interface{}, format string, fields ...interface{}) (interface{}, string, string, error) {
	o := objectPath{
		value:  reflect.ValueOf(object),
		format: strings.Split(format, "/"),
	}

	o, _, err := findByFields(o, fields)
	if err != nil {
		return nil, "", "", err
	}

	if !o.value.CanAddr() {
		// Returning a copy if the object is non-addressable
		return o.value.Interface(), strings.Join(o.keypath, "/"), strings.Join(o.format, "/"), nil
	}

	// If the value is addressable, return a pointer
	return o.value.Addr().Interface(), strings.Join(o.keypath, "/"), strings.Join(o.format, "/"), nil
}

// Encode part of the object stored at position key.
// The subfield is identified by a list of fields.
// Structure attributes are identified by name (as a string).
// Slice indexes are identified with integers.
// Map keys are identified by given an object of the same type than the map key.
func Encode(key string, object interface{}, fields ...interface{}) (map[string]string, error) {

	keypath := strings.Split(key, "/")
	if len(keypath) == 0 || keypath[0] != "" {
		return nil, ErrFirstSlash
	}

	o := objectPath{
		value:   reflect.ValueOf(object),
		format:  keypath[1:],
		keypath: []string{""},
	}

	o, _, err := findByFields(o, fields)
	if err != nil {
		return nil, err
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
func findByKeyOneStruct(o objectPath, path []string) (objectPath, error) {
	if len(o.format) != 1 && o.format[0] != "" {
		return o, fmt.Errorf("Struct object expect [\"\"] format")
	}

	v := o.value
	for i := 0; i < v.Type().NumField(); i++ {
		f := v.Type().Field(i)

		k, err := getStructFieldKey(f)
		if err != nil {
			return o, err
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
	return o, errFindPathNotFound
}

// Finds a sub-object inside a map with the provided object format (e.g. {key}, {key}/, {key}/name).
func findByKeyOneMap(o objectPath, path []string) (objectPath, error) {

	if len(o.format) == 0 || o.format[0] != "{key}" {
		//TODO: Replace this by a check in caller
		return o, fmt.Errorf("Map format must contain a '{key}' element")
	}
	o.format = o.format[1:] // Consume {key} format

	// Consume key
	t := o.value.Type()

	keyvalue, err := unserializeMapKey(path[0], t.Key())
	if err != nil {
		return o, err
	}

	v := o.value.MapIndex(keyvalue)
	if !v.IsValid() {
		//TODO: Add possibility to create the value
		return o, errFindKeyNotFound
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
		} else if o.format[0] == "{key}" || o.format[0] == "{index}" {
			//We stop here and can format a map, array or slice element
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

func findByKey(o objectPath, path []string) (objectPath, error) {
	// First go through format prefixing element (before "", "{key}" or "{index}")
	o, path, err := findByKeyFormat(o, path)
	if err != nil {
		return o, err
	}

	if len(o.format) == 0 {
		// The object is supposed to be encoded as a blob
		if len(path) != 0 {
			// Path is too specific and therefore does not correspond to an encoded object.
			return o, errFindPathPastObject
		}
		return o, nil
	}

	if len(path) == 0 || (path[0] == "" && len(path) != 1) {
		// We reached the end of the requested path but the object expects more.
		return o, errFindKeyInvalid
	}

	if path[0] == "" {
		return o, nil
	}

	switch o.value.Type().Kind() {
	case reflect.Struct:
		return findByKeyOneStruct(o, path)
	case reflect.Map:
		return findByKeyOneMap(o, path)
	case reflect.Slice:
		return o, errNotImplemented
	case reflect.Array:
		return o, errNotImplemented
	case reflect.Ptr:
		o.value = o.value.Elem()
		return findByKey(o, path)
	default:
		return o, fmt.Errorf("Unsupported type %v", o.value.Type().Kind())
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
		format: strings.Split(format, "/"),
	}
	op, err := findByKey(op, strings.Split(path, "/"))
	if err != nil {
		return nil, nil, err
	}
	if !op.value.CanAddr() {
		// If the value is not addressable, return a copy
		return op.value.Interface(), op.fields, nil
	}

	return op.value.Addr().Interface(), op.fields, nil
}

// Update transforms a (key,value) into an actually modified object.
//
// Given an object and its format, as well as a (key, value) pair (were key is relative to the object),
// Update returns the sub-object as described by the provided value, as well as the field path to that sub-object.
func Update(object interface{}, format string, keypath string, value string) (interface{}, []interface{}, error) {
	o := objectPath{
		value:  reflect.ValueOf(object),
		format: strings.Split(format, "/"),
	}
	o, err := findByKey(o, strings.Split(keypath, "/"))
	if err != nil {
		return nil, nil, err
	}

	object, err = unserializeValue(value, o.value.Type())
	if err != nil {
		return nil, nil, err
	}

	return object, o.fields, nil
}
