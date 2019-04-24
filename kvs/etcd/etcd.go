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

// Generic kvs interface implementation using etcd.
package etcd

import (
	"context"
	"github.com/Oryon/kvsync/kvs"
	"go.etcd.io/etcd/client"
	"time"
)

type Etcd struct {
	directory     string
	kapi          client.KeysAPI
	listing       []*client.Node
	lastEtcdIndex uint64
	watcher       client.Watcher
	err           error
}

func CreateFromKeysAPI(kapi client.KeysAPI, directory string) (*Etcd, error) {
	etcd := &Etcd{
		kapi:      kapi,
		directory: directory,
		err:       nil,
	}

	return etcd, nil
}

func CreateFromConfig(cfg *client.Config, directory string) (*Etcd, error) {
	c, err := client.New(*cfg)
	if err != nil {
		return nil, err
	}

	kapi := client.NewKeysAPI(c)
	_, err = kapi.Set(context.Background(), "/ping", "", nil)
	if err != nil {
		return nil, err
	}

	return CreateFromKeysAPI(kapi, directory)
}

func CreateFromEndpoint(etcdEndpoint string, directory string) (*Etcd, error) {
	cfg := &client.Config{
		Endpoints:               []string{etcdEndpoint},
		Transport:               client.DefaultTransport,
		HeaderTimeoutPerRequest: time.Second,
	}

	return CreateFromConfig(cfg, directory)
}

func (etcd *Etcd) Set(c context.Context, key string, value string) error {
	_, err := etcd.kapi.Set(c, key, value, nil)
	return err
}

func (etcd *Etcd) Delete(c context.Context, key string) error {
	_, err := etcd.kapi.Delete(c, key, &client.DeleteOptions{Recursive: true})
	return err
}

func (etcd *Etcd) Get(c context.Context, key string) (string, error) {
	r, err := etcd.kapi.Get(c, key, nil)
	if err != nil {
		return "", err
	}
	if r.Node == nil {
		return "", kvs.ErrNoSuchKey
	}
	return r.Node.Value, nil
}

func (etcd *Etcd) Next(c context.Context) (*kvs.Update, error) {
	if etcd.err != nil {
		// We had an error, just return it
		return nil, etcd.err
	}

	if etcd.watcher == nil {
		l, err := etcd.kapi.Get(c, etcd.directory, &client.GetOptions{Recursive: true})
		if err != nil {
			e := err.(client.Error)
			if e.Code != client.ErrorCodeKeyNotFound {
				etcd.err = err
				return nil, err
			}

			// In case etcd.directory, we still need to retrieve an index
			l, err := etcd.kapi.Get(c, "/", nil)
			if err != nil {
				etcd.err = err
				return nil, err
			}
			etcd.lastEtcdIndex = l.Index
		} else {
			etcd.listing = append(etcd.listing, l.Node)
			etcd.lastEtcdIndex = l.Index
		}

		etcd.watcher = etcd.kapi.Watcher(etcd.directory, &client.WatcherOptions{Recursive: true, AfterIndex: etcd.lastEtcdIndex})
	}

	for len(etcd.listing) != 0 {
		if etcd.listing[0].Dir {
			etcd.listing = append(etcd.listing, etcd.listing[0].Nodes...) // Append childrens
			etcd.listing = etcd.listing[1:]                               // Remove first
			continue
		} else {
			n := etcd.listing[0]
			etcd.listing = etcd.listing[1:]
			e := &kvs.Update{Key: n.Key, Value: &n.Value}
			return e, nil
		}
	}

	r, err := etcd.watcher.Next(c)
	if err != nil {
		etcd.err = err
		etcd.watcher = nil
		return nil, err
	}

	var prev *string = nil
	if r.PrevNode != nil {
		prev = &r.PrevNode.Value
	}

	var new *string = nil
	if r.Action != "delete" {
		new = &r.Node.Value
	}

	e := &kvs.Update{Key: r.Node.Key, Value: new, Previous: prev}
	return e, nil
}
