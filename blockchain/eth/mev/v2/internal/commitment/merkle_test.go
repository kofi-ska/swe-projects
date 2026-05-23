package commitment

import "testing"

func TestRootDeterministic(t *testing.T) {
	a := Root([]byte("a"), []byte("b"))
	b := Root([]byte("a"), []byte("b"))
	if a != b {
		t.Fatal("expected deterministic root")
	}
	if a == Root([]byte("b"), []byte("a")) {
		t.Fatal("expected order-sensitive root")
	}
}
