package testdata

import (
	"testing"
)

func TestFiles(t *testing.T) {
	s := New(t.TempDir())
	if names, _ := s.List("c", "p"); len(names) != 0 {
		t.Fatal("empty dir should list nothing")
	}
	if err := s.Put("c", "p", "a01.in", []byte("3 4")); err != nil {
		t.Fatal(err)
	}
	if err := s.Put("c", "p", "a01.out", []byte("7")); err != nil {
		t.Fatal(err)
	}
	names, _ := s.List("c", "p")
	if len(names) != 2 || names[0] != "a01.in" {
		t.Fatalf("sorted list: %v", names)
	}
	if b, _ := s.Get("c", "p", "a01.out"); string(b) != "7" {
		t.Fatal("round-trip broken")
	}
	if m, _ := s.Pairs("c", "p"); len(m) != 0 {
		t.Fatalf("pairs complete: %v", m)
	}
	_ = s.Put("c", "p", "b01.in", []byte("x"))
	if m, _ := s.Pairs("c", "p"); len(m) != 1 || m[0] != "b01.in" {
		t.Fatalf("missing pair: %v", m)
	}
	if err := s.Delete("c", "p", "b01.in"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("c", "p", "nope"); err == nil {
		t.Fatal("missing file should error")
	}
	for _, bad := range []string{"../x", "a/b", "", "a\\b"} {
		if err := s.Put("c", "p", bad, []byte("x")); err == nil {
			t.Fatalf("traversal %q must fail", bad)
		}
	}
	if err := s.Put("c", "p", "big.in", make([]byte, MaxFileSize+1)); err == nil {
		t.Fatal("oversize must fail")
	}
	if p, err := s.Path("c", "p", "a01.in"); err != nil || p == "" {
		t.Fatal("path helper")
	}
}
