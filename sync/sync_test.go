package sync

import (
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
