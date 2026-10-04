package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/yviscool/forge/internal/testdata"
)

func putRaw(t *testing.T, url, token, body string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest("PUT", url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r.Body)
	return r.StatusCode, buf.Bytes()
}

func TestFilesAndRejudge(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	e.srv.Files = testdata.New(t.TempDir())

	_, c := postAuth(t, e.url+"/api/v1/contests", e.teacher, `{"name":"F"}`)
	cid := c["id"].(string)
	_, p := postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems", e.teacher,
		`{"code":"A","title":"T","statement":"s","input":"i","output":"o","constraints":"k"}`)
	pid := p["id"].(string)

	base := fmt.Sprintf("%s/api/v1/contests/%s/problems/%s/files", e.url, cid, pid)
	if code, _ := putRaw(t, base+"/a01.in", e.student, "3 4"); code != 403 {
		t.Fatalf("student upload should 403, got %d", code)
	}
	if code, _ := putRaw(t, base+"/a01.in", e.teacher, "3 4"); code != 200 {
		t.Fatalf("upload %d", code)
	}
	if code, _ := putRaw(t, base+"/../evil", e.teacher, "x"); code != 400 && code != 404 {
		t.Fatalf("traversal should fail, got %d", code)
	}
	if code, body := getAuth(t, base, e.teacher); code != 200 {
		t.Fatalf("list %d", code)
	} else {
		var v map[string]any
		_ = json.Unmarshal(body, &v)
		if len(v["files"].([]any)) != 1 || len(v["missingPairs"].([]any)) != 1 {
			t.Fatalf("pairs check: %s", body)
		}
	}
	if code, _ := putRaw(t, base+"/a01.out", e.teacher, "7"); code != 200 {
		t.Fatalf("upload out %d", code)
	}
	if code, body := getAuth(t, base, e.teacher); code == 200 {
		var v map[string]any
		_ = json.Unmarshal(body, &v)
		if len(v["missingPairs"].([]any)) != 0 {
			t.Fatalf("pairs complete: %s", body)
		}
	}
	if code, body := getAuth(t, base+"/a01.in", e.teacher); code != 200 || string(body) != "3 4" {
		t.Fatalf("download: %d %s", code, body)
	}

	// 重判：空提交返回 0。
	if code, body := postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems/"+pid+"/rejudge", e.teacher, ``); code != 202 {
		t.Fatalf("rejudge %d %v", code, body)
	} else if int(body["total"].(float64)) != 0 {
		t.Fatalf("empty rejudge: %v", body)
	}
	postAuth(t, e.url+"/api/v1/contests/"+cid+"/start", e.teacher, ``)
	postAuth(t, e.url+"/api/v1/contests/"+cid+"/submissions", e.student, fmt.Sprintf(`{"problemID":%q,"language":"cpp","code":"x"}`, pid))
	if code, body := postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems/"+pid+"/rejudge", e.teacher, ``); code != 202 || int(body["total"].(float64)) != 1 {
		t.Fatalf("rejudge count: %d %v", code, body)
	}
	if code, _ := postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems/"+pid+"/rejudge", e.student, ``); code != 403 {
		t.Fatalf("student rejudge should 403, got %d", code)
	}

	// 子任务配置 + 统计。
	subBody := `{"subtasks":[{"subtask":0,"score":100,"timeLimitMs":500,"memoryMib":128}]}`
	if code, _ := putRaw(t, e.url+"/api/v1/contests/"+cid+"/problems/"+pid+"/subtasks", e.teacher, subBody); code != 200 {
		t.Fatalf("subtasks %d", code)
	}
	if code, body := getAuth(t, e.url+"/api/v1/contests/"+cid+"/statistics", e.teacher); code != 200 {
		t.Fatalf("statistics %d", code)
	} else {
		var v map[string]any
		_ = json.Unmarshal(body, &v)
		if int(v["submissions"].(float64)) != 1 {
			t.Fatalf("stats: %s", body)
		}
	}
	if code, _ := getAuth(t, e.url+"/api/v1/contests/"+cid+"/statistics", e.student); code != 403 {
		t.Fatalf("student statistics should 403, got %d", code)
	}
}

func TestUsersImportExport(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	csvBody := "name,role,group,password\nDave,student,ClassA,pw-dave\nEve,teacher,,pw-eve\nZed,student,,ab\n"
	payload, _ := json.Marshal(map[string]string{"csv": csvBody})
	code, body := postAuth(t, e.url+"/api/v1/users/import", e.teacher, string(payload))
	if code != 200 {
		t.Fatalf("import %d %v", code, body)
	}
	// Zed 建号成功但密码过短：created 照计，errors 记一条。
	if int(body["created"].(float64)) != 3 || len(body["errors"].([]any)) != 1 {
		t.Fatalf("import result: %v", body)
	}
	if code, _ := postAuth(t, e.url+"/api/v1/users/import", e.student, string(payload)); code != 403 {
		t.Fatalf("student import should 403, got %d", code)
	}
	// Dave 能用导入的密码登录。
	if code, _ := post(t, e.url+"/api/v1/auth/login", `{"login":"Dave","password":"pw-dave"}`); code != 200 {
		t.Fatalf("imported login %d", code)
	}
	// 导出含 Dave/ClassA，不含密码。
	if code, data := getAuth(t, e.url+"/api/v1/users/export", e.teacher); code != 200 {
		t.Fatalf("export %d", code)
	} else if !strings.Contains(string(data), "Dave,student,ClassA") || strings.Contains(string(data), "pw-dave") {
		t.Fatalf("export content: %s", data)
	}
}

