package etcd

import (
	"context"
	"github.com/Oryon/kvsync/kvs"
	"go.etcd.io/etcd/client"
	"time"
)

type Etcd struct {
	directory        string
	kapi             client.KeysAPI
	listing          *client.Response
	nextListingIndex int
	lastEtcdIndex    uint64
	watcher          client.Watcher
	err              error
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
	_, err := etcd.kapi.Delete(c, key, nil)
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
		l, err := etcd.kapi.Get(c, etcd.directory, nil)
		etcd.listing = l

		if err != nil {
			etcd.err = err
			return nil, err
		}

		etcd.watcher = etcd.kapi.Watcher(etcd.directory, &client.WatcherOptions{Recursive: true, AfterIndex: etcd.lastEtcdIndex})

		etcd.lastEtcdIndex = etcd.listing.Index
	}

	if etcd.listing != nil {
		if etcd.listing.Node != nil && etcd.nextListingIndex < len(etcd.listing.Node.Nodes) {
			n := etcd.listing.Node.Nodes[etcd.nextListingIndex]
			e := &kvs.Update{Key: n.Key, Value: &n.Value}
			etcd.nextListingIndex += 1
			return e, nil
		} else {
			etcd.listing = nil
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
