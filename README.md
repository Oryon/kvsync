# Key-Value Synchronization Library

The *kvsync* is a Go library, providing a simple and flexible way to *set*, *get* and *synchronize* objects, Go types (e.g. `int`, `string`, `struct`, `map`, etc..), to and from a key-value store (e.g. *etcd*). Releaving the burden of writing and testing (de)serialization or synchronization code.

## Storage Format

Go types, such as `int` and `string`, but also `struct` fields or `map` are stored as key-value pairs.

Basic types like `int` and `string` are stored as a single key-value pair, while more complex types like `struct` or `map` can either be stored as JSON-encoded data, or be split into multiple key-value pairs.

The *kvsync* library simplifies the mapping between Go types and key-value pairs with an approach that is very similar to the famous [`encoding/json`](https://golang.org/pkg/encoding/json) from the Go standard library.

### Basics

Any object, when stored, synced or retrieved, is associated with a **format**. The **format** is defined as `string` slash-separated path, specifying the following:

- **Where** to store the object 
- **How** to store the object or its attributes (e.g. *struct fields*, *map keys* or *map values*)

For example, an object with format `/store/here`, would be stored as JSON at key `/store/here`. Formats ending with `/`  indicate that object's attributes should be stored resursively.

### Storing Structs

For `struct` types, adding an additional `/`, results in storing each of struct's fields as different object. Such objects are either stored as JSON (using format `<parent-format>/<attribute-name>`), or using a custom format, which can be specified by *struct tags*.

For example, considering the following struct:

```go
type Person struct {
	Age    int     // no tag
	Name   string  `kvs:"custom/path/to/name"`
	Parent *Person `kvs:"parent/"`
}
```

Storing `Person` with format `/store/here/` (notice the trailing `/` indicating recursive storage) would result in storing fields:

- `Age` at key `/store/here/Age` (default behavior)
- `Name` at key `/store/here/custom/path/to/name`
- `Parent` at key `/store/here/parent/` recursively

Considering another example:

```go
type Student struct {
	Person Person `kvs:"person/"`
	School string
}
```

Storing `Student` with format `/student/` would result in storing `School` attribute in `/student/School` (default behavior), and recursively store each of the `Person` attributes at key `/student/person/<attribute-format>`.

Notice that, if the `Person` tag had been `kvs:"person"` instead, it would have been stored as JSON at key `/student/person`.

### Storing Maps

As with other objects, storing/syncing/retrieving a `map` with format `/store/map/here` would use a JSON-encoded object at key `/store/map/here`.

By appending `/{key}<element-format>` to the format, each of `map` values gets stored using format `<map-format>/{key}<element-format>`, where the `{key}` part is replaced with a map key.

For instance, using format `/map/here/{key}`, element in `mymap["42"]` would be stored as JSON at key `/map/here/42`, whereas using format `/map/here/{key}/` would recursively store the map value at key `/map/here/42/`.


## Change Notifications

The *kvsync* provides callbacks upon modification of a synchronized object. Since an object can be split into multiple keys, the library will tell exactly which part of the object was modified using a **field path** rather than key.

Consider the following struct (using previously defined `Student`):

```go
type Directory struct {
	Students map[int]Student `kvs:"/{key}/"`
}
```

Synchronizing such an object with format `/root/here/there/dir/`, when `Name` of some student gets modified, would notify callback that the `Directory` object was modified at path `{"Students", 42, "Person", "Name"}`.

This approach will let you filter callbacks very efficiently, without having to worry about the actual keys that are used in the key-value store.

Notice that, for map keys, the key field uses the *native* type, which avoids having to parse a string into the proper type!
