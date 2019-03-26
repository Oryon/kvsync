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
var errFindSetNoExists = errors.New("Cannot set non existent object")
var errFindSetWrongType = errors.New("The provided object is of wrong type")
var errScalarType = errors.New("Cannot recursively store scalar type")

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

	// When non-nil, sets the searched object with the given value.
	SetObject interface{}

	// Next time a map entry is crossed, it will be made addressable for the rest of the way
	MakeMapAddressable bool
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
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Invalid, reflect.UnsafePointer:
		return errUnsupportedType
	default:
		return errScalarType
	}
}

func findByFieldsMap(o objectPath, fields []interface{}) (objectPath, []interface{}, error) {
	if len(o.format) == 0 || o.format[0] != "{key}" {
		return o, fields, fmt.Errorf("Map format must contain a '{key}' element")
	}
	o.format = o.format[1:] //Remove "{key}" from format

	key_type := o.vtype.Key()
	key := reflect.ValueOf(fields[0])
	if key.Type() != key_type {
		return o, nil, ErrWrongFieldType
	}

	keystr, err := serializeMapKey(key)
	if err != nil {
		return o, nil, err
	}

	if o.value.IsValid() {
		// When the current value exists, try to lookup the map
		v := o.value.MapIndex(key)
		if v.IsValid() {
			// Set object to inner object
			// Note: Use of indirect here is probably weird.
			// It is used to dereference pointers whenever the map stores pointers,
			// such that the object is addressible.
			// But it would also work if it does not.
			o.value = reflect.Indirect(v)
		}
		//TODO: Add option to add when non existent
	}

	o.vtype = o.vtype.Elem()                     // Get type of the element
	o.keypath = append(o.keypath, keystr)        // Add object key to keypath
	o.fields = append(o.fields, key.Interface()) // Set field to key object

	return findByFields(o, fields)
}

func findByFieldsStruct(o objectPath, fields []interface{}) (objectPath, []interface{}, error) {
	name, ok := fields[0].(string)
	if !ok {
		return o, nil, ErrWrongFieldType
	}
	fields = fields[1:]

	f, ok := o.vtype.FieldByName(name)
	if !ok {
		return o, nil, ErrWrongFieldName
	}

	format, err := getStructFieldKey(f)
	if err != nil {
		return o, fields, err
	}

	if o.value.IsValid() {
		o.value = o.value.FieldByIndex(f.Index)
	}
	o.vtype = o.vtype.FieldByIndex(f.Index).Type
	o.format = strings.Split(format, "/")
	o.fields = append(o.fields, name)

	return findByFields(o, fields)
}

// Goes directely down an object
func findByFields(o objectPath, fields []interface{}) (objectPath, []interface{}, error) {

	// First we always dereference pointers, even though the value may become invalid
	if o.vtype.Kind() == reflect.Ptr {
		if o.value.IsValid() {
			o.value = o.value.Elem()
			// TODO: Maybe create the value if does not exist
		}
		o.vtype = o.vtype.Elem()
		return findByFields(o, fields)
	}

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

	switch o.vtype.Kind() {
	case reflect.Struct:
		return findByFieldsStruct(o, fields)
	case reflect.Map:
		return findByFieldsMap(o, fields)
	case reflect.Slice:
		return o, nil, errNotImplemented
	case reflect.Array:
		return o, nil, errNotImplemented
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Invalid, reflect.UnsafePointer:
		return o, nil, errUnsupportedType
	default:
		return o, nil, errScalarType
	}
}

// Finds a sub-object based on the path of successive fields.
//
// Returns the found object, its path, and possibly an error.
func FindByFields(object interface{}, format string, fields ...interface{}) (interface{}, string, string, error) {
	o := objectPath{
		value:  reflect.ValueOf(object),
		vtype:  reflect.TypeOf(object),
		format: strings.Split(format, "/"),
	}

	o, _, err := findByFields(o, fields)
	if err != nil {
		return nil, "", "", err
	}

	if !o.value.IsValid() {
		return nil, "", "", errFindKeyNotFound
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
		vtype:   reflect.TypeOf(object),
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
func findByKeyOneStruct(o objectPath, path []string, opt findOptions) (objectPath, error) {
	if len(o.format) != 1 && o.format[0] != "" {
		return o, fmt.Errorf("Struct object expect [\"\"] format")
	}

	v := o.value
	t := o.vtype
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)

		k, err := getStructFieldKey(f)
		if err != nil {
			return o, err
		}

		if v.IsValid() {
			o.value = v.Field(i) // Get field if value exists
		}
		o.vtype = f.Type // Get attribute type
		o.format = strings.Split(k, "/")

		// First see if the format corresponds
		o2, path2, err := findByKeyFormat(o, path)
		if err == nil {
			// We can fully look in there
			o2.fields = append(o2.fields, f.Name)
			return findByKey(o2, path2, opt)
		}
		// Let's continue searching
	}
	return o, errFindPathNotFound
}

// Finds a sub-object inside a map with the provided object format (e.g. {key}, {key}/, {key}/name).
func findByKeyOneMap(o objectPath, path []string, opt findOptions) (objectPath, error) {
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
			if !o.value.CanSet() {
				return findByKeyRevertAddressable(o, path, opt)
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
		return o, errFindSetNoExists
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
			return o, err
		}
	} else {
		value = reflect.ValueOf(opt.SetObject).Elem()
	}

	// Check the type
	if value.Type() != o.vtype {
		return o, errFindSetWrongType
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
			return o, errFindPathPastObject
		}
		return findByKeySetMaybe(o, path, opt)
	}

	if len(path) == 0 || (path[0] == "" && len(path) != 1) {
		// We reached the end of the requested path but the object expects more.
		return o, errFindKeyInvalid
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
		return o, errNotImplemented
	case reflect.Array:
		return o, errNotImplemented
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Invalid, reflect.UnsafePointer:
		return o, errUnsupportedType
	default:
		return o, errScalarType
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
		return nil, nil, errFindKeyNotFound
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
		Create:   true,
		SetValue: &value,
	}
	o, err := findByKey(o, strings.Split(keypath, "/"), opt)
	if err != nil {
		return nil, err
	}

	return o.fields, nil
}
