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

// Deletes a part of an object in the KV Store
func Delete(s kvs.Store, c context.Context, key string, object interface{}, fields ...interface{}) error {
	return ErrNotImplemented
}
