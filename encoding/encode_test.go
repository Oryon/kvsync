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

func testEncode(t *testing.T, key string, obj interface{}, truth map[string]string) {
	m, e := Encode(key, obj)
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
}
