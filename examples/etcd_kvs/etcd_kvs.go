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

// Simple example displaying etcd notification changes.
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
