package httpapi

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/yviscool/forge/internal/testdata"
)

func makeTestPolygonZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, _ := zw.Create("problem.xml")
	_, _ = f.Write([]byte(`<problem>
		<names><name language="chinese" value="题包导入测试"/></names>
		<judging><testset><time-limit>1000</time-limit><memory-limit>268435456</memory-limit></testset></judging>
	</problem>`))
	f2, _ := zw.Create("tests/01")
	_, _ = f2.Write([]byte("1 2"))
	f3, _ := zw.Create("tests/01.a")
	_, _ = f3.Write([]byte("3"))
	_ = zw.Close()
	return buf.Bytes()
}

func postRawBytes(t *testing.T, target, token string, body []byte) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/zip")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, respBody
}

func TestImportAndExportProblemPackage(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	e.srv.Files = testdata.New(t.TempDir())

	// 1. 创建比赛
	_, c := postAuth(t, e.url+"/api/v1/contests", e.teacher, `{"name":"PkgContest"}`)
	cid := c["id"].(string)

	// 2. 学生无权导入
	zipData := makeTestPolygonZip(t)
	code, _ := postRawBytes(t, e.url+"/api/v1/contests/"+cid+"/problems/import-package", e.student, zipData)
	if code != http.StatusForbidden {
		t.Fatalf("student import should be 403, got %d", code)
	}

	// 3. 教师成功导入
	code, resp := postRawBytes(t, e.url+"/api/v1/contests/"+cid+"/problems/import-package", e.teacher, zipData)
	if code != http.StatusCreated {
		t.Fatalf("teacher import should be 201, got %d: %s", code, resp)
	}
	var prob map[string]any
	if err := json.Unmarshal(resp, &prob); err != nil {
		t.Fatal(err)
	}
	if prob["title"] != "题包导入测试" {
		t.Fatalf("imported problem title = %v, want '题包导入测试'", prob["title"])
	}

	// 4. 导出 NOI 标准包
	req, _ := http.NewRequest("GET", e.url+"/api/v1/contests/"+cid+"/export-package", nil)
	req.Header.Set("Authorization", "Bearer "+e.teacher)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("export NOI package status = %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/zip" {
		t.Fatalf("content-type = %s, want application/zip", ct)
	}
}
