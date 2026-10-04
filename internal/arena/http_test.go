package arena

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPContestLifecycle(t *testing.T) {
	srv := httptest.NewServer(NewServer(NewService()))
	defer srv.Close()

	// 1. Create Contest
	r, err := http.Post(srv.URL+"/api/contests", "application/json", bytes.NewBufferString(`{"name":"Demo Contest","description":"Testing"}`))
	if err != nil {
		t.Fatal(err)
	}
	if r.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", r.StatusCode)
	}

	// 2. Query Contests
	listResp, err := http.Get(srv.URL + "/api/contests")
	if err != nil {
		t.Fatal(err)
	}
	if listResp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", listResp.StatusCode)
	}

	// 3. Create User
	uResp, err := http.Post(srv.URL+"/api/users", "application/json", bytes.NewBufferString(`{"name":"Alice","role":"student"}`))
	if err != nil {
		t.Fatal(err)
	}
	if uResp.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", uResp.StatusCode)
	}

	// 4. Create Group
	gResp, err := http.Post(srv.URL+"/api/groups", "application/json", bytes.NewBufferString(`{"name":"Class 1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if gResp.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", gResp.StatusCode)
	}

	// 5. Add Problem to Contest
	pResp, err := http.Post(srv.URL+"/api/contests/cnt-0001/problems", "application/json", bytes.NewBufferString(`{
		"code": "A",
		"title": "Sum",
		"statement": "Calculate a+b",
		"input": "Two integers a and b",
		"output": "One integer",
		"constraints": "1 <= a,b <= 1000",
		"examples": "1 2\n3"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if pResp.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", pResp.StatusCode)
	}
	var createdProb Problem
	if err := json.NewDecoder(pResp.Body).Decode(&createdProb); err != nil {
		t.Fatal(err)
	}

	// 6. Validate Problem
	vResp, err := http.Post(srv.URL+"/api/contests/cnt-0001/problems/"+createdProb.ID+"/validate", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	if vResp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", vResp.StatusCode)
	}

	// 7. Export Problem as CCF HTML
	expResp, err := http.Get(srv.URL + "/api/contests/cnt-0001/problems/" + createdProb.ID + "/export")
	if err != nil {
		t.Fatal(err)
	}
	if expResp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", expResp.StatusCode)
	}
	expBody, _ := io.ReadAll(expResp.Body)
	if !strings.Contains(string(expBody), "【题目描述】") || !strings.Contains(string(expBody), "A4") {
		t.Fatalf("export HTML missing expected CCF format: %s", string(expBody))
	}

	// 8. i18n endpoint
	i18nResp, err := http.Get(srv.URL + "/api/i18n")
	if err != nil {
		t.Fatal(err)
	}
	if i18nResp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", i18nResp.StatusCode)
	}

	// 9. Static pages & assets
	page, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	if page.StatusCode != 200 {
		t.Fatalf("page %d", page.StatusCode)
	}

	teacherPage, err := http.Get(srv.URL + "/teacher")
	if err != nil {
		t.Fatal(err)
	}
	if teacherPage.StatusCode != 200 {
		t.Fatalf("teacher page %d", teacherPage.StatusCode)
	}

	asset, err := http.Get(srv.URL + "/web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	if asset.StatusCode != 200 {
		t.Fatalf("asset %d", asset.StatusCode)
	}
}
