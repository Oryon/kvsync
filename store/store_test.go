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

package store

import (
	"context"
	"fmt"
	"github.com/Oryon/kvsync/encoding"
	"github.com/Oryon/kvsync/kvs/gomap"
	"reflect"
	"testing"
)

type S1 struct {
	A int
	B int
}

type S2 struct {
	S S1 `kvs:"S/"`
	B string
	M map[int]S1 `kvs:"map/{key}/s1/"`
}

func testStore(t *testing.T, gm *gomap.Gomap, obj interface{}, format string, truth map[string]string, err error, fields ...interface{}) {
	e := Store(gm, context.Background(), obj, format, fields...)
	if e != err {
		fmt.Printf("FAIL::::: Set returned %v\n", e)
		t.Errorf("Set returned %v", e)
	}
	if !reflect.DeepEqual(truth, gm.GetBackingMap()) {
		fmt.Printf("FAIL::::: Incorrect return %v (should be %v)\n", gm.GetBackingMap(), truth)
		t.Errorf("Incorrect return %v (should be %v)", gm.GetBackingMap(), truth)
	}
}

func testDelete(t *testing.T, gm *gomap.Gomap, obj interface{}, format string, truth map[string]string, err error, fields ...interface{}) {
	e := Delete(gm, context.Background(), obj, format, fields...)
	if e != err {
		fmt.Printf("FAIL::::: Set returned %v\n", e)
		t.Errorf("Set returned %v", e)
	}
	if !reflect.DeepEqual(truth, gm.GetBackingMap()) {
		fmt.Printf("FAIL::::: Incorrect return %v (should be %v)\n", gm.GetBackingMap(), truth)
		t.Errorf("Incorrect return %v (should be %v)", gm.GetBackingMap(), truth)
	}
}

func TestStore(t *testing.T) {
	gm := gomap.Create()
	st := S2{}

	m := make(map[string]string)

	m["/here/B"] = ""
	m["/here/S/A"] = "0"
	m["/here/S/B"] = "0"
	testStore(t, gm, &st, "/here/", m, nil)

	st.B = "test"
	testStore(t, gm, &st, "/here/", m, nil, "S")
	testStore(t, gm, &st, "/here/", m, nil, "M")

	m["/here/B"] = "test"
	testStore(t, gm, &st, "/here/", m, nil, "B")

	m["/here/B"] = "test"
	testStore(t, gm, &st, "/here/", m, nil, "B")

	testStore(t, gm, &st, "/here/", m, encoding.ErrFindObjectNotFound, "M", 2)
	testStore(t, gm, &st, "/here/", m, encoding.ErrFindKeyWrongType, "M", "nya")

	m["/here/S/A"] = "1"
	st.S.A = 1
	testStore(t, gm, &st, "/here/", m, nil, "S", "A")

	st.M = make(map[int]S1)
	st.M[2] = S1{123, 0}
	m["/here/map/2/s1/A"] = "123"
	testStore(t, gm, &st, "/here/", m, nil, "M", 2, "A")

	//m["/here"] = "{\"A\":1,\"B\":\"test\",\"C\":3.3}"
	//testEncode(t, "/here", &o, c)

	delete(m, "/here/map/2/s1/A")
	testDelete(t, gm, &st, "/here/", m, nil, "M", 2)
}

func testSet(t *testing.T, gm *gomap.Gomap, obj interface{}, format string, val interface{}, truth map[string]string, err error, fields ...interface{}) {
	e := Set(gm, context.Background(), obj, format, val, fields...)
	if e != err {
		fmt.Printf("FAIL::::: Set returned %v\n", e)
		t.Errorf("Set returned %v", e)
	}
	if !reflect.DeepEqual(truth, gm.GetBackingMap()) {
		fmt.Printf("FAIL::::: Incorrect return %v (should be %v)\n", gm.GetBackingMap(), truth)
		t.Errorf("Incorrect return %v (should be %v)", gm.GetBackingMap(), truth)
	}
}

func TestSet(t *testing.T) {
	gm := gomap.Create()
	st := S2{}

	setter := S2{}

	m := make(map[string]string)

	m["/here/B"] = ""
	m["/here/S/A"] = "0"
	m["/here/S/B"] = "0"
	testSet(t, gm, &st, "/here/", setter, m, nil)

	setter.B = "test"
	m["/here/B"] = "test"
	testSet(t, gm, &st, "/here/", setter, m, nil)

	m["/here/B"] = "test2"
	testSet(t, gm, &st, "/here/", "test2", m, nil, "B")
	testSet(t, gm, &st, "/here/", 10, m, encoding.ErrFindSetWrongType, "B")

	setter1 := S1{A: 10}
	m["/here/S/A"] = "10"
	testSet(t, gm, &st, "/here/", setter1, m, nil, "S")
	testSet(t, gm, &st, "/here/", 10, m, encoding.ErrFindSetWrongType, "S")

	m["/here/S/A"] = "12"
	testSet(t, gm, &st, "/here/", 12, m, nil, "S", "A")
	testSet(t, gm, &st, "/here/", "str", m, encoding.ErrFindSetWrongType, "S", "A")

	m["/here/map/2/s1/A"] = "14"
	testSet(t, gm, &st, "/here/", 14, m, nil, "M", 2, "A")
	testSet(t, gm, &st, "/here/", "str", m, encoding.ErrFindSetWrongType, "M", 2, "A")
}
