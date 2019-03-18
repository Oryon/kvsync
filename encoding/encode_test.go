package encoding

import (
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
