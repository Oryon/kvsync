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

package encoding

import (
	"fmt"
	"reflect"
	//"strings"
	"testing"
)

func failIfError(t *testing.T, err error) {
	if err != nil {
		fmt.Printf("FAIL::::: Error: %v\n", err)
		t.Errorf("Error: %v", err)
	}
}

func failIfNotError(t *testing.T, err error) {
	if err == nil {
		t.Errorf("Expected error")
	}
}

func failIfErrorDifferent(t *testing.T, err error, expected error) {
	if err != expected {
		fmt.Printf("FAIL::::: Error '%v' differs from expected '%v'\n", err, expected)
		t.Errorf("Error '%v' differs from expected '%v'", err, expected)
	}
}

type S1 struct {
	A int
	B string
	C float64
}

type S2 struct {
	A S1 `kvs:"custom"`
	B S1 `kvs:"sub/"`
}

func testEncode(t *testing.T, key string, obj interface{}, truth map[string]string, fields ...interface{}) {
	m, e := Encode(key, obj, fields...)
	if e != nil {
		fmt.Printf("FAIL::::: Encode returned %v\n", e)
		t.Errorf("Encode returned %v", e)
	}
	if !reflect.DeepEqual(truth, m) {
		fmt.Printf("FAIL::::: Incorrect return %v (should be %v)\n", m, truth)
		t.Errorf("Incorrect return %v (should be %v)", m, truth)
	}
}

func TestJSONMArshall(t *testing.T) {
	var c map[string]string

	o := S1{
		A: 1,
		B: "test",
		C: 3.3,
	}
	c = make(map[string]string)
	c["/here"] = "{\"A\":1,\"B\":\"test\",\"C\":3.3}"
	testEncode(t, "/here", &o, c)

	c = make(map[string]string)
	c["/here/A"] = "1"
	c["/here/B"] = "test"
	c["/here/C"] = "3.3"

	testEncode(t, "/here/", &o, c)
	testEncode(t, "/here/", o, c)

	o2 := S2{
		A: o,
		B: o,
	}

	c = make(map[string]string)
	c["/here/custom"] = "{\"A\":1,\"B\":\"test\",\"C\":3.3}"
	c["/here/sub/A"] = "1"
	c["/here/sub/B"] = "test"
	c["/here/sub/C"] = "3.3"

	testEncode(t, "/here/", &o2, c)
	testEncode(t, "/here/", o2, c)

	c = make(map[string]string)
	c["/here/sub/A"] = "1"
	c["/here/sub/B"] = "test"
	c["/here/sub/C"] = "3.3"
	testEncode(t, "/here/", &o2, c, "B")
	testEncode(t, "/here/", o2, c, "B")

	c = make(map[string]string)
	c["/here/custom"] = "{\"A\":1,\"B\":\"test\",\"C\":3.3}"
	testEncode(t, "/here/", &o2, c, "A")
	testEncode(t, "/here/", o2, c, "A")
}

type S3 struct {
	A map[string]string `kvs:"{key}/after"`
	B map[int]S1        `kvs:"prev/{key}/"`
	C map[string]string `kvs:"C/{key}/"`
}

func TestMap(t *testing.T) {
	o := S3{
		A: make(map[string]string),
		B: make(map[int]S1),
		C: make(map[string]string),
	}

	c := make(map[string]string)
	testEncode(t, "/here/", o, c, "A")
	testEncode(t, "/here/", o, c, "B")
	testEncode(t, "/here/", o, c)

	o.B[1] = S1{
		A: 4,
		B: "test2",
		C: 3.5,
	}

	testEncode(t, "/here/", o, c, "A")

	c["/here/prev/1/A"] = "4"
	c["/here/prev/1/B"] = "test2"
	c["/here/prev/1/C"] = "3.5"
	testEncode(t, "/here/", o, c, "B")

	o.B[4] = S1{
		A: 0,
		B: "test3",
		C: 0,
	}
	c["/here/prev/4/A"] = "0"
	c["/here/prev/4/B"] = "test3"
	c["/here/prev/4/C"] = "0"

	testEncode(t, "/here/", o, c, "B")
	testEncode(t, "/here/", o, c)

	o.A["nyu"] = "test6"
	c["/here/nyu/after"] = "test6"
	testEncode(t, "/here/", o, c)

}

