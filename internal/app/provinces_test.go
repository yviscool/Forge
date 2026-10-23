package app

import (
	"strings"
	"testing"

	"github.com/yviscool/forge/internal/adapters/store/memory"
	"github.com/yviscool/forge/internal/domain"
)

func newProvinceTestService(t *testing.T) *Service {
	t.Helper()
	return NewService(memory.New(), nil, nil, nil)
}

func TestCreateAccountWithProvince(t *testing.T) {
	svc := newProvinceTestService(t)

	u, err := svc.CreateAccount("张三", "zhangsan", "student", "河南")
	if err != nil {
		t.Fatal(err)
	}
	if u.Province != "HA" {
		t.Fatalf("province = %q, want HA (Chinese name accepted)", u.Province)
	}

	// 大小写不敏感编号。
	if u, err := svc.CreateAccount("李四", "", "student", "gd"); err != nil || u.Province != "GD" {
		t.Fatalf("province GD: %v %+v", err, u)
	}

	// 未知省份拒绝。
	if _, err := svc.CreateAccount("王五", "", "student", "亚特兰蒂斯"); err == nil {
		t.Fatal("unknown province must be rejected")
	}

	// 停用省份拒绝新建。
	if err := svc.SetProvinceEnabled("admin", "CQ", false); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateAccount("赵六", "", "student", "CQ"); err == nil {
		t.Fatal("disabled province must be rejected on create")
	}
}

func TestUpdateUserProvinceAndProfile(t *testing.T) {
	svc := newProvinceTestService(t)
	u, err := svc.CreateAccount("张三", "zhangsan", "student", "HA")
	if err != nil {
		t.Fatal(err)
	}

	// 管理员改名 + 换省。
	got, err := svc.UpdateUser("admin", u.ID, UpdateUserInput{
		Name: "张三丰", Username: "zhangsf", Province: "BJ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "张三丰" || got.Username != "zhangsf" || got.Province != "BJ" {
		t.Fatalf("updated user = %+v", got)
	}

	// 用户名冲突。
	other, _ := svc.CreateUser("李四", "student")
	if _, err := svc.UpdateUser("admin", u.ID, UpdateUserInput{
		Name: "张三丰", Username: other.Username, Province: "BJ",
	}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("duplicate username must fail, got %v", err)
	}

	// 教师可以编辑学生，但不能动管理员账号、不能改角色。
	if _, err := svc.UpdateUser("teacher", u.ID, UpdateUserInput{
		Name: "张三丰", Username: "zhangsf", Province: "BJ", Role: "teacher",
	}); err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("teacher promoting student must fail, got %v", err)
	}
	admin, _ := svc.CreateUser("root", "admin")
	if _, err := svc.UpdateUser("teacher", admin.ID, UpdateUserInput{
		Name: "root", Username: "root", Province: "HA",
	}); err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("teacher editing admin must fail, got %v", err)
	}

	// 编辑时不改省份则不受停用影响（存量数据可继续改名）。
	if err := svc.SetProvinceEnabled("admin", "BJ", false); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetProvinceEnabled("admin", "CQ", false); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateUser("admin", u.ID, UpdateUserInput{
		Name: "张三丰", Username: "zhangsf", Province: "BJ",
	}); err != nil {
		t.Fatalf("keeping unchanged disabled province must be allowed: %v", err)
	}
	if _, err := svc.UpdateUser("admin", u.ID, UpdateUserInput{
		Name: "张三丰", Username: "zhangsf", Province: "CQ",
	}); err == nil {
		t.Fatal("switching to a disabled province must fail")
	}
}

