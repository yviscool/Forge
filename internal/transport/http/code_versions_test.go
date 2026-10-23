package httpapi

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestCodeVersionsRoute(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	admin := e.mkAdmin(t)

	code, body := requestJSON(t, "POST", e.url+"/api/v1/contests", admin, `{"name":"版本赛"}`)
	if code != 201 {
		t.Fatalf("create contest = %d %v", code, body)
	}
	cid, _ := body["id"].(string)
	code, body = requestJSON(t, "POST", e.url+"/api/v1/contests/"+cid+"/participants", admin,
		fmt.Sprintf(`{"userId":%q}`, e.stuID))
	if code != 200 {
		t.Fatalf("authorize = %d %v", code, body)
	}
	code, body = requestJSON(t, "POST", e.url+"/api/v1/contests/"+cid+"/problems", admin,
		`{"code":"A","title":"版本题","statement":"迭代作答。","input":"n","output":"x","constraints":"n<=10","examples":"1\n2"}`)
	if code != 201 {
		t.Fatalf("add problem = %d %v", code, body)
	}
	pid, _ := body["id"].(string)

	// 学生保存两个版本。
	for i, src := range []string{"int v1;", "int v2;"} {
		code, body = requestJSON(t, "POST", e.url+"/api/v1/contests/"+cid+"/problems/"+pid+"/versions",
			e.student, fmt.Sprintf(`{"code":%q,"language":"cpp","source":"save"}`, src))
		if code != 201 {
			t.Fatalf("save v%d = %d %v", i+1, code, body)
		}
		if body["seq"].(float64) != float64(i+1) {
			t.Fatalf("seq = %v, want %d", body["seq"], i+1)
		}
	}
	// 幂等：同内容不追加。
	code, body = requestJSON(t, "POST", e.url+"/api/v1/contests/"+cid+"/problems/"+pid+"/versions",
		e.student, `{"code":"int v2;","language":"cpp","source":"save"}`)
	if code != 201 || body["seq"].(float64) != 2 {
		t.Fatalf("dedup save = %d %v", code, body)
	}

	// 学生读自己的版本（最新在前）。
	code, raw := getAuth(t, e.url+"/api/v1/contests/"+cid+"/problems/"+pid+"/versions", e.student)
	if code != 200 {
		t.Fatalf("list own = %d", code)
	}
	var versions []map[string]any
	_ = json.Unmarshal(raw, &versions)
	if len(versions) != 2 || versions[0]["code"] != "int v2;" {
		t.Fatalf("versions = %v", versions)
	}

	// 学生不能读别人的；教师可以代查。
	if code, _ := requestJSON(t, "GET", e.url+"/api/v1/contests/"+cid+"/problems/"+pid+"/versions?userId=other", e.student, ""); code != 403 {
		t.Fatalf("student reading others = %d, want 403", code)
	}
	code, raw = getAuth(t, e.url+"/api/v1/contests/"+cid+"/problems/"+pid+"/versions?userId="+e.stuID, admin)
	if code != 200 {
		t.Fatalf("teacher reading student versions = %d", code)
	}
	_ = json.Unmarshal(raw, &versions)
	if len(versions) != 2 {
		t.Fatalf("teacher view versions = %d", len(versions))
	}
}
