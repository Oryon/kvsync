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

package sync

import (
	"context"
	"fmt"
	"github.com/Oryon/kvsync/kvs/gomap"
	"testing"
	"time"
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

	err = gm.Set(context.Background(), "/o/map/sds/s1/A", "5")
	failIfError(t, err)

	lastEvent = nil
	c, _ := context.WithDeadline(context.Background(), time.Now().Add(time.Millisecond))
	s.Next(c)
	if lastEvent != nil {
		t.Errorf("There should not have been an event")
	}

	err = gm.Set(context.Background(), "/o/map/123/s1/A", "6")
	failIfError(t, err)

	lastEvent = nil
	s.Next(context.Background())
	if lastEvent == nil {
		t.Errorf("There should be an event")
	}

	intkey := 0
	if i, err := lastEvent.Field("M").Value(&intkey).Field("A").Int(); err == nil {
		if intkey != 123 {
			t.Errorf("Wrong key")
		} else if i != 6 {
			t.Errorf("Wrong value")
		}
	} else {
		t.Errorf("Returned: %v", err)
	}

	if i, err := lastEvent.Field("M").Value(nil).Field("A").Int(); err != nil {
		t.Errorf("Returned: %v", err)
		if i != 6 {
			t.Errorf("Wrong value")
		}
	}

}