func TestBatchSetProvince(t *testing.T) {
	svc := newProvinceTestService(t)
	ids := make([]string, 0, 3)
	for _, name := range []string{"甲", "乙", "丙"} {
		u, err := svc.CreateUser(name, "student")
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, u.ID)
	}
	admin, _ := svc.CreateUser("root", "admin")

	updated, err := svc.BatchSetProvince("teacher", ids, "河南")
	if err != nil {
		t.Fatal(err)
	}
	if updated != 3 {
		t.Fatalf("updated = %d, want 3", updated)
	}
	got, _ := svc.Store().GetUser(ids[0])
	if got.Province != "HA" {
		t.Fatalf("province = %q, want HA", got.Province)
	}

	// 混入教师账号 → 整批拒绝（事务原子性）。
	if _, err := svc.BatchSetProvince("teacher", []string{ids[0], admin.ID}, "HA"); err == nil {
		t.Fatal("batch touching staff account must fail for teacher")
	}
	got, _ = svc.Store().GetUser(ids[0])
	if got.Province != "HA" {
		t.Fatal("existing HA assignment should survive failed batch (same value)")
	}
	// 重新指向别的省份时必须整批失败，不留半批。
	if _, err := svc.BatchSetProvince("teacher", []string{ids[0], admin.ID}, "SH"); err == nil {
		t.Fatal("batch must fail atomically")
	}
	got, _ = svc.Store().GetUser(ids[0])
	if got.Province != "HA" {
		t.Fatalf("atomic batch: province = %q, want HA", got.Province)
	}
}

