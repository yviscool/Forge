package app

import (
	"strings"
	"testing"

	"github.com/yviscool/forge/internal/domain"
)

// TestAssignCSPTicketsSeasonScoping 号是（赛季, 组别）维度上的身份：
// 同季同组一号一人，跨季重新起号，J/S 互不挤占。
func TestAssignCSPTicketsSeasonScoping(t *testing.T) {
	svc := newProvinceTestService(t)
	u, err := svc.CreateAccount("张三", "zhangsan", "student", "HA")
	if err != nil {
		t.Fatal(err)
	}

	first, err := svc.AssignCSPTickets("admin", CSPTicketRequest{
		Season: "2026", Province: "HA", Group: "J", UserIDs: []string{u.ID},
	})
	if err != nil || first[0].Ticket != "HA-J00001" || first[0].Season != "2026" {
		t.Fatalf("2026 J issue = %+v err=%v", first, err)
	}

	// 同赛季同组别：拒绝重发。
	if _, err := svc.AssignCSPTickets("admin", CSPTicketRequest{
		Season: "2026", Province: "HA", Group: "J", UserIDs: []string{u.ID},
	}); err == nil {
		t.Fatal("same-season reissue must fail")
	}

	// 同赛季 S 组：独立号段，J00001 不挤占 S00001。
	sOut, err := svc.AssignCSPTickets("admin", CSPTicketRequest{
		Season: "2026", Province: "HA", Group: "S", UserIDs: []string{u.ID},
	})
	if err != nil || sOut[0].Ticket != "HA-S00001" {
		t.Fatalf("same-season S issue = %+v err=%v", sOut, err)
	}

	// 新赛季：号重新从 00001 起，旧号不受影响。
	next, err := svc.AssignCSPTickets("admin", CSPTicketRequest{
		Season: "2027", Province: "HA", Group: "J", UserIDs: []string{u.ID},
	})
	if err != nil || next[0].Ticket != "HA-J00001" {
		t.Fatalf("2027 J issue = %+v err=%v", next, err)
	}

	tickets, err := svc.ListTickets("2026", "J")
	if err != nil || len(tickets) != 1 || tickets[0].Number != "HA-J00001" {
		t.Fatalf("ListTickets(2026, J) = %+v err=%v", tickets, err)
	}
	all, err := svc.ListTickets("", "")
	if err != nil || len(all) != 3 {
		t.Fatalf("ListTickets(all) = %+v err=%v", all, err)
	}

	// 空赛季回落到当前年份（测试环境 2026）。
	auto, err := svc.AssignCSPTickets("admin", CSPTicketRequest{
		Province: "GD", Group: "J", CreateCount: 1,
	})
	if err != nil || auto[0].Season == "" {
		t.Fatalf("default season = %+v err=%v", auto, err)
	}

	// 非法赛季标识拒绝。
	if _, err := svc.AssignCSPTickets("admin", CSPTicketRequest{
		Season: "2026/秋", Province: "HA", Group: "J", UserIDs: []string{u.ID},
	}); err == nil {
		t.Fatal("path separator in season must fail")
	}
}

