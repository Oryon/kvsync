package encoding

import (
	"github.com/Oryon/kvsync/kvs/gomap"
	"testing"
)

func failIfError(t *testing.T, err error) {
	if err != nil {
		t.Errorf("Error: %v", err)
	}
}

func failIfErrorDifferent(t *testing.T, err error, expected error) {
	if err != expected {
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
		Key: "/test/key",
	})
	failIfError(t, err)

	err = s.SyncObject(SyncObject{
		Key: "/test/key2",
	})
	failIfError(t, err)

	err = s.UnsyncObject("/test/key3")
	failIfNotError(t, err)

	err = s.UnsyncObject("/test/key2")
	failIfError(t, err)

	err = s.UnsyncObject("/test/key2")
	failIfNotError(t, err)

	err = s.SyncObject(SyncObject{
		Key: "/test/key2",
	})
	failIfError(t, err)
}