func TestAssignCSPTicketsCreateMode(t *testing.T) {
	svc := newProvinceTestService(t)
	out, err := svc.AssignCSPTickets("teacher", CSPTicketRequest{
		Province: "HA", Group: "J", CreateCount: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	wantTickets := []string{"HA-J00001", "HA-J00002", "HA-J00003"}
	for i, a := range out {
		if a.Ticket != wantTickets[i] {
			t.Fatalf("ticket[%d] = %q, want %q", i, a.Ticket, wantTickets[i])
		}
		if a.User.ExamTicket != a.Ticket || a.User.Province != "HA" {
			t.Fatalf("assignment user = %+v", a.User)
		}
		if a.User.Role != domain.RoleStudent {
			t.Fatalf("created account role = %q, want student", a.User.Role)
		}
	}
	// 登录名是准考证号的紧凑小写形式，唯一可登录。
	if out[0].User.Username != "haj00001" {
		t.Fatalf("username = %q, want haj00001", out[0].User.Username)
	}

	// 序号接续：再发 2 个从 00004 开始。
	out2, err := svc.AssignCSPTickets("teacher", CSPTicketRequest{Province: "HA", Group: "J", CreateCount: 2})
	if err != nil {
		t.Fatal(err)
	}
	if out2[0].Ticket != "HA-J00004" || out2[1].Ticket != "HA-J00005" {
		t.Fatalf("sequence must continue: %q %q", out2[0].Ticket, out2[1].Ticket)
	}

	// J/S 独立计数；不同省份独立计数。
	sOut, err := svc.AssignCSPTickets("teacher", CSPTicketRequest{Province: "HA", Group: "S", CreateCount: 1})
	if err != nil || sOut[0].Ticket != "HA-S00001" {
		t.Fatalf("senior group counter independent: %v %+v", err, sOut)
	}
	gdOut, err := svc.AssignCSPTickets("teacher", CSPTicketRequest{Province: "GD", Group: "J", CreateCount: 1})
	if err != nil || gdOut[0].Ticket != "GD-J00001" {
		t.Fatalf("province counter independent: %v %+v", err, gdOut)
	}

	// 自定义位宽（6 位）在后续发放中生效。
	c6, err := svc.AssignCSPTickets("teacher", CSPTicketRequest{Province: "HA", Group: "J", CreateCount: 1, NumberLength: 6})
	if err != nil || c6[0].Ticket != "HA-J000006" {
		t.Fatalf("custom width ticket = %v %q", err, c6[0].Ticket)
	}

	// 校验：未知组别/超宽位长/非法省份。
	if _, err := svc.AssignCSPTickets("teacher", CSPTicketRequest{Province: "HA", Group: "A", CreateCount: 1}); err == nil {
		t.Fatal("invalid group must fail")
	}
	if _, err := svc.AssignCSPTickets("teacher", CSPTicketRequest{Province: "HA", Group: "J", CreateCount: 1, NumberLength: 12}); err == nil {
		t.Fatal("12-digit length must fail")
	}
	if _, err := svc.AssignCSPTickets("teacher", CSPTicketRequest{Province: "XX", Group: "J", CreateCount: 1}); err == nil {
		t.Fatal("unknown province must fail")
	}
	if _, err := svc.AssignCSPTickets("teacher", CSPTicketRequest{Province: "HA", Group: "J"}); err == nil {
		t.Fatal("empty request must fail")
	}
}

func TestAssignCSPTicketsToExistingUsers(t *testing.T) {
	svc := newProvinceTestService(t)
	students := make([]string, 0, 2)
	for _, name := range []string{"甲", "乙"} {
		u, err := svc.CreateUser(name, "student")
		if err != nil {
			t.Fatal(err)
		}
		students = append(students, u.ID)
	}
	henan, err := svc.CreateAccount("豫籍", "yuji", "student", "HA")
	if err != nil {
		t.Fatal(err)
	}

	out, err := svc.AssignCSPTickets("teacher", CSPTicketRequest{
		Province: "HA", Group: "J", UserIDs: []string{henan.ID, students[0], students[1]},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 || out[0].Ticket != "HA-J00001" || out[2].Ticket != "HA-J00003" {
		t.Fatalf("existing-user assignment = %+v", out)
	}

	// 已有票号的账号拒绝二次发号。
	if _, err := svc.AssignCSPTickets("teacher", CSPTicketRequest{
		Province: "HA", Group: "J", UserIDs: []string{henan.ID},
	}); err == nil || !strings.Contains(err.Error(), "already has ticket") {
		t.Fatalf("double assignment must fail, got %v", err)
	}

	// 省份不一致的账号拒绝（必须先批量改省）。
	other, _ := svc.CreateAccount("粤籍", "yueji", "student", "GD")
	if _, err := svc.AssignCSPTickets("teacher", CSPTicketRequest{
		Province: "HA", Group: "J", UserIDs: []string{other.ID},
	}); err == nil || !strings.Contains(err.Error(), "belongs to province") {
		t.Fatalf("cross-province assignment must fail, got %v", err)
	}

	// 教师不能给管理员发号。
	admin, _ := svc.CreateUser("root", "admin")
	if _, err := svc.AssignCSPTickets("teacher", CSPTicketRequest{
		Province: "HA", Group: "J", UserIDs: []string{admin.ID},
	}); err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("teacher issuing ticket to admin must fail, got %v", err)
	}
	// 管理员可以。
	if _, err := svc.AssignCSPTickets("admin", CSPTicketRequest{
		Province: "HA", Group: "J", UserIDs: []string{admin.ID},
	}); err != nil {
		t.Fatalf("admin issuing ticket to self-role admin: %v", err)
	}
}

func TestListProvincesAndCounts(t *testing.T) {
	svc := newProvinceTestService(t)
	if _, err := svc.CreateAccount("甲", "a", "student", "HA"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateAccount("乙", "b", "student", "HA"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateAccount("丙", "c", "student", "GD"); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetProvinceEnabled("admin", "CQ", false); err != nil {
		t.Fatal(err)
	}

	views, err := svc.ListProvinces()
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 34 {
		t.Fatalf("views = %d, want 34", len(views))
	}
	byCode := map[string]ProvinceView{}
	for _, v := range views {
		byCode[v.Code] = v
	}
	if byCode["HA"].Users != 2 || byCode["GD"].Users != 1 || byCode["BJ"].Users != 0 {
		t.Fatalf("user counts = %+v", byCode["HA"])
	}
	if byCode["CQ"].Enabled {
		t.Fatal("CQ should be disabled")
	}
	if !byCode["BJ"].Enabled {
		t.Fatal("unset provinces default to enabled")
	}
	if views[0].Code != "BJ" || views[33].Code != "TW" {
		t.Fatalf("views must be sort-ordered, first=%s last=%s", views[0].Code, views[33].Code)
	}
}

func TestSetProvinceEnabledRequiresAdmin(t *testing.T) {
	svc := newProvinceTestService(t)
	if err := svc.SetProvinceEnabled("teacher", "HA", false); err == nil {
		t.Fatal("teacher must not toggle provinces")
	}
	if err := svc.SetProvinceEnabled("admin", "HA", false); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetProvinceEnabled("admin", "XX", false); err == nil {
		t.Fatal("unknown province must fail")
	}
	views, _ := svc.ListProvinces()
	for _, v := range views {
		if v.Code == "HA" && v.Enabled {
			t.Fatal("HA should stay disabled")
		}
	}
}