type S4 struct {
	A int `kvs:"A"`
	B string
	C float64
}

type S5 struct {
	A S4             `kvs:"in/blob"`
	B S4             `kvs:"sub/path/"`
	C map[string]*S4 `kvs:"map1/{key}/in/here"`
	D map[int]*S4    `kvs:"map2/{key}/"`
}

func testFindByKeyResult(t *testing.T, o1 interface{}, fields1 []interface{}, o2 interface{}, fields2 []interface{}) {
	if o1 != o2 {
		fmt.Printf("FAIL::::: FindByKey returned '%v' instead of '%v'\n", o1, o2)
		t.Errorf("FindByKey returned '%v' instead of '%v'", o1, o2)
	}
	if !reflect.DeepEqual(fields1, fields2) {
		fmt.Printf("FAIL::::: FindByKey returned '%v' instead of '%v'\n", fields1, fields2)
		t.Errorf("FindByKey returned '%v' instead of '%v'", fields1, fields2)
	}
}

func TestFindByKey0(t *testing.T) {
	s4 := S4{
		A: 1,
		B: "nya",
		C: 1.2,
	}

	s := S5{
		A: s4,
		B: s4,
	}

	// S5.A in blob with root
	o, fields, err := FindByKey(&s, "root/", "root/in/blob")
	failIfError(t, err)
	testFindByKeyResult(t, o, fields, &s.A, []interface{}{"A"})

	o, fields, err = FindByKey(&s, "/root/", "/root/in/blob")
	failIfError(t, err)
	testFindByKeyResult(t, o, fields, &s.A, []interface{}{"A"})

	o, fields, err = FindByKey(&s, "root/", "root/in/blob/")
	failIfNotError(t, err)

	o, fields, err = FindByKey(&s, "root/", "root/in/blob/nya")
	failIfErrorDifferent(t, err, ErrFindPathPastObject)

	o, fields, err = FindByKey(&s, "root/", "rot/in/blob")
	failIfErrorDifferent(t, err, ErrFindPathNotFound)

	o, fields, err = FindByKey(&s, "root/", "root/in2/blob")
	failIfErrorDifferent(t, err, ErrFindPathNotFound)

	// S5.A in blob without root
	o, fields, err = FindByKey(&s, "", "in/blob")
	failIfError(t, err)
	testFindByKeyResult(t, o, fields, &s.A, []interface{}{"A"})

	o, fields, err = FindByKey(&s, "", "in2/blob")
	failIfErrorDifferent(t, err, ErrFindPathNotFound)

	o, fields, err = FindByKey(&s, "", "in/blob/")
	failIfErrorDifferent(t, err, ErrFindPathPastObject)

	// S5.B as a subpath without root
	o, fields, err = FindByKey(&s, "", "sub/path")
	failIfErrorDifferent(t, err, ErrFindKeyInvalid)

	o, fields, err = FindByKey(&s, "", "sub/path/")
	failIfError(t, err)
	testFindByKeyResult(t, o, fields, &s.B, []interface{}{"B"})

	o, fields, err = FindByKey(&s, "", "sub/")
	failIfErrorDifferent(t, err, ErrFindPathNotFound)

	o, fields, err = FindByKey(&s, "", "sub")
	failIfNotError(t, err)

	o, fields, err = FindByKey(&s, "", "sub/path/A")
	failIfError(t, err)
	testFindByKeyResult(t, o, fields, &s.B.A, []interface{}{"B", "A"})

	o, fields, err = FindByKey(&s, "", "sub/path/B")
	failIfError(t, err)
	testFindByKeyResult(t, o, fields, &s.B.B, []interface{}{"B", "B"})

	// S5.C as map of elements stored as blobs
	o, fields, err = FindByKey(&s, "", "map1/")
	failIfError(t, err)
	testFindByKeyResult(t, o, fields, &s.C, []interface{}{"C"})
	o, fields, err = FindByKey(&s, "", "map1/testkey")
	failIfErrorDifferent(t, err, ErrFindPathNotFound)

	o, fields, err = FindByKey(&s, "", "map1/testkey/")
	failIfErrorDifferent(t, err, ErrFindPathNotFound)

	o, fields, err = FindByKey(&s, "", "map1/testkey/nnn")
	failIfErrorDifferent(t, err, ErrFindPathNotFound)

	s.C = make(map[string]*S4)
	o, fields, err = FindByKey(&s, "", "map1/")
	failIfError(t, err)
	testFindByKeyResult(t, o, fields, &s.C, []interface{}{"C"})

	o, fields, err = FindByKey(&s, "", "map1/testkey")
	failIfErrorDifferent(t, err, ErrFindPathNotFound)
	o, fields, err = FindByKey(&s, "", "map1/testkey/")
	failIfErrorDifferent(t, err, ErrFindPathNotFound)

	o, fields, err = FindByKey(&s, "", "map1/testkey/nnn")
	failIfErrorDifferent(t, err, ErrFindPathNotFound)

	s.C["testkey"] = &s.A

	o, fields, err = FindByKey(&s, "", "map1/")
	failIfError(t, err)
	testFindByKeyResult(t, o, fields, &s.C, []interface{}{"C"})

	o, fields, err = FindByKey(&s, "", "map1/testkey")
	failIfErrorDifferent(t, err, ErrFindPathNotFound)

	o, fields, err = FindByKey(&s, "", "map1/testkey/")
	failIfErrorDifferent(t, err, ErrFindPathNotFound)

	o, fields, err = FindByKey(&s, "", "map1/testkey/in")
	failIfErrorDifferent(t, err, ErrFindPathNotFound)

	o, fields, err = FindByKey(&s, "", "map1/testkey/in/here")
	failIfError(t, err)
	testFindByKeyResult(t, o, fields, s.C["testkey"], []interface{}{"C", "testkey"})

	s.D = make(map[int]*S4)

	o, fields, err = FindByKey(&s, "", "map2/")
	failIfError(t, err)
	testFindByKeyResult(t, o, fields, &s.D, []interface{}{"D"})

	o, fields, err = FindByKey(&s, "", "map2/111")
	failIfErrorDifferent(t, err, ErrFindKeyInvalid)

	o, fields, err = FindByKey(&s, "", "map2/111/")
	failIfErrorDifferent(t, err, ErrFindKeyNotFound)

	o, fields, err = FindByKey(&s, "", "map2/111/nnn")
	failIfErrorDifferent(t, err, ErrFindPathNotFound)

	s.D[111] = &s.A

	o, fields, err = FindByKey(&s, "", "map2/111")
	failIfErrorDifferent(t, err, ErrFindKeyInvalid)

	o, fields, err = FindByKey(&s, "", "map2/111/")
	failIfError(t, err)
	testFindByKeyResult(t, o, fields, s.D[111], []interface{}{"D", 111})

	o, fields, err = FindByKey(&s, "", "map2/111/A")
	failIfError(t, err)
	testFindByKeyResult(t, o, fields, &s.D[111].A, []interface{}{"D", 111, "A"})

}

