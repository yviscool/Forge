package arena

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPContestLifecycle(t *testing.T) {
	srv := httptest.NewServer(NewServer(NewService()))
	defer srv.Close()
	r, err := http.Post(srv.URL+"/api/contests", "application/json", bytes.NewBufferString(`{"name":"Demo"}`))
	if err != nil {
		t.Fatal(err)
	}
	if r.StatusCode != 201 {
		t.Fatalf("status %d", r.StatusCode)
	}
	page, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	if page.StatusCode != 200 {
		t.Fatalf("page %d", page.StatusCode)
	}
	asset, err := http.Get(srv.URL + "/web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	if asset.StatusCode != 200 {
		t.Fatalf("asset %d", asset.StatusCode)
	}
}
