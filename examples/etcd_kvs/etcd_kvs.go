package main

import (
	"context"
	"fmt"
	"github.com/Oryon/kvsync/kvs/etcd"
)

// This example shows a simple etcd synchronizer.
//
// It gets all updates from a given repository,
// but in contrast of etcd's native api, the listener
// also starts by getting the current state as if it was created
// when the program starts.

func main() {

	// Creating an etcd kvs object.
	kv, err := etcd.CreateFromEndpoint("http://localhost:2379/", "/")
	if err != nil {
		fmt.Printf("etcd.CreateFromEndpoint returned error: %v\n", err)
		return
	}

	for {
		// Getting next sync kvs event.
		u, err := kv.Next(context.Background())
		if err != nil {
			fmt.Printf("kv.Next returned error: %v\n", err)
			return
		}

		// Just displaying what happened
		if u.Previous != nil && u.Value != nil {
			fmt.Printf("Key '%s' Update from '%v' to '%v'\n", u.Key, *u.Previous, *u.Value)
		} else if u.Previous != nil {
			fmt.Printf("Key '%s' Deleted from '%v'\n", u.Key, *u.Previous)
		} else {
			fmt.Printf("Key '%s' Added to '%v'\n", u.Key, *u.Value)
		}
	}
}
