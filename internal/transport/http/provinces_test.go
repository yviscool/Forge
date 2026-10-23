package httpapi

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// mkAdmin 建一个可登录的管理员并返回 token。
func (e *testEnv) mkAdmin(t *testing.T) string {
	t.Helper()
	return e.mkUser(t, "rootadmin", "admin", "pw-admin-1")
}

func TestProvincesEndpoints(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	admin := e.mkAdmin(t)

	code, raw := getAuth(t, e.url+"/api/v1/provinces", e.teacher)
	if code != 200 {
		t.Fatalf("GET /provinces = %d", code)
	}
	var views []map[string]any
	if err := json.Unmarshal(raw, &views); err != nil {
		t.Fatalf("decode provinces: %v", err)
	}
	if len(views) != 34 {
		t.Fatalf("provinces = %d, want 34", len(views))
	}
	byCode := map[string]map[string]any{}
	for _, v := range views {
		byCode[v["code"].(string)] = v
	}
	if byCode["HA"]["name"] != "河南" || byCode["HA"]["sort"].(float64) != 170 {
		t.Fatalf("HA row = %+v", byCode["HA"])
	}
	if !byCode["HA"]["enabled"].(bool) {
		t.Fatal("HA should default to enabled")
	}

	// 学生无权访问。
	if code, _ := requestJSON(t, "GET", e.url+"/api/v1/provinces", e.student, ""); code != 403 {
		t.Fatalf("student GET /provinces = %d, want 403", code)
	}

	// 教师不能停用省份；管理员可以。
	if code, _ := requestJSON(t, "PUT", e.url+"/api/v1/provinces/HA", e.teacher, `{"enabled":false}`); code != 403 {
		t.Fatalf("teacher PUT province = %d, want 403", code)
	}
	if code, body := requestJSON(t, "PUT", e.url+"/api/v1/provinces/HA", admin, `{"enabled":false}`); code != 200 {
		t.Fatalf("admin PUT province = %d %v", code, body)
	}
	code, raw = getAuth(t, e.url+"/api/v1/provinces", e.teacher)
	_ = json.Unmarshal(raw, &views)
	for _, v := range views {
		if v["code"] == "HA" && v["enabled"].(bool) {
			t.Fatal("HA should be disabled after admin toggle")
		}
	}

	// 停用省份阻止新建账号（编号与中文全名 alike）。
	payload := `{"name":"钱七","username":"qianqi","role":"student","password":"pw-qianqi-1","province":"HA"}`
	if code, body := requestJSON(t, "POST", e.url+"/api/v1/users", e.teacher, payload); code != 400 {
		t.Fatalf("create user with disabled province = %d %v, want 400", code, body)
	}
	payload = `{"name":"钱八","username":"qianba","role":"student","password":"pw-qianba-1","province":"河南"}`
	if code, _ := requestJSON(t, "POST", e.url+"/api/v1/users", e.teacher, payload); code != 400 {
		t.Fatalf("create user with disabled province (Chinese name) = %d, want 400", code)
	}
	if code, _ := requestJSON(t, "PUT", e.url+"/api/v1/provinces/HA", admin, `{"enabled":true}`); code != 200 {
		t.Fatal("re-enable HA")
	}

	// 启用后正常创建并回显省份；未知省份 400。
	code, body := requestJSON(t, "POST", e.url+"/api/v1/users", e.teacher, payload)
	if code != 201 {
		t.Fatalf("create user with province = %d %v", code, body)
	}
	if body["province"] != "HA" {
		t.Fatalf("created user province = %v, want HA", body["province"])
	}
	bad := `{"name":"孙九","username":"sunjiu","role":"student","password":"pw-sunjiu-1","province":"XX"}`
	if code, _ := requestJSON(t, "POST", e.url+"/api/v1/users", e.teacher, bad); code != 400 {
		t.Fatalf("unknown province = %d, want 400", code)
	}
}

