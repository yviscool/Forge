package realtime

import (
	"testing"

	"github.com/yviscool/forge/internal/domain"
)

func TestHubFilter(t *testing.T) {
	h := NewHub()
	chAll, un1 := h.Subscribe("")
	defer un1()
	chC1, un2 := h.Subscribe("c1")
	defer un2()
	h.Publish(domain.Event{Type: "x", ContestID: "c1"})
	if e := <-chAll; e.ContestID != "c1" {
		t.Fatal("broadcast fan-out broken")
	}
	if e := <-chC1; e.ContestID != "c1" {
		t.Fatal("filtered sub should receive")
	}
	h.Publish(domain.Event{Type: "x", ContestID: "c2"})
	if e := <-chAll; e.ContestID != "c2" {
		t.Fatal("all-sub should receive c2")
	}
	select {
	case e := <-chC1:
		t.Fatalf("filtered sub leaked: %#v", e)
	default:
	}
}
