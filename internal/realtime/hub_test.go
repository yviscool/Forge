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
	if got := h.SubscriberCount(); got != 2 {
		t.Fatalf("subscriber count = %d, want 2", got)
	}
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

func TestHubSignalsOverflowInsteadOfSilentlyDropping(t *testing.T) {
	h := NewHub()
	ch, cancel := h.Subscribe("c1")
	defer cancel()

	for i := 0; i < 32; i++ {
		h.Publish(domain.Event{Type: "queued", ContestID: "c1"})
	}
	h.Publish(domain.Event{Type: "newest", ContestID: "c1"})

	var sawOverflow bool
	for i := 0; i < 32; i++ {
		event := <-ch
		if event.Type == "stream.overflow" {
			sawOverflow = true
			if event.ContestID != "c1" || event.Data == nil {
				t.Fatalf("overflow marker is not actionable: %#v", event)
			}
		}
	}
	if !sawOverflow {
		t.Fatal("slow subscriber must receive an explicit overflow marker")
	}
}
