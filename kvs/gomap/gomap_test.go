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

package gomap

import (
	"context"
	"github.com/Oryon/kvsync/kvs"
	"testing"
	"time"
)

func testStringPointers(t *testing.T, name string, s1 *string, s2 *string) {
	if (s1 == nil) != (s2 == nil) {
		t.Errorf("Unexpected nil %s nil='%v' instead of nil='%v'", name, s1 == nil, s2 == nil)
	}
	if s1 == nil {
		return
	}
	if *s1 != *s2 {
		t.Errorf("Unexpected %s '%s' instead of '%s'", name, *s1, *s2)
	}
}

func testNext(t *testing.T, sync kvs.Sync, updates []kvs.Update) {
	for _, u := range updates {
		r, e := sync.Next(context.Background())
		if e != nil {
			t.Errorf("Next returned error: %v", e)
		}
		if r.Key != u.Key {
			t.Errorf("Unexpected key '%s' instead of '%s'", r.Key, u.Key)
		}
		testStringPointers(t, "value", r.Value, u.Value)
		testStringPointers(t, "previous", r.Previous, u.Previous)
	}
}

func TestInit(t *testing.T) {
	m := Create()
	if m.channel == nil {
		t.Errorf("nil channel")
	}
	if len(m.channel) != 0 {
		t.Errorf("non empty channel")
	}
	if m.gomap == nil {
		t.Errorf("nil map")
	}
	if len(m.gomap) != 0 {
		t.Errorf("non empty map")
	}

	ck := [4]string{"a", "b", "b", "a"}
	cv := [4]string{"1", "2", "3", "4"}

	expected := []kvs.Update{
		{Key: ck[0], Value: &cv[0], Previous: nil},
		{Key: ck[1], Value: &cv[1], Previous: nil},
		{Key: ck[2], Value: &cv[2], Previous: &cv[1]},
		{Key: ck[3], Value: &cv[3], Previous: &cv[0]},
	}

	for i := range ck {
		m.Set(context.Background(), ck[i], cv[i])
	}

	testNext(t, m, expected)

	m.Delete(context.Background(), "b")
	m.Delete(context.Background(), "a")

	expected = []kvs.Update{
		{Key: ck[2], Value: nil, Previous: &cv[2]},
		{Key: ck[3], Value: nil, Previous: &cv[3]},
	}

	testNext(t, m, expected)

	e := m.Delete(context.Background(), "a")
	if e == nil {
		t.Error("Should have returned error")
	}

	c, _ := context.WithTimeout(context.Background(), time.Microsecond)
	u, e := m.Next(c)
	if u != nil {
		t.Error("Update should be nil")
	}
	if e == nil {
		t.Error("Should have returned error")
	}
}
