package domain

import (
	"strings"
	"testing"
	"time"
)

func collectFixture() CollectSource {
	now := time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)
	users := []User{
		{ID: "u-j", Username: "stu_j", Name: "甲", Role: RoleStudent},
		{ID: "u-both", Username: "stu_both", Name: "乙", Role: RoleStudent},
		{ID: "u-none", Username: "stu_none", Name: "丙", Role: RoleStudent},
	}
	contest := Contest{
		ID: "c1", Name: "CSP-J 模拟", Status: string(Running),
		Season: "2026", TicketGroup: "J",
		ParticipantUserIDs: []string{"u-j", "u-none"},
		GroupIDs:           []string{"g1"},
		ProblemIDs:         []string{"p-a", "p-b"},
	}
	groups := []Group{{ID: "g1", Name: "提高组", UserIDs: []string{"u-both"}}}
	problems := []Problem{
		{ID: "p-a", ContestID: "c1", Code: "A", Slug: "poker", Title: "扑克牌"},
		{ID: "p-b", ContestID: "c1", Code: "B", Title: "没有英文名"},
	}
	tickets := []Ticket{
		{UserID: "u-j", Season: "2026", Group: "J", Province: "HA", Number: "HA-J00001"},
		// 乙只有 S 组号：J 赛不收他的 J 号——没有号就退登录名。
		{UserID: "u-both", Season: "2026", Group: "S", Province: "HA", Number: "HA-S00001"},
	}
	subs := []Submission{
		{ID: "s1", ContestID: "c1", ProblemID: "p-a", UserID: "u-j", Language: "cpp",
			Code: "// first", Verdict: VerdictAccepted, Score: 100, SubmittedAt: now},
		{ID: "s2", ContestID: "c1", ProblemID: "p-a", UserID: "u-j", Language: "cpp",
			Code: "// last wins", Verdict: VerdictWrongAnswer, Score: 30, SubmittedAt: now.Add(time.Minute)},
		{ID: "s3", ContestID: "c1", ProblemID: "p-b", UserID: "u-both", Language: "python3",
			Code: "# py", Verdict: VerdictAccepted, Score: 100, SubmittedAt: now},
	}
	return CollectSource{
		Contest: contest, Groups: groups, Users: users,
		Problems: problems, Tickets: tickets, Submissions: subs,
	}
}

func TestComputeCollectLayout(t *testing.T) {
	pkg := ComputeCollectLayout(collectFixture())

	if pkg.ContestID != "c1" || pkg.Season != "2026" || pkg.Group != "J" {
		t.Fatalf("package header = %+v", pkg)
	}
	if len(pkg.Entries) != 2 {
		t.Fatalf("entries = %+v", pkg.Entries)
	}

	// 甲：J 号目录，每题收最后一次提交。
	var byFile = map[string]CollectEntry{}
	for _, e := range pkg.Entries {
		byFile[e.Folder+"/"+e.FileName] = e
	}
	j, ok := byFile["HA-J00001/poker.cpp"]
	if !ok {
		t.Fatalf("missing HA-J00001/poker.cpp: %+v", pkg.Entries)
	}
	if j.Code != "// last wins" || j.Verdict != VerdictWrongAnswer || j.Score != 30 {
		t.Fatalf("last submission must win: %+v", j)
	}
	if j.Ticket != "HA-J00001" || j.UserName != "甲" || j.Language != "cpp" {
		t.Fatalf("entry header = %+v", j)
	}

	// 乙：分组绑定进名单，但只有 S 号 → 登录名目录 + 告警。
	b, ok := byFile["stu_both/poker.cpp"]
	if ok {
		t.Fatalf("u-both submitted p-a? %+v", b)
	}
	if _, ok := byFile["stu_both/noenglish.py"]; ok {
		t.Fatal("python3 must normalize to .py, and p-b has no slug")
	}
	// 乙只交了 p-b：文件名退回题号小写。
	found := false
	for _, e := range pkg.Entries {
		if e.Folder == "stu_both" {
			found = true
			if e.FileName != "b.py" {
				t.Fatalf("slug fallback to lowercased code = %q", e.FileName)
			}
		}
	}
	if !found {
		t.Fatalf("group-bound participant missing from roster: %+v", pkg.Entries)
	}

	joined := strings.Join(pkg.Warnings, "\n")
	if !strings.Contains(joined, "no slug") {
		t.Fatalf("missing-slug warning expected: %v", pkg.Warnings)
	}
	if !strings.Contains(joined, "stu_both has no J ticket") {
		t.Fatalf("missing-ticket warning expected: %v", pkg.Warnings)
	}
	if !strings.Contains(joined, "stu_none submitted nothing") {
		t.Fatalf("blank-sheet warning expected: %v", pkg.Warnings)
	}
}

func TestComputeCollectLayoutUsernameMode(t *testing.T) {
	src := collectFixture()
	src.Contest.Season = ""
	src.Contest.TicketGroup = ""
	pkg := ComputeCollectLayout(src)

	for _, e := range pkg.Entries {
		if e.Ticket != "" {
			t.Fatalf("no ticket mode must not attach tickets: %+v", e)
		}
		if e.Folder != e.Username {
			t.Fatalf("folders must be usernames without ticketGroup: %+v", e)
		}
	}
}

func TestComputeCollectLayoutIgnoresForeignSubmissions(t *testing.T) {
	src := collectFixture()
	src.Submissions = append(src.Submissions, Submission{
		ID: "s9", ContestID: "c1", ProblemID: "p-a", UserID: "u-ghost",
		Language: "cpp", Code: "// ghost", SubmittedAt: time.Now().UTC().Add(time.Hour),
	})
	pkg := ComputeCollectLayout(src)
	for _, e := range pkg.Entries {
		if e.UserID == "u-ghost" {
			t.Fatalf("submission from non-roster user leaked in: %+v", e)
		}
	}
}

func TestCollectLanguageExtension(t *testing.T) {
	cases := map[string]string{
		"cpp": ".cpp", "C++": ".cpp", "c": ".c",
		"python3": ".py", "py": ".py", "go": ".go",
		"": ".txt", "brainfuck": ".txt",
	}
	for lang, want := range cases {
		if got := collectLanguageExtension(lang); got != want {
			t.Fatalf("ext(%q) = %q, want %q", lang, got, want)
		}
	}
}

func TestSanitizeCollectPath(t *testing.T) {
	if got := SanitizeCollectPath(`a/b\c:d*e?f"g<h>i|j`); strings.ContainsAny(got, `/\:*?"<>|`) {
		t.Fatalf("path separators must be replaced: %q", got)
	}
	if got := SanitizeCollectPath("  HA-J00001 "); got != "HA-J00001" {
		t.Fatalf("trim failed: %q", got)
	}
}