type S6 struct {
	IntPtrMap map[string]*int `kvs:"IntPtrMap/{key}"`
	IntMap    map[string]int  `kvs:"IntMap/{key}"`
	N         int
}

type S7 struct {
	I           int
	S6PtrMap    map[string]*S6 `kvs:"s6_ptr_map/{key}/sub/"`
	S6StructMap map[string]S6  `kvs:"s6_struct_map/{key}/sub/"`
}

func testUpdateKeyObject(t *testing.T, object interface{}, format string, keypath string, value string, path []interface{}) {
	rpath, err := UpdateKeyObject(object, format, keypath, value)
	if err != nil {
		t.Errorf("findByKey returned %v", err)
		return
	}

	if !reflect.DeepEqual(rpath, path) {
		t.Errorf("returned path is %v and should be %v", rpath, path)
	}
}

func TestUpdateKeyObject(t *testing.T) {
	s := S7{
		S6PtrMap:    make(map[string]*S6),
		S6StructMap: make(map[string]S6),
	}
	s.S6PtrMap["a"] = &S6{
		IntPtrMap: make(map[string]*int),
		IntMap:    make(map[string]int),
	}
	s.S6StructMap["a"] = S6{
		IntPtrMap: make(map[string]*int),
		IntMap:    make(map[string]int),
	}

	testUpdateKeyObject(t, &s, "", "I", "122", []interface{}{"I"})
	if s.I != 122 {
		t.Errorf("Error\n")
	}

	testUpdateKeyObject(t, &s, "", "s6_struct_map/a/sub/IntMap/b", "123", []interface{}{"S6StructMap", "a", "IntMap", "b"})
	if s.S6StructMap["a"].IntMap["b"] != 123 {
		t.Errorf("Error\n")
	}

	testUpdateKeyObject(t, &s, "", "s6_ptr_map/aa/sub/IntMap/bb", "124", []interface{}{"S6PtrMap", "aa", "IntMap", "bb"})
	if s.S6PtrMap["aa"].IntMap["bb"] != 124 {
		t.Errorf("Error\n")
	}

	testUpdateKeyObject(t, &s, "", "s6_ptr_map/aa/sub/IntMap/cc", "112", []interface{}{"S6PtrMap", "aa", "IntMap", "cc"})
	if s.S6PtrMap["aa"].IntMap["cc"] != 112 {
		t.Errorf("Error\n")
	}
	if s.S6PtrMap["aa"].IntMap["bb"] != 124 {
		t.Errorf("Error\n")
	}
	if s.S6StructMap["a"].IntMap["b"] != 123 {
		t.Errorf("Error\n")
	}

	testUpdateKeyObject(t, &s, "", "s6_ptr_map/ee/sub/N", "42", []interface{}{"S6PtrMap", "ee", "N"})
	if s.S6PtrMap["ee"].N != 42 {
		t.Errorf("Error\n")
	}
}