func TestACMRankingEndpoint(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	_, c := postAuth(t, e.url+"/api/v1/contests", e.teacher, `{"name":"ACM","rankingMode":"acm"}`)
	if c["rankingMode"] != "acm" {
		t.Fatalf("rankingMode not stored: %v", c)
	}
	if code, _ := postAuth(t, e.url+"/api/v1/contests", e.teacher, `{"name":"X","rankingMode":"elo"}`); code != 400 {
		t.Fatalf("bad mode should 400, got %d", code)
	}
	cid := c["id"].(string)
	_, p := postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems", e.teacher,
		`{"code":"A","title":"T","statement":"s","input":"i","output":"o","constraints":"k"}`)
	postAuth(t, e.url+"/api/v1/contests/"+cid+"/start", e.teacher, ``)
	submit := fmt.Sprintf(`{"problemID":%q,"language":"cpp","code":"x"}`, p["id"])
	_, sub := postAuth(t, e.url+"/api/v1/contests/"+cid+"/submissions", e.student, submit)
	postAuth(t, e.url+"/api/v1/submissions/"+sub["id"].(string)+"/judge", e.teacher, `{"verdict":"accepted","score":100}`)

	if code, body := getAuth(t, e.url+"/api/v1/contests/"+cid+"/ranking", e.student); code != 200 {
		t.Fatalf("acm ranking %d", code)
	} else {
		var ranks []map[string]any
		_ = json.Unmarshal(body, &ranks)
		found := false
		for _, r := range ranks {
			if r["userName"] == "Alice" {
				if int(r["solved"].(float64)) != 1 {
					t.Fatalf("acm solved: %s", body)
				}
				found = true
			}
		}
		if !found {
			t.Fatalf("alice missing: %s", body)
		}
	}
}

func TestLanguagesAndExport(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	// 建赛即锁 C++。
	_, c := postAuth(t, e.url+"/api/v1/contests", e.teacher, `{"name":"CSP","allowedLanguages":["cpp"]}`)
	if len(c["allowedLanguages"].([]any)) != 1 {
		t.Fatalf("langs: %v", c)
	}
	cid := c["id"].(string)
	_, p := postAuth(t, e.url+"/api/v1/contests/"+cid+"/problems", e.teacher,
		`{"code":"A","title":"T","statement":"s","input":"i","output":"o","constraints":"k"}`)
	pid := p["id"].(string)
	postAuth(t, e.url+"/api/v1/contests/"+cid+"/start", e.teacher, ``)

	py := fmt.Sprintf(`{"problemID":%q,"language":"python","code":"x"}`, pid)
	if code, _ := postAuth(t, e.url+"/api/v1/contests/"+cid+"/submissions", e.student, py); code != 400 {
		t.Fatalf("python should be rejected, got %d", code)
	}
	cpp := fmt.Sprintf(`{"problemID":%q,"language":"cpp","code":"int main(){}"}`, pid)
	if code, sub := postAuth(t, e.url+"/api/v1/contests/"+cid+"/submissions", e.student, cpp); code != 202 {
		t.Fatalf("cpp submit %d %v", code, sub)
	} else {
		// 教师判 CE 带原文。
		sid := sub["id"].(string)
		postAuth(t, e.url+"/api/v1/submissions/"+sid+"/judge", e.teacher, `{"verdict":"compile_error","score":0}`)
		if _, err := e.svc.Store().GetSubmission(sid); err != nil {
			t.Fatal(err)
		}
	}

	// settings 放开 python。
	if code, body := putRaw(t, e.url+"/api/v1/contests/"+cid+"/settings", e.teacher,
		`{"allowedLanguages":["cpp","python"]}`); code != 200 {
		t.Fatalf("settings %d %s", code, body)
	} else {
		var v map[string]any
		_ = json.Unmarshal(body, &v)
		if len(v["allowedLanguages"].([]any)) != 2 {
			t.Fatalf("settings langs: %s", body)
		}
	}
	if code, _ := postAuth(t, e.url+"/api/v1/contests/"+cid+"/submissions", e.student, py); code != 202 {
		t.Fatalf("python after open %d", code)
	}
	if code, _ := putRaw(t, e.url+"/api/v1/contests/"+cid+"/settings", e.student, `{}`); code != 403 {
		t.Fatalf("student settings should 403, got %d", code)
	}
	if code, _ := putRaw(t, e.url+"/api/v1/contests/"+cid+"/settings", e.teacher, `{"rankingMode":"elo"}`); code != 400 {
		t.Fatalf("bad mode should 400, got %d", code)
	}

	// 成绩单导出。
	if code, data := getAuth(t, e.url+"/api/v1/contests/"+cid+"/statistics/export", e.teacher); code != 200 {
		t.Fatalf("export %d", code)
	} else if !strings.Contains(string(data), "Alice") || !strings.Contains(string(data), "name,total,accepted") {
		t.Fatalf("export content: %s", data)
	}
	if code, _ := getAuth(t, e.url+"/api/v1/contests/"+cid+"/statistics/export", e.student); code != 403 {
		t.Fatalf("student export should 403, got %d", code)
	}
}
