package sync

import (
	"context"
	"fmt"
	"github.com/Oryon/kvsync/kvs/gomap"
	"testing"
)

func failIfError(t *testing.T, err error) {
	if err != nil {
		fmt.Printf("FAIL::::: Error: %v\n", err)
		t.Errorf("Error: %v", err)
	}
}

func failIfErrorDifferent(t *testing.T, err error, expected error) {
	if err != expected {
		fmt.Printf("FAIL::::: Error '%v' differs from expected '%v'\n", err, expected)
		t.Errorf("Error '%v' differs from expected '%v'", err, expected)
	}
}

func failIfNotError(t *testing.T, err error) {
	if err == nil {
		t.Errorf("Expected error")
	}
}

func TestBasicSyncUnSync(t *testing.T) {
	gm := gomap.Create()

	s := Sync{
		Sync: gm,
	}

	var err error

	err = s.SyncObject(SyncObject{
		Format: "/test/key",
	})
	failIfError(t, err)

	err = s.SyncObject(SyncObject{
		Format: "/test/key2",
	})
	failIfError(t, err)

	err = s.UnsyncObject("/test/key3")
	failIfNotError(t, err)

	err = s.UnsyncObject("/test/key2")
	failIfError(t, err)

	err = s.UnsyncObject("/test/key2")
	failIfNotError(t, err)

	err = s.SyncObject(SyncObject{
		Format: "/test/key2",
	})
	failIfError(t, err)
}

type S1 struct {
	A int
}

type S2 struct {
	S S1 `kvs:"S/"`
	B string
	M map[int]S1 `kvs:"map/{key}/s1/"`
}

var lastEvent *SyncEvent

func expectSyncEventCB(e *SyncEvent) error {
	fmt.Printf("event %v\n", e)
	lastEvent = e
	return nil
}

func TestBasicNext(t *testing.T) {
	gm := gomap.Create()

	s := Sync{
		Sync: gm,
	}

	st := S2{}

	var err error

	err = s.SyncObject(SyncObject{
		Format:   "/o/",
		Object:   &st,
		Callback: expectSyncEventCB,
	})
	failIfError(t, err)

	err = gm.Set(context.Background(), "/o/B", "nya")
	failIfError(t, err)

	lastEvent = nil
	s.Next(context.Background())
	if str, err := lastEvent.Field("B").String(); err == nil {
		if str != "nya" {
			t.Errorf("Wrong value")
		}
	} else {
		t.Errorf("Returned %v", err)
	}

	err = gm.Set(context.Background(), "/o/S/A", "5")
	failIfError(t, err)

	lastEvent = nil
	s.Next(context.Background())
	if i, err := lastEvent.Field("S").Field("A").Int(); err == nil {
		if i != 5 {
			t.Errorf("Wrong value")
		}
	} else {
		t.Errorf("Returned %v", err)
	}
}
