package transfer

import (
	"bytes"
	"fmt"
	"io"
	"testing"
)

type WeirdString string

func (this *WeirdString) Encode(out io.Writer) error {
	fmt.Printf("Doing special encode of: %s\n", *this)
	return Encode(out, ":"+string(*this)+":")

}

func (this *WeirdString) Decode(in io.Reader) error {
	fmt.Printf("Doing special decode\n")
	var orig string
	err := Decode(in, &orig)
	*this = WeirdString(orig)
	return err

}

type StructB struct {
	list []int
	Arr  [2]int
	Ws   WeirdString
}

type StructA struct {
	Foo  uint
	Bar  int
	Baz  string
	B    *StructB
	Amap map[int]int
}

func TestSync(t *testing.T) {
	x := &StructA{
		Foo: 8675309,
		Bar: -123456,
		Baz: "Hello",
		B: &StructB{
			list: []int{1, 2, 3, 4, 5},
			Arr:  [2]int{-1, -2},
			Ws:   "World",
		},
		Amap: make(map[int]int),
	}
	x.Amap[5] = 6
	x.Amap[1] = 2
	var y *StructA
	var buf bytes.Buffer
	fmt.Printf("About to encode\n")
	err := Encode(&buf, x)
	if err != nil {
		fmt.Printf("Err: %s\n", err)
	}
	fmt.Printf("buf: %v\n", buf.Bytes())
	err = Decode(&buf, &y)
	if err != nil {
		t.Fatalf("Err: %s\n", err)
	}
	fmt.Printf("y: %v\n", y)
	//fmt.Printf("y.Amap: %v\n", y.Amap)
	fmt.Printf("y.B: %v\n", y.B)
}
