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

// Generic key-value storage interface definitions.
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