// TestDeleteUserReleasesTicket 删除账号必须一并注销准考证，否则号段里
// 留下永远无法对号的空号。
func TestDeleteUserReleasesTicket(t *testing.T) {
	svc := newProvinceTestService(t)
	admin, err := svc.CreateUser("管理员", "admin")
	if err != nil {
		t.Fatal(err)
	}
	_ = admin
	u, err := svc.CreateAccount("张三", "zhangsan", "student", "HA")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AssignCSPTickets("admin", CSPTicketRequest{
		Season: "2026", Province: "HA", Group: "J", UserIDs: []string{u.ID},
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteUser("admin", "admin", u.ID); err != nil {
		t.Fatal(err)
	}
	if tickets, _ := svc.ListTickets("", ""); len(tickets) != 0 {
		t.Fatalf("tickets must be released with the account: %+v", tickets)
	}
}

func TestSetContestTicketScope(t *testing.T) {
	svc := newProvinceTestService(t)
	c, err := svc.CreateContest("模拟赛", "")
	if err != nil {
		t.Fatal(err)
	}

	// 组别必须 J/S，且组别不能脱离赛季。
	if _, err := svc.SetContestTicketScope(c.ID, "2026", "A"); err == nil {
		t.Fatal("unknown ticket group must fail")
	}
	if _, err := svc.SetContestTicketScope(c.ID, "", "J"); err == nil {
		t.Fatal("ticket group without season must fail")
	}
	if _, err := svc.SetContestTicketScope(c.ID, "2026/秋", "J"); err == nil {
		t.Fatal("path separator in season must fail")
	}

	got, err := svc.SetContestTicketScope(c.ID, "2026", "j")
	if err != nil {
		t.Fatal(err)
	}
	if got.Season != "2026" || got.TicketGroup != "J" {
		t.Fatalf("scope = %q/%q, want 2026/J", got.Season, got.TicketGroup)
	}

	// 清空 = 退回普通在线赛。
	cleared, err := svc.SetContestTicketScope(c.ID, "", "")
	if err != nil || cleared.Season != "" || cleared.TicketGroup != "" {
		t.Fatalf("clear scope = %+v err=%v", cleared, err)
	}
}

func TestCollectSubmissions(t *testing.T) {
	svc := newProvinceTestService(t)

	// 考点统一发号：张三 J 号、李四没号（只报了 S）。
	zhang, err := svc.CreateAccount("张三", "zhangsan", "student", "HA")
	if err != nil {
		t.Fatal(err)
	}
	li, err := svc.CreateAccount("李四", "lisi", "student", "HA")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AssignCSPTickets("admin", CSPTicketRequest{
		Season: "2026", Province: "HA", Group: "J", UserIDs: []string{zhang.ID},
	}); err != nil {
		t.Fatal(err)
	}

	c, err := svc.CreateContest("CSP-J 模拟", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetContestTicketScope(c.ID, "2026", "J"); err != nil {
		t.Fatal(err)
	}
	pa, err := svc.CreateProblem(domain.Problem{
		ContestID: c.ID, Code: "A", Title: "扑克牌", Slug: "poker",
		Statement: "题面", Constraints: "约束", Input: "输入", Output: "输出",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateProblem(domain.Problem{
		ContestID: c.ID, Code: "B", Title: "没有英文名",
		Statement: "题面", Constraints: "约束", Input: "输入", Output: "输出",
	}); err != nil {
		t.Fatal(err)
	}
	for _, uid := range []string{zhang.ID, li.ID} {
		if err := svc.AddParticipantToContest(c.ID, uid); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.StartContest(c.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Submit(c.ID, pa.ID, zhang.ID, "cpp", "// v1"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Submit(c.ID, pa.ID, zhang.ID, "cpp", "// v2 last"); err != nil {
		t.Fatal(err)
	}

	pkg, err := svc.CollectSubmissions(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Season != "2026" || pkg.Group != "J" {
		t.Fatalf("package header = %+v", pkg)
	}
	if len(pkg.Entries) != 1 {
		t.Fatalf("entries = %+v", pkg.Entries)
	}
	e := pkg.Entries[0]
	if e.Folder != "HA-J00001" || e.FileName != "poker.cpp" || e.Code != "// v2 last" {
		t.Fatalf("entry = %+v", e)
	}
	joined := strings.Join(pkg.Warnings, "\n")
	if !strings.Contains(joined, "lisi has no J ticket") {
		t.Fatalf("missing-ticket warning expected: %v", pkg.Warnings)
	}
	if !strings.Contains(joined, "lisi submitted nothing") {
		t.Fatalf("blank-sheet warning expected: %v", pkg.Warnings)
	}
	if !strings.Contains(joined, "no slug") {
		t.Fatalf("missing-slug warning expected: %v", pkg.Warnings)
	}

	// 不存在的比赛报错。
	if _, err := svc.CollectSubmissions("ghost"); err == nil {
		t.Fatal("collect on missing contest must fail")
	}
}