func TestUserEditEndpoint(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	admin := e.mkAdmin(t)

	payload := `{"name":"周十","username":"zhoushi","role":"student","password":"pw-zhoushi-1","province":"HA"}`
	code, body := requestJSON(t, "POST", e.url+"/api/v1/users", e.teacher, payload)
	if code != 201 {
		t.Fatalf("create = %d %v", code, body)
	}
	uid, _ := body["id"].(string)
	if uid == "" {
		t.Fatal("created user must expose id")
	}

	// 教师改学生：姓名/省份。
	code, body = requestJSON(t, "PUT", e.url+"/api/v1/users/"+uid, e.teacher,
		`{"name":"周十一","username":"zhoushi","province":"GD"}`)
	if code != 200 {
		t.Fatalf("edit by teacher = %d %v", code, body)
	}
	if body["name"] != "周十一" || body["province"] != "GD" {
		t.Fatalf("edited user = %+v", body)
	}

	// 教师不能把学生提为教师。
	code, body = requestJSON(t, "PUT", e.url+"/api/v1/users/"+uid, e.teacher,
		`{"name":"周十一","username":"zhoushi","province":"GD","role":"teacher"}`)
	if code != 403 {
		t.Fatalf("teacher promoting = %d %v, want 403", code, body)
	}

	// 管理员可以改角色；教师不能编辑管理员。
	var adminUID string
	for _, u := range e.svc.ListUsers() {
		if u.Username == "rootadmin" {
			adminUID = u.ID
		}
	}
	if adminUID == "" {
		t.Fatal("admin user missing")
	}
	if code, _ := requestJSON(t, "PUT", e.url+"/api/v1/users/"+adminUID, e.teacher,
		`{"name":"rootadmin","username":"rootadmin","province":"HA"}`); code != 403 {
		t.Fatalf("teacher editing admin = %d, want 403", code)
	}
	code, body = requestJSON(t, "PUT", e.url+"/api/v1/users/"+uid, admin,
		`{"name":"周十一","username":"zhoushi","province":"GD","role":"teacher"}`)
	if code != 200 || body["role"] != "teacher" {
		t.Fatalf("admin role change = %d %v", code, body)
	}

	// 用户名冲突 400。
	code, _ = requestJSON(t, "PUT", e.url+"/api/v1/users/"+uid, admin,
		`{"name":"周十一","username":"Prof","province":"GD"}`)
	if code != 400 {
		t.Fatalf("duplicate username = %d, want 400", code)
	}
}

func TestBatchProvinceEndpoint(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	ids := []string{e.stuID}
	payload, _ := json.Marshal(map[string]any{"userIds": ids, "province": "河南"})
	if code, body := requestJSON(t, "POST", e.url+"/api/v1/users/batch-province", e.teacher, string(payload)); code != 200 {
		t.Fatalf("batch province = %d %v", code, body)
	}
	u, err := e.svc.Store().GetUser(e.stuID)
	if err != nil || u.Province != "HA" {
		t.Fatalf("user province = %+v err=%v", u, err)
	}

	// 空列表 400。
	if code, _ := requestJSON(t, "POST", e.url+"/api/v1/users/batch-province", e.teacher, `{"userIds":[],"province":"HA"}`); code != 400 {
		t.Fatalf("empty batch = %d, want 400", code)
	}
}

