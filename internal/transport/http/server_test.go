package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yviscool/forge/internal/adapters/store/memory"
	"github.com/yviscool/forge/internal/app"
	"github.com/yviscool/forge/internal/realtime"
)

func newTestServer() *httptest.Server {
	svc := app.NewService(memory.New(), realtime.NewHub(), nil, nil)
	return httptest.NewServer(NewServer(svc, nil))
}

func post(t *testing.T, url, body string) (int, map[string]any) {
	t.Helper()
	var r *http.Response
	var err error
	if body == "" {
		r, err = http.Post(url, "application/json", nil)
	} else {
		r, err = http.Post(url, "application/json", bytes.NewBufferString(body))
	}
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(r.Body).Decode(&out)
	return r.StatusCode, out
}

func TestV1ContestLifecycle(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	if code, _ := post(t, srv.URL+"/api/v1/contests", `{"name":""}`); code != 400 {
		t.Fatalf("empty name should 400, got %d", code)
	}
	code, c := post(t, srv.URL+"/api/v1/contests", `{"name":"CSP 模拟"}`)
	if code != 201 {
		t.Fatalf("create %d", code)
	}
	cid := c["id"].(string)

	// user + problem + start + submit + judge + ranking
	_, u := post(t, srv.URL+"/api/v1/users", `{"name":"Alice","role":"student"}`)
	uid := u["id"].(string)
	code, p := post(t, srv.URL+"/api/v1/contests/"+cid+"/problems",
		`{"code":"A","title":"和","statement":"a+b","input":"a b","output":"s","constraints":"k"}`)
	if code != 201 {
		t.Fatalf("create problem %d %v", code, p)
	}
	pid := p["id"].(string)

	if sc, _ := post(t, srv.URL+"/api/v1/contests/"+cid+"/submissions",
		`{"problemID":"`+pid+`","userId":"`+uid+`","language":"cpp","code":"x"}`); sc != 400 {
		t.Fatalf("draft submit should 400, got %d", sc)
	}
	if sc, _ := post(t, srv.URL+"/api/v1/contests/"+cid+"/start", ``); sc != 200 {
		t.Fatalf("start %d", sc)
	}
	sc, sub := post(t, srv.URL+"/api/v1/contests/"+cid+"/submissions",
		`{"problemID":"`+pid+`","userId":"`+uid+`","language":"cpp","code":"x"}`)
	if sc != 202 {
		t.Fatalf("submit %d %v", sc, sub)
	}
	sid := sub["id"].(string)
	if sc, _ := post(t, srv.URL+"/api/v1/submissions/"+sid+"/judge", `{"verdict":"accepted","score":100}`); sc != 200 {
		t.Fatalf("judge %d", sc)
	}

	r, err := http.Get(srv.URL + "/api/v1/contests/" + cid + "/ranking")
	if err != nil || r.StatusCode != 200 {
		t.Fatal("ranking failed")
	}
	var ranks []map[string]any
	_ = json.NewDecoder(r.Body).Decode(&ranks)
	r.Body.Close()
	if len(ranks) != 1 || ranks[0]["score"].(float64) != 100 {
		t.Fatalf("ranking wrong: %v", ranks)
	}

	// validate endpoint
	if sc, _ := post(t, srv.URL+"/api/v1/contests/"+cid+"/problems/"+pid+"/validate", ``); sc != 200 {
		t.Fatalf("validate %d", sc)
	}

	// health + i18n must be 200; /app is 200 when built, else 404 with rebuild hint.
	for _, p := range []string{"/healthz", "/readyz", "/api/v1/i18n"} {
		rr, err := http.Get(srv.URL + p)
		if err != nil || rr.StatusCode != 200 {
			t.Fatalf("%s -> %v %v", p, rr, err)
		}
		rr.Body.Close()
	}
	rr, err := http.Get(srv.URL + "/app")
	if err != nil {
		t.Fatal(err)
	}
	defer rr.Body.Close()
	if rr.StatusCode == 200 {
		return // built bundle served
	}
	if rr.StatusCode != 404 {
		t.Fatalf("/app -> %d, want 200 or 404", rr.StatusCode)
	}
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(rr.Body)
	if !bytes.Contains(buf.Bytes(), []byte("npm run build")) {
		t.Fatalf("/app 404 should hint rebuild, got %q", buf.String())
	}

	// restricted contest rejects outsider, allows after binding
	_, c2 := post(t, srv.URL+"/api/v1/contests", `{"name":"邀请赛"}`)
	cid2 := c2["id"].(string)
	_, p2 := post(t, srv.URL+"/api/v1/contests/"+cid2+"/problems",
		`{"code":"A","title":"T","statement":"s","input":"i","output":"o","constraints":"k"}`)
	pid2 := p2["id"].(string)
	_, u2 := post(t, srv.URL+"/api/v1/users", `{"name":"Bob"}`)
	uid2 := u2["id"].(string)
	_, g := post(t, srv.URL+"/api/v1/groups", `{"name":"集训A"}`)
	post(t, srv.URL+"/api/v1/contests/"+cid2+"/start", ``)
	if sc, _ := post(t, srv.URL+"/api/v1/contests/"+cid2+"/submissions",
		`{"problemID":"`+pid2+`","userId":"`+uid2+`","language":"cpp","code":"x"}`); sc == 202 {
		// open contest allows all: bind a group first to make it restricted
		post(t, srv.URL+"/api/v1/contests/"+cid2+"/groups", `{"groupId":"`+g["id"].(string)+`"}`)
		if sc2, _ := post(t, srv.URL+"/api/v1/contests/"+cid2+"/submissions",
			`{"problemID":"`+pid2+`","userId":"`+uid2+`","language":"cpp","code":"x"}`); sc2 == 202 {
			t.Fatal("outsider should be rejected after group binding")
		}
		post(t, srv.URL+"/api/v1/contests/"+cid2+"/participants", `{"userId":"`+uid2+`"}`)
		if sc3, _ := post(t, srv.URL+"/api/v1/contests/"+cid2+"/submissions",
			`{"problemID":"`+pid2+`","userId":"`+uid2+`","language":"cpp","code":"x"}`); sc3 != 202 {
			t.Fatalf("bound user should submit, got %d", sc3)
		}
	}
}
