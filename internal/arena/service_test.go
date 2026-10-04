package arena

import (
	"testing"
)

func TestContestSubmissionRanking(t *testing.T) {
	s := NewService()
	u, err := s.CreateUser("Ada", "student")
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.CreateContest("Spring", "practice")
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.AddProblem(Problem{
		ContestID: c.ID,
		Code:      "A",
		Title:     "Sum",
		Statement: "a+b",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Cannot submit to draft contest
	if _, err = s.Submit(Submission{ContestID: c.ID, ProblemID: p.ID, UserID: u.ID, Code: "1", Language: "go"}); err == nil {
		t.Fatal("expected stopped contest rejection")
	}

	// Start contest
	if _, err = s.StartContest(c.ID); err != nil {
		t.Fatal(err)
	}

	// Submit code
	sub, err := s.Submit(Submission{ContestID: c.ID, ProblemID: p.ID, UserID: u.ID, Code: "1", Language: "go"})
	if err != nil {
		t.Fatal(err)
	}

	// Judge submission
	if _, err = s.Judge(sub.ID, "accepted", 100); err != nil {
		t.Fatal(err)
	}

	// Check ranking
	r := s.Ranking(c.ID)
	if len(r) != 1 || r[0].Score != 100 || r[0].Accepted != 1 {
		t.Fatalf("unexpected ranking: %#v", r)
	}

	// Finish contest
	finished, err := s.FinishContest(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if finished.Status != string(Finished) {
		t.Fatalf("expected finished status, got %s", finished.Status)
	}
}

func TestValidation(t *testing.T) {
	s := NewService()
	if _, err := s.CreateContest("", ""); err == nil {
		t.Fatal("empty contest should fail")
	}
	if _, err := s.CreateUser("x", "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateGroup(""); err == nil {
		t.Fatal("empty group name should fail")
	}

	// Problem validation
	invalidProb := Problem{
		ContestID: "cnt-0001",
		Code:      "A",
		Title:     "Reverse",
		// missing statement, input, output, constraints
	}
	if err := s.ValidateProblem(invalidProb); err == nil {
		t.Fatal("expected problem validation failure on missing fields")
	}

	validProb := Problem{
		ContestID:   "cnt-0001",
		Code:        "A",
		Title:       "Reverse",
		Statement:   "Reverse the binary string",
		Input:       "One line containing string s",
		Output:      "One line containing reversed string",
		Constraints: "1 <= |s| <= 10^5",
	}
	if err := s.ValidateProblem(validProb); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestIndependentUsersAndGroups(t *testing.T) {
	s := NewService()

	u1, err := s.CreateUser("Alice", "student")
	if err != nil {
		t.Fatal(err)
	}
	u2, err := s.CreateUser("Bob", "student")
	if err != nil {
		t.Fatal(err)
	}

	g1, err := s.CreateGroup("Training Camp 2026")
	if err != nil {
		t.Fatal(err)
	}

	if err := s.AddUserToGroup(u1.ID, g1.ID); err != nil {
		t.Fatal(err)
	}

	updatedG1, err := s.GetGroup(g1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(updatedG1.UserIDs) != 1 || updatedG1.UserIDs[0] != u1.ID {
		t.Fatalf("expected u1 in g1, got %v", updatedG1.UserIDs)
	}

	// Remove user from group
	if err := s.RemoveUserFromGroup(u1.ID, g1.ID); err != nil {
		t.Fatal(err)
	}
	updatedG1, _ = s.GetGroup(g1.ID)
	if len(updatedG1.UserIDs) != 0 {
		t.Fatalf("expected 0 users in g1, got %d", len(updatedG1.UserIDs))
	}

	// Add both users
	_ = s.AddUserToGroup(u1.ID, g1.ID)
	_ = s.AddUserToGroup(u2.ID, g1.ID)

	// Contest participant access checking
	c, _ := s.CreateContest("Restricted Match", "Invite only")
	p, _ := s.AddProblem(Problem{ContestID: c.ID, Code: "P1", Title: "Problem 1", Statement: "Solve it"})
	_, _ = s.StartContest(c.ID)

	// Bind group to contest
	if err := s.AddGroupToContest(c.ID, g1.ID); err != nil {
		t.Fatal(err)
	}

	// u1 and u2 can submit
	sub1, err := s.Submit(Submission{ContestID: c.ID, ProblemID: p.ID, UserID: u1.ID, Code: "cpp", Language: "cpp"})
	if err != nil {
		t.Fatalf("u1 should be allowed: %v", err)
	}
	if sub1.ID == "" {
		t.Fatal("empty submission ID")
	}

	// Unrelated user cannot submit
	u3, _ := s.CreateUser("Charlie", "student")
	if _, err := s.Submit(Submission{ContestID: c.ID, ProblemID: p.ID, UserID: u3.ID, Code: "cpp", Language: "cpp"}); err == nil {
		t.Fatal("u3 should be rejected from restricted contest")
	}

	// Explicitly add u3 as individual participant
	if err := s.AddParticipantToContest(c.ID, u3.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Submit(Submission{ContestID: c.ID, ProblemID: p.ID, UserID: u3.ID, Code: "cpp", Language: "cpp"}); err != nil {
		t.Fatalf("u3 should now be allowed: %v", err)
	}
}

func TestMultiContestConcurrency(t *testing.T) {
	s := NewService()
	c1, _ := s.CreateContest("Contest 1", "Morning")
	c2, _ := s.CreateContest("Contest 2", "Afternoon")

	p1, _ := s.AddProblem(Problem{ContestID: c1.ID, Code: "A", Title: "Prob 1A", Statement: "Do 1A"})
	p2, _ := s.AddProblem(Problem{ContestID: c2.ID, Code: "A", Title: "Prob 2A", Statement: "Do 2A"})

	u, _ := s.CreateUser("Dave", "student")

	// Start both contests in parallel
	_, err1 := s.StartContest(c1.ID)
	_, err2 := s.StartContest(c2.ID)
	if err1 != nil || err2 != nil {
		t.Fatalf("both contests should start: %v, %v", err1, err2)
	}

	// Submit to both
	s1, err := s.Submit(Submission{ContestID: c1.ID, ProblemID: p1.ID, UserID: u.ID, Code: "1", Language: "cpp"})
	if err != nil {
		t.Fatal(err)
	}
	s2, err := s.Submit(Submission{ContestID: c2.ID, ProblemID: p2.ID, UserID: u.ID, Code: "2", Language: "cpp"})
	if err != nil {
		t.Fatal(err)
	}

	// Judge independently
	_, _ = s.Judge(s1.ID, "accepted", 100)
	_, _ = s.Judge(s2.ID, "accepted", 60)

	// Check independent rankings
	r1 := s.Ranking(c1.ID)
	r2 := s.Ranking(c2.ID)

	if len(r1) != 1 || r1[0].Score != 100 {
		t.Fatalf("c1 ranking mismatch: %#v", r1)
	}
	if len(r2) != 1 || r2[0].Score != 60 {
		t.Fatalf("c2 ranking mismatch: %#v", r2)
	}
}
