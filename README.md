# Key-Value Synchronization Library

This library provides a simple way to set, get and synchronize golang objects (e.g., `int`, `string`, `struct`, ...) to and from a key-value storage (e.g. *etcd*).

The way the different objects are stored within the key-value store can be modified using struct attribute tags, very similarly to using JSON marshaling from `"encoding/json"`.

*etcd* and *golang maps* are supported natively, but any other Key-Value storage can be used by implementing the provided `KVS` interface.

## Exemple

TODO