package pdf

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yviscool/forge/internal/domain"
)

func TestCappedBufferTruncatesBrowserOutput(t *testing.T) {
	buf := &cappedBuffer{limit: 5}
	n, err := buf.Write([]byte("abcdefgh"))
	if err != nil || n != 8 {
		t.Fatalf("writer contract: n=%d err=%v", n, err)
	}
	if got := buf.String(); !strings.Contains(got, "abcde") ||
		!strings.Contains(got, "output truncated") {
		t.Fatalf("unexpected bounded output: %q", got)
	}
}

func TestBrowserRenderHonorsContextWhileWaitingForSlot(t *testing.T) {
	for i := 0; i < maxConcurrentPDFRenders; i++ {
		pdfRenderSlots <- struct{}{}
	}
	defer func() {
		for i := 0; i < maxConcurrentPDFRenders; i++ {
			<-pdfRenderSlots
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := runBrowserPDF(ctx, "unused-browser", "forge_test_*", "in.html", "out.pdf",
		[]byte("<html></html>"))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("queued render must fail on request cancellation, got %v", err)
	}
}

func TestRenderHTMLUsesConfiguredProblem(t *testing.T) {
	html, err := RenderHTML(
		domain.Contest{Name: "Round One"},
		domain.Problem{Code: "A", Title: "Sum", Statement: "Add two numbers."},
	)
	if err != nil {
		t.Fatal(err)
	}
	text := string(html)
	for _, want := range []string{"Round One", "Sum", "Add two numbers."} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered HTML missing %q", want)
		}
	}
}
