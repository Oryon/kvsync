# Key-Value Synchronization Library

The golang *kvsync* library provides a simple and flexible way to set, get and synchronize golang objects (e.g., `int`, `string`, `struct`, `map`, etc...) to and from a key-value storage (e.g. *etcd*). Releaving the burden of writing/testing (de)serialization or synchronization code.

## Storage schemes

Golang objects such as `int`, `string`, but also `struct` fields or `map` are stored as key-value pairs. Simple objects like `int` or `string` are stored as a single pair, while more complex objects like `struct` or `map` can either be stored as JSON blobs, or be split into multiple key-value pairs.

The *kvsync* library simplifies the mapping between golang objects and key-value pairs with an approach that is very similar to the famous `encoding/json` golang standard library.

### The basics

Any object, when stored, synced or retrieved, is associated with a `format`. It is a string similar to a filepath, specifying:

- *Where* to store the object.
- *How* the struct attributes, map keys or map values should be stored.

For example, an object with format `/store/here`, would be stored as JSON at key `/store/here`.

For a `struct`, adding an additional `/` means each of the `struct`'s attributes are stored as different objects. Such attributes are either stored as a JSON blob (using format `<parent-format>/<attribute-name>`), or using a custom format as specified by the `struct tag`. 

For example, considering the following struct:

```
type Person struct {
	Age int
	Name string `kvs:"custom/path/to/name"`
	Parent *Person `kvs:"parent/"`
}
```

Storing a Person object with format `/store/here/` (notice the trailing `/` to request recursive storage) would result in storing the `Age` attribute in `/store/here/Age` (default behavior), and the `Name` attribute in `/store/here/custom/path/to/name`.

Considering another structure:

```
type Studen struct {
	Person Person `kvs:"person/"`
	School string
}
```

Storing a `Student` object with format `/student/` would result in storing `School` attribute in `/student/School` (default behavior), and recursively store the `Person` attribute with format `/student/person/`.

Notice that, if the `Person` tag had been `kvs:"person"` instead, the `Person` attribute would have been stored as a JSON blob with key `/student/person`.


### Golang Maps

As with other objects, storing/syncing/retrieving a `map` with format `/store/map/here` would use a JSON blob at key `/store/map/here`.

By appending `/{key}<element-format>` to the format, each map element gets stored using format `<map-format>/{key}<element-format>`, with the `{key}` string replaced with the object key.

For instance, using format `/map/here/{key}`, element in m["42"] would be stored as JSON at key `/map/here/42`, whereas using key `/map/here/{key}/` would recursively store each object with format `/map/here/42`.


## Change notifications

The *kvsync* provides callbacks upon modification of a synchronized object. Since an object can be split into multiple keys, the library will tell exactly which part of the object was modified using a **field path** rather than key.

Considering the following `struct` (reusing previous `Student` struct).

```
type Directory struct {
	Students map[int]Student `kvs:"/{key}/"`
}
```

Synchronizing such an object with format `/root/here/there/dir/`. if some student's `Name` is modified, the callback would notify that the `Directory` object was modified at path `{"Students", 42, "Person", "Name"}`.

This approach will let you filter callback calls very efficiently, without having to worry about the actual keys that are used in the Key-Value storage.

Notice that, for map keys, the fey field uses the *native* type. No need to parse a string into the correct type !
