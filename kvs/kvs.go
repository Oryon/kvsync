// This package includes KVS interface defintion as well as some
// implementations.
package kvs

import (
	"context"
	"errors"
)

// This interface provides basic functionality to read from a Key-Value store.
type Store interface {
	Set(c context.Context, key string, value string) error
	Delete(c context.Context, key string) error
}

// This struct contains a Key-Value pair update.
type Update struct {
	// The key from the key-value pair.
	Key string

	// The new value, or nil if the pair is being deleted.
	Value *string

	// The previous value, or nil if the pair is being created.
	Previous *string
}

// This interface provides synchronization capability.
type Sync interface {
	// Next method blocks until some change occurs in the backing key-value store,
	// or the context expires.
	// When the key-value store is first open, Next must behave like if all
	// existing key-value pairs had been created instantly.
	// There is no assumption over the order updates are returned.
	Next(c context.Context) (*Update, error)
}

var ErrNoSuchKey = errors.New("No such key")

// This interface provides a way to get the value for a certain key
type Get interface {
	// Get method returns the value associated with the key.
	// If the key can't be found, ErrNoSuchKey is returned as error.
	// Other errors might be returned depending on the underlying storage.
	Get(c context.Context, key string) (string, error)
}
