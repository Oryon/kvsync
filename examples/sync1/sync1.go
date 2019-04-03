package main

import (
	"context"
	"fmt"
	"github.com/Oryon/kvsync/kvs"
	"github.com/Oryon/kvsync/kvs/gomap"
	"github.com/Oryon/kvsync/store"
	"github.com/Oryon/kvsync/sync"
)

// This is the state we are going to store in the KVS.
// It is defined as a golang structure.
// The actual keys are abstracted away.
// The data will be stored using key root /db/stored/here/.
type Data struct {

	// 'Nodes/{key}' means that each node is stored as JSON
	// blobs using keys '/db/stored/here/Nodes/{key}'
	Nodes map[string]Node `kvs:"Nodes/{key}"`

	// 'Edges/{key}/' means that each edge is stored with sub-keys:
	// - '/db/stored/here/Edges/{key}/node_id1'
	// - '/db/stored/here/Edges/{key}/node_id2'
	Edges map[string]Edge `kvs:"Edges/{key}/"`

	// This is just used to stop the synchronizing thread.
	// But also demonstrate the ability to set any gotype.
	QuitDemo bool
}

// Since the Node objects are stored as JSON blob,
// there is no need to specify any kvs encoding.
type Node struct {
	ID          string
	Name        string
	Description string
}

// Here, the kvs parameters are optional.
// If absent, 'NodeID1' and 'NodeID2' names
// would have been used instead of 'node_id1' and 'node_id2'
type Edge struct {
	NodeID1 string `kvs:"node_id1"`
	NodeID2 string `kvs:"node_id2"`
}

// This function creates two nodes, one edge, and then sets the 'QuitDemo' boolean.
func set(s kvs.Store) {
	c := context.Background()
	db := &Data{}

	// Creating the first node object
	n := Node{
		ID:          "100",
		Name:        "foobar",
		Description: "This is a nice node",
	}
	store.Set(s, c, db, "/db/stored/here/", n, "Nodes", "100")

	// Creating the second node object
	n = Node{
		ID:          "101",
		Name:        "barfoo",
		Description: "This is another nice node",
	}
	store.Set(s, c, db, "/db/stored/here/", n, "Nodes", "101")

	// Creating an edge
	store.Set(s, c, db, "/db/stored/here/", Edge{
		NodeID1: "100",
		NodeID2: "101",
	}, "Edges", "10")

	// Setting the QuitDemo boolean
	store.Set(s, c, db, "/db/stored/here/", true, "QuitDemo")
}

var stopTimeWheel = false

// This function is called when an object is modified.
func SyncCallback(e *sync.SyncEvent) error {
	var id string
	var e2 sync.SyncEvent

	// Change notifications must usually be routed depending on the type of change.
	// This is done by using Field (to go down a struct) and Value (to access a map) methods.
	// Those functions can be chained. Checking for error later will tell if
	// any of the steps failed.
	if e2 = e.Field("Nodes").Value(&id); e2.Error() == nil {
		// Here we know the change is a Node object in the Nodes map.
		// The key is stored in 'id'.

		// Current gets us the Node object
		c, _ := e2.Current()

		fmt.Printf("Modified Node with key %s: %v\n", id, c)

	} else if e2 = e.Field("Edges").Value(&id); e2.Error() == nil {
		// Here we know the change is an Edge object in the Nodes map.
		// The key is stored in 'id'.

		// Current gets us the Node object
		c, _ := e2.Current()

		// Note that, since the Edge object is stored as 2 different keys,
		// The callback will be called twice.

		fmt.Printf("Modified Edge with key %s: %v\n", id, c)

	} else if b, err := e.Field("QuitDemo").Bool(); err == nil {

		// Since QuitDemo is a boolean, we can use Bool() method to get the value
		// directly.
		stopTimeWheel = b

	}
	return nil
}

// This function registers the object to synchronize and then executes the timing wheel.
func synchronize(kvsync kvs.Sync) {
	c := context.Background()

	// This is the object to be synchronized
	db := Data{}

	// Creating a sync object from the provided lower-level kv sync.
	s := sync.Sync{
		Sync: kvsync,
	}

	// Registering callback for given object with given format
	s.SyncObject(sync.SyncObject{
		Callback: SyncCallback,
		Object:   &db,
		Format:   "/db/stored/here/",
	})

	// Time wheel to trigger callback notification for successive events
	for {
		e := s.Next(c)
		if e != nil {
			panic(e)
		}
		if stopTimeWheel {
			break
		}
	}

	fmt.Printf("Final DB state %v\n\n", &db)
}

func main() {

	// Here, a map[string]string is used as backing KV storage.
	// You could also use etcd (if you run an etcd server somewhere):
	// gm, _ := etcd.CreateFromEndpoint("http://localhost:2379/", "/")
	gm := gomap.Create()

	// Start set function in a different thread.
	// Here gm is provided as a kvs.Store interface.
	go set(gm)

	// Start sync function in this thread.
	// Here gm is provided as a kvs.Sync interface.
	synchronize(gm)

	fmt.Printf("Final KV state %v\n\n", gm.GetBackingMap())
}
