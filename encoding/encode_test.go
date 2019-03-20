package encoding

import (
	//"fmt"
	"reflect"
	"testing"
)

type S1 struct {
	A int
	B string
	C float64
}

type S2 struct {
	A S1 `kvs:"custom"`
	B S1 `kvs:"sub/"`
}

func TestBasic(t *testing.T) {
	_, e := Encode("here", 1)
	if e != ErrFirstSlash {
		t.Errorf("Incorrect error returned")
	}
}

func testEncode(t *testing.T, key string, obj interface{}, truth map[string]string, fields ...interface{}) {
	m, e := Encode(key, obj, fields...)
	if e != nil {
		t.Errorf("Encode returned %v", e)
	}
	if !reflect.DeepEqual(truth, m) {
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
	c["/here/B"] = "\"test\""
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
	c["/here/sub/B"] = "\"test\""
	c["/here/sub/C"] = "3.3"

	testEncode(t, "/here/", &o2, c)
	testEncode(t, "/here/", o2, c)

	c = make(map[string]string)
	c["/here/sub/A"] = "1"
	c["/here/sub/B"] = "\"test\""
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
	C map[string]string `kvs:"C/"`
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
	c["/here/prev/1/B"] = "\"test2\""
	c["/here/prev/1/C"] = "3.5"

	testEncode(t, "/here/", o, c, "B")

	o.B[4] = S1{
		A: 0,
		B: "test3",
		C: 0,
	}
	c["/here/prev/4/A"] = "0"
	c["/here/prev/4/B"] = "\"test3\""
	c["/here/prev/4/C"] = "0"

	testEncode(t, "/here/", o, c, "B")
	testEncode(t, "/here/", o, c)

	o.A["nyu"] = "test6"
	c["/here/\"nyu\"/after"] = "\"test6\""
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
		t.Errorf("FindByKey returned '%v' instead of '%v'", o1, o2)
	}
	if !reflect.DeepEqual(fields1, fields2) {
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

	o, fields, err = FindByKey(&s, "root/", "root/in/blob/")
	failIfNotError(t, err)

	o, fields, err = FindByKey(&s, "root/", "root/in/blob/nya")
	failIfErrorDifferent(t, err, errFindPathPastObject)

	o, fields, err = FindByKey(&s, "root/", "rot/in/blob")
	failIfErrorDifferent(t, err, errFindPathNotFound)

	o, fields, err = FindByKey(&s, "root/", "root/in2/blob")
	failIfErrorDifferent(t, err, errFindPathNotFound)

	// S5.A in blob without root
	o, fields, err = FindByKey(&s, "", "in/blob")
	failIfError(t, err)
	testFindByKeyResult(t, o, fields, &s.A, []interface{}{"A"})

	o, fields, err = FindByKey(&s, "", "in2/blob")
	failIfErrorDifferent(t, err, errFindPathNotFound)

	o, fields, err = FindByKey(&s, "", "in/blob/")
	failIfErrorDifferent(t, err, errFindPathPastObject)

	// S5.B as a subpath without root
	o, fields, err = FindByKey(&s, "", "sub/path")
	failIfError(t, err)
	testFindByKeyResult(t, o, fields, &s.B, []interface{}{"B"})

	o, fields, err = FindByKey(&s, "", "sub/path/")
	failIfError(t, err)
	testFindByKeyResult(t, o, fields, &s.B, []interface{}{"B"})

	o, fields, err = FindByKey(&s, "", "sub/")
	failIfErrorDifferent(t, err, errFindPathNotFound)

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

	o, fields, err = FindByKey(&s, "", "map1/\"testkey\"")
	failIfErrorDifferent(t, err, errFindKeyNotFound)

	o, fields, err = FindByKey(&s, "", "map1/\"testkey\"/")
	failIfErrorDifferent(t, err, errFindKeyNotFound)

	o, fields, err = FindByKey(&s, "", "map1/\"testkey\"/nnn")
	failIfErrorDifferent(t, err, errFindKeyNotFound)

	s.C = make(map[string]*S4)

	o, fields, err = FindByKey(&s, "", "map1/")
	failIfError(t, err)
	testFindByKeyResult(t, o, fields, &s.C, []interface{}{"C"})

	o, fields, err = FindByKey(&s, "", "map1/\"testkey\"")
	failIfErrorDifferent(t, err, errFindKeyNotFound)

	o, fields, err = FindByKey(&s, "", "map1/\"testkey\"/")
	failIfErrorDifferent(t, err, errFindKeyNotFound)

	o, fields, err = FindByKey(&s, "", "map1/\"testkey\"/nnn")
	failIfErrorDifferent(t, err, errFindKeyNotFound)

	s.C["testkey"] = &s.A

	o, fields, err = FindByKey(&s, "", "map1/")
	failIfError(t, err)
	testFindByKeyResult(t, o, fields, &s.C, []interface{}{"C"})

	o, fields, err = FindByKey(&s, "", "map1/\"testkey\"")
	failIfErrorDifferent(t, err, errFindPathNotFound)

	o, fields, err = FindByKey(&s, "", "map1/\"testkey\"/")
	failIfErrorDifferent(t, err, errFindPathNotFound)

	o, fields, err = FindByKey(&s, "", "map1/\"testkey\"/in")
	failIfErrorDifferent(t, err, errFindPathNotFound)

	o, fields, err = FindByKey(&s, "", "map1/\"testkey\"/in/here")
	failIfError(t, err)
	testFindByKeyResult(t, o, fields, s.C["testkey"], []interface{}{"C", "testkey"})

	s.D = make(map[int]*S4)

	o, fields, err = FindByKey(&s, "", "map2/")
	failIfError(t, err)
	testFindByKeyResult(t, o, fields, &s.D, []interface{}{"D"})

	o, fields, err = FindByKey(&s, "", "map2/111")
	failIfErrorDifferent(t, err, errFindKeyNotFound)

	o, fields, err = FindByKey(&s, "", "map2/111/")
	failIfErrorDifferent(t, err, errFindKeyNotFound)

	o, fields, err = FindByKey(&s, "", "map2/111/nnn")
	failIfErrorDifferent(t, err, errFindKeyNotFound)

	s.D[111] = &s.A

	o, fields, err = FindByKey(&s, "", "map2/111")
	failIfError(t, err)
	testFindByKeyResult(t, o, fields, s.D[111], []interface{}{"D", 111})

	o, fields, err = FindByKey(&s, "", "map2/111/")
	failIfError(t, err)
	testFindByKeyResult(t, o, fields, s.D[111], []interface{}{"D", 111})

	o, fields, err = FindByKey(&s, "", "map2/111/A")
	failIfError(t, err)
	testFindByKeyResult(t, o, fields, &s.D[111].A, []interface{}{"D", 111, "A"})

}
