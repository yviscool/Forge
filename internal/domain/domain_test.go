package domain

import "testing"

func TestValidateProblem(t *testing.T) {
	if err := ValidateProblem(Problem{ContestID: "c", Code: "A", Title: "T", Statement: "s", Input: "i", Output: "o", Constraints: "k"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateProblem(Problem{Code: "A"}); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestIsUserAllowed(t *testing.T) {
	open := Contest{ID: "c"}
	if !IsUserAllowed(open, User{ID: "u"}) {
		t.Fatal("open contest should allow all")
	}
	restricted := Contest{ID: "c", ParticipantUserIDs: []string{"u1"}, GroupIDs: []string{"g1"}}
	if !IsUserAllowed(restricted, User{ID: "u1"}) {
		t.Fatal("direct participant should pass")
	}
	if !IsUserAllowed(restricted, User{ID: "u2", Groups: []string{"g1"}}) {
		t.Fatal("group member should pass")
	}
	if IsUserAllowed(restricted, User{ID: "u3"}) {
		t.Fatal("outsider should be rejected")
	}
}

func TestComputeRanking(t *testing.T) {
	c := Contest{ID: "c"}
	users := []User{{ID: "u1", Name: "Bob"}, {ID: "u2", Name: "Ada"}}
	subs := []Submission{
		{ContestID: "c", ProblemID: "p1", UserID: "u1", UserName: "Bob", Score: 60},
		{ContestID: "c", ProblemID: "p1", UserID: "u1", UserName: "Bob", Score: 100},
		{ContestID: "c", ProblemID: "p1", UserID: "u2", UserName: "Ada", Score: 100},
	}
	r := ComputeRanking(users, c, subs)
	if len(r) != 2 || r[0].UserName != "Ada" {
		t.Fatalf("deterministic order broken: %#v", r)
	}
}
