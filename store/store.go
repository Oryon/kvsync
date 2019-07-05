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

// Functions to store objects into a kvs.
package store

import (
	"context"
	"errors"
	"github.com/Oryon/kvsync/encoding"
	"github.com/Oryon/kvsync/kvs"
)

var ErrNotImplemented = errors.New("Not implemented")

// Puts an object into the key-value store
func Store(s kvs.Store, c context.Context, object interface{}, format string, fields ...interface{}) error {
	m, err := encoding.Encode(format, object, fields...)
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

// Set a value and store it into the KV store
func Set(s kvs.Store, c context.Context, object interface{}, format string, value interface{}, fields ...interface{}) error {
	s.Lock()

	err := encoding.SetByFields(object, format, value, fields...)
	s.Unlock()
	if err != nil {
		return err
	}

	Store(s, c, object, format, fields...)
	return nil
}

// Deletes a part of an object in the KV Store and push the change to the underlying KVStore
func Delete(s kvs.Store, c context.Context, object interface{}, format string, fields ...interface{}) error {
	s.Lock()

	err, key := encoding.DeleteByFields(object, format, fields...)
	s.Unlock()
	if err != nil {
		return err
	}
	s.Delete(c, key)
	return err
}
