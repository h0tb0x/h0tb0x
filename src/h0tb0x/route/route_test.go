package route

import (
	"h0tb0x/crypto"
	"testing"
)

func TestPushPop(t *testing.T) {
	var r Route
	sk1 := crypto.NewSymmetricKey()
	sk2 := crypto.NewSymmetricKey()
	r.Push(sk1, 1)
	r.Push(sk2, 2)
	r.Push(sk1, 3)
	x, e := r.Pop(sk2)
	if e == nil {
		t.Fatal("Invalid pop worked")
	}
	x, e = r.Pop(sk1)
	if x != 3 || e != nil {
		t.Fatal("Pop failed")
	}
	x, e = r.Pop(sk2)
	if x != 2 || e != nil {
		t.Fatal("Pop failed")
	}
	x, e = r.Pop(sk1)
	if x != 1 || e != nil {
		t.Fatal("Pop failed")
	}
	x, e = r.Pop(sk1)
	if e == nil {
		t.Fatal("Pop of empty list return no error")
	}
}

func TestLoopOptimize(t *testing.T) {
	var r Route
	sk1 := crypto.NewSymmetricKey()
	sk2 := crypto.NewSymmetricKey()
	sk3 := crypto.NewSymmetricKey()
	r.Push(sk1, 1)
	r.Push(sk2, 2)
	r.Push(sk1, 3)
	if r.HasSelf(sk3) {
		t.Fatal("Found self where none was there")
	}
	if !r.HasSelf(sk2) {
		t.Fatal("Self not found when should have been")
	}
	r.Optimize(sk2)
	if r.HasSelf(sk2) {
		t.Fatal("Found sk2 after optimize")
	}
	x, e := r.Pop(sk1)
	if x != 1 || e != nil {
		t.Fatal("Post-optimize pop failed")
	}
	if r.Length() != 0 {
		t.Fatal("Extra entries after optimize")
	}
}
