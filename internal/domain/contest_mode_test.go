package domain

import (
	"testing"
	"time"
)

func TestNormalizeAndValidMode(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"", ModeOnline}, {"online", ModeOnline}, {"NOI", ModeNOI},
		{" gesp ", ModeGESP}, {"demo", ModeOnline},
	} {
		if got := NormalizeMode(c.in); got != c.want {
			t.Errorf("NormalizeMode(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if ValidMode("demo") {
		t.Error("ValidMode(demo) should be false")
	}
	if !ValidMode("") || !ValidMode("noi") || !ValidMode("gesp") {
		t.Error("empty/noi/gesp must be valid modes")
	}
}

func TestStudentResultsVisibleMatrix(t *testing.T) {
	cases := []struct {
		mode   string
		status Status
		want   bool
	}{
		{ModeOnline, Running, true},
		{ModeGESP, Running, true},
		{ModeNOI, Running, false},
		{ModeNOI, Finished, true},
		{ModeNOI, Draft, true},
	}
	for _, c := range cases {
		if got := StudentResultsVisible(c.mode, c.status); got != c.want {
			t.Errorf("StudentResultsVisible(%s,%s) = %v, want %v", c.mode, c.status, got, c.want)
		}
	}
}

func TestMaskSubmissionForStudent(t *testing.T) {
	s := Submission{
		ID: "s1", Verdict: "accepted", Score: 90,
		Cases: []CaseResult{{CaseIndex: 0, Verdict: "accepted"}},
	}
	m := MaskSubmissionForStudent(s)
	if m.Verdict != VerdictSubmittedMasked || m.Score != 0 || m.Cases != nil {
		t.Fatalf("masked submission = %+v", m)
	}
	// 原对象不被修改。
	if s.Verdict != "accepted" || s.Score != 90 || s.Cases == nil {
		t.Fatalf("source submission mutated: %+v", s)
	}
}

func TestNOIRankingUsesLastSubmission(t *testing.T) {
	base := time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)
	mk := func(id, uid, pid string, score int, at time.Time) Submission {
		return Submission{ID: id, ContestID: "c1", UserID: uid, ProblemID: pid,
			Verdict: "accepted", Score: score, SubmittedAt: at}
	}
	users := []User{{ID: "u1", Name: "甲", Role: RoleStudent}}
	contest := Contest{ID: "c1", Status: string(Finished), Mode: ModeNOI, ProblemIDs: []string{"p1"}}

	// 先高分后低分：NOI 以最后一次（低分）为准。
	subs := []Submission{
		mk("s1", "u1", "p1", 90, base),
		mk("s2", "u1", "p1", 40, base.Add(time.Minute)),
	}
	rank := ComputeRanking(users, contest, subs)
	if rank[0].ProblemScores["p1"] != 40 || rank[0].Score != 40 {
		t.Fatalf("noi last-submission ranking = %+v", rank[0])
	}

	// online 同样提交序列取最高分。
	contestOnline := contest
	contestOnline.Mode = ModeOnline
	rank = ComputeRanking(users, contestOnline, subs)
	if rank[0].Score != 90 {
		t.Fatalf("online best-score ranking = %+v", rank[0])
	}

	// 时间并列按 ID 兜底：s2 视为更晚。
	subsTie := []Submission{
		mk("s2", "u1", "p1", 40, base),
		mk("s1", "u1", "p1", 90, base),
	}
	rank = ComputeRanking(users, contest, subsTie)
	if rank[0].Score != 40 {
		t.Fatalf("noi tie-break ranking = %+v", rank[0])
	}
}