func TestCSPTicketsEndpoint(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	// 批量建考生：J 组 HA 省 3 个。
	code, body := requestJSON(t, "POST", e.url+"/api/v1/users/csp-tickets", e.teacher,
		`{"province":"HA","group":"J","createCount":3,"password":"pw-csp-ticket-1"}`)
	if code != 201 {
		t.Fatalf("csp-tickets create = %d %v", code, body)
	}
	assignments, _ := body["assignments"].([]any)
	if len(assignments) != 3 {
		t.Fatalf("assignments = %d, want 3", len(assignments))
	}
	first, _ := assignments[0].(map[string]any)
	if first["ticket"] != "HA-J00001" {
		t.Fatalf("first ticket = %v, want HA-J00001", first["ticket"])
	}
	// 统一密码可登录。
	created, _ := first["user"].(map[string]any)
	uname, _ := created["username"].(string)
	if code, _, tok := loginForToken(t, e.url, uname, "pw-csp-ticket-1"); code != 200 || tok == "" {
		t.Fatalf("created candidate cannot login: %d", code)
	}

	// 序号接续 + 组别/省份独立。
	code, body = requestJSON(t, "POST", e.url+"/api/v1/users/csp-tickets", e.teacher,
		`{"province":"HA","group":"J","createCount":1}`)
	if code != 201 {
		t.Fatalf("continue sequence = %d %v", code, body)
	}
	assignments, _ = body["assignments"].([]any)
	a0, _ := assignments[0].(map[string]any)
	if a0["ticket"] != "HA-J00004" {
		t.Fatalf("sequence must continue, got %v", a0["ticket"])
	}
	code, body = requestJSON(t, "POST", e.url+"/api/v1/users/csp-tickets", e.teacher,
		`{"province":"HA","group":"S","createCount":1,"numberLength":6}`)
	if code != 201 {
		t.Fatalf("senior group = %d %v", code, body)
	}
	assignments, _ = body["assignments"].([]any)
	a0, _ = assignments[0].(map[string]any)
	if a0["ticket"] != "HA-S000001" {
		t.Fatalf("senior ticket = %v, want HA-S000001", a0["ticket"])
	}

	// 给既有学生发号（学生当前无省份）。
	code, body = requestJSON(t, "POST", e.url+"/api/v1/users/csp-tickets", e.teacher,
		fmt.Sprintf(`{"province":"HA","group":"J","userIds":["%s"]}`, e.stuID))
	if code != 201 {
		t.Fatalf("assign existing = %d %v", code, body)
	}
	u, _ := e.svc.Store().GetUser(e.stuID)
	if u.ExamTicket != "HA-J00005" || u.Province != "HA" {
		t.Fatalf("existing user ticket = %+v", u)
	}

	// 二次发号 400。
	code, _ = requestJSON(t, "POST", e.url+"/api/v1/users/csp-tickets", e.teacher,
		fmt.Sprintf(`{"province":"HA","group":"J","userIds":["%s"]}`, e.stuID))
	if code != 400 {
		t.Fatalf("double assign = %d, want 400", code)
	}

	// 同时给 createCount 和 userIds 400。
	code, _ = requestJSON(t, "POST", e.url+"/api/v1/users/csp-tickets", e.teacher,
		fmt.Sprintf(`{"province":"HA","group":"J","createCount":1,"userIds":["%s"]}`, e.stuID))
	if code != 400 {
		t.Fatalf("ambiguous request = %d, want 400", code)
	}
}

func TestCSVImportExportWithProvince(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	csv := strings.Join([]string{
		"name,role,group,password,username,province",
		"甲,student,一班,pw-import-11,stu_a,河南",
		"乙,student,一班,pw-import-11,stu_b,gd",
		"丙,student,,pw-import-11,stu_c,亚特兰蒂斯",
	}, "\n")
	code, body := requestJSON(t, "POST", e.url+"/api/v1/users/import", e.teacher,
		fmt.Sprintf(`{"csv":%q}`, csv))
	if code != 200 {
		t.Fatalf("import = %d %v", code, body)
	}
	if body["created"].(float64) != 2 {
		t.Fatalf("created = %v, want 2", body["created"])
	}
	errs, _ := body["errors"].([]any)
	if len(errs) != 1 || !strings.Contains(fmt.Sprint(errs[0]), "unknown province") {
		t.Fatalf("errors = %v, want one unknown-province row", errs)
	}

	// 导出包含省份与准考证列（省份为归一化后的编号）。
	code, raw := getAuth(t, e.url+"/api/v1/users/export", e.teacher)
	if code != 200 {
		t.Fatalf("export = %d", code)
	}
	exported := string(raw)
	for _, want := range []string{"province", "examTicket", "stu_a", "HA", "GD"} {
		if !strings.Contains(exported, want) {
			t.Fatalf("export missing %q:\n%s", want, exported)
		}
	}
}