type S8 struct {
	A int `kvs:"A"`
	B string
	C float64
}

type S9 struct {
	A S8             `kvs:"in/blob"`
	B S8             `kvs:"sub/path/"`
	C map[string]*S8 `kvs:"map1/{key}/in/here"`
	D map[int]*S8    `kvs:"map2/{key}/"`
}

func testFindByField(t *testing.T, o interface{}, format string, fields []interface{}, ret_format string, expected error) interface{} {
	o, f, err := FindByFields(o, format, fields)
	if err != expected {
		fmt.Printf("FAIL::::: FindByFields error '%v' instead of '%v'\n", err, expected)
		t.Errorf("FindByFields error '%v' instead of '%v'", err, expected)
		return nil
	}
	if f != ret_format {
		fmt.Printf("FAIL::::: FindByFields returned format '%v' instead of '%v'\n", f, ret_format)
		t.Errorf("FindByFields returned format '%v' instead of '%v'\n", f, ret_format)
		return nil
	}
	return o
}

func TestFindByFieldsBasic(t *testing.T) {
	s := S9{}
	s.B.A = 1

	o := testFindByField(t, &s, "store/here/", []interface{}{"B", "A"}, "store/here/sub/path/A", nil)
	if *o.(*int) != 1 {
		t.Errorf("Invalid value")
	}

	s.A.A = 2
	o = testFindByField(t, &s, "store/here/", []interface{}{"A"}, "store/here/in/blob", nil)
	if o.(*S8).A != 2 {
		t.Errorf("Invalid value 2")
	}

	s.A.A = 3
	o = testFindByField(t, &s, "/store/here/", []interface{}{"A"}, "/store/here/in/blob", nil)
	if o.(*S8).A != 3 {
		t.Errorf("Invalid value 2")
	}

	testFindByField(t, &s, "store/here/", []interface{}{"A", "A"}, "", ErrFindPathPastObject)
	testFindByField(t, &s, "store/here/", []interface{}{"C", "key"}, "", ErrFindKeyNotFound)
	testFindByField(t, &s, "store/here/", []interface{}{"C", "key", "A"}, "", ErrFindPathPastObject)

	s.C = make(map[string]*S8)
	testFindByField(t, &s, "store/here/", []interface{}{"C", "key"}, "", ErrFindKeyNotFound)
	testFindByField(t, &s, "store/here/", []interface{}{"C", "key", "A"}, "", ErrFindPathPastObject)

	s.C["key"] = &S8{
		A: 1,
		B: "test",
	}
	testFindByField(t, &s, "store/here/", []interface{}{"C", "key", "A"}, "", ErrFindPathPastObject)
	o = testFindByField(t, &s, "store/here/", []interface{}{"C", "key"}, "store/here/map1/key/in/here", nil)
	if o.(*S8).A != 1 || o.(*S8).B != "test" {
		t.Errorf("Invalid value 3")
	}

	testFindByField(t, &s, "store/here/", []interface{}{"D", "key"}, "", ErrFindKeyWrongType)
	testFindByField(t, &s, "store/here/", []interface{}{"D", 1}, "", ErrFindKeyNotFound)

	s.D = make(map[int]*S8)
	testFindByField(t, &s, "store/here/", []interface{}{"D", 1}, "", ErrFindKeyNotFound)

	s.D[1] = &S8{
		A: 1,
		B: "test",
	}
	o = testFindByField(t, &s, "store/here/", []interface{}{"D", 1}, "store/here/map2/1/", nil)
	if o.(*S8).A != 1 || o.(*S8).B != "test" {
		t.Errorf("Invalid value 3")
	}

	o = testFindByField(t, &s, "store/here/", []interface{}{"D", 1, "A"}, "store/here/map2/1/A", nil)
	if *o.(*int) != 1 {
		t.Errorf("Invalid value 2")
	}

	o = testFindByField(t, &s, "store/here/", []interface{}{"D", 1, "B"}, "store/here/map2/1/B", nil)
	if *o.(*string) != "test" {
		t.Errorf("Invalid value 2")
	}

}

type S10 struct {
	A int
}

type S11 struct {
	M map[string]S10 `kvs:"M/{key}/"`
}

func TestSetByFields(t *testing.T) {

	s := S11{}

	err := SetByFields(&s, "/la/", 10, "M", "test", "A")
	if err != nil {
		t.Errorf("SetByFields error %v", err)
	}
	q, ok := s.M["test"]
	if !ok {
		t.Errorf("Could not find key")
	}
	if q.A != 10 {
		t.Errorf("Invalid value")
	}

	err = DeleteByFields(&s, "/la/", "M")
	if err != ErrNotMapIndex {
		t.Errorf("Cannot delete Map object")
	}

	err = DeleteByFields(&s, "/la/", "M", 10)
	if err != ErrFindKeyWrongType {
		t.Errorf("Cannot delete Map object")
	}

	err = DeleteByFields(&s, "/la/", "M", "test")
	if err != nil {
		t.Errorf("DeleteByFields error %v", err)
	}
	_, ok = s.M["test"]
	if ok {
		t.Errorf("Key should not exist")
	}
}
