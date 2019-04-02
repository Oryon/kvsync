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

func TestStore(t *testing.T) {
	gm := gomap.Create()
	st := S2{}

	m := make(map[string]string)

	m["/here/B"] = ""
	m["/here/S/A"] = "0"
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
	st.M[2] = S1{123}
	m["/here/map/2/s1/A"] = "123"
	testStore(t, gm, &st, "/here/", m, nil, "M", 2, "A")

	//m["/here"] = "{\"A\":1,\"B\":\"test\",\"C\":3.3}"
	//testEncode(t, "/here", &o, c)

}