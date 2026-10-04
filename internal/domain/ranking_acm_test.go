package domain

import (
	"testing"
	"time"
)

func TestComputeRankingACM(t *testing.T) {
	base := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	c := Contest{ID: "c", StartedAt: base}
	users := []User{{ID: "u1", Name: "Ada"}, {ID: "u2", Name: "Bob"}, {ID: "u3", Name: "Cid"}}
	subs := []Submission{
		// Ada: P1 WA@5min then AC@10min (20 penalty), P2 AC@30min.
		{ContestID: "c", ProblemID: "p1", UserID: "u1", UserName: "Ada", Verdict: "wrong_answer", SubmittedAt: base.Add(5 * time.Minute)},
		{ContestID: "c", ProblemID: "p1", UserID: "u1", UserName: "Ada", Verdict: "accepted", SubmittedAt: base.Add(10 * time.Minute)},
		{ContestID: "c", ProblemID: "p2", UserID: "u1", UserName: "Ada", Verdict: "accepted", SubmittedAt: base.Add(30 * time.Minute)},
		// Bob: P1 AC@15min (no penalty).
		{ContestID: "c", ProblemID: "p1", UserID: "u2", UserName: "Bob", Verdict: "accepted", SubmittedAt: base.Add(15 * time.Minute)},
		// Cid submits nothing.
	}
	r := ComputeRankingACM(users, c, subs)
	if len(r) != 3 {
		t.Fatalf("all users ranked: %+v", r)
	}
	if r[0].UserName != "Ada" || r[0].Solved != 2 || r[0].Penalty != 60 {
		t.Fatalf("ada first: %+v", r[0])
	}
	if r[1].UserName != "Bob" || r[1].Solved != 1 || r[1].Penalty != 15 {
		t.Fatalf("bob second: %+v", r[1])
	}
	if r[2].UserName != "Cid" || r[2].Solved != 0 {
		t.Fatalf("cid last: %+v", r[2])
	}
}

func TestComputeRankingACMTieBreak(t *testing.T) {
	c := Contest{ID: "c"}
	users := []User{{ID: "u1", Name: "Zed"}, {ID: "u2", Name: "Amy"}}
	subs := []Submission{
		{ContestID: "c", ProblemID: "p1", UserID: "u1", UserName: "Zed", Verdict: "accepted", SubmittedAt: time.Now()},
		{ContestID: "c", ProblemID: "p1", UserID: "u2", UserName: "Amy", Verdict: "accepted", SubmittedAt: time.Now()},
	}
	r := ComputeRankingACM(users, c, subs)
	if r[0].UserName != "Amy" {
		t.Fatalf("name tiebreak: %+v", r)
	}
}
