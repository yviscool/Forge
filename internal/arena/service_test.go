package arena

import "testing"

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
	p, err := s.AddProblem(Problem{ContestID: c.ID, Code: "A", Title: "Sum", Statement: "a+b"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Submit(Submission{ContestID: c.ID, ProblemID: p.ID, UserID: u.ID, Code: "1", Language: "go"}); err == nil {
		t.Fatal("expected stopped contest rejection")
	}
	if _, err = s.StartContest(c.ID); err != nil {
		t.Fatal(err)
	}
	sub, err := s.Submit(Submission{ContestID: c.ID, ProblemID: p.ID, UserID: u.ID, Code: "1", Language: "go"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Judge(sub.ID, "accepted", 100); err != nil {
		t.Fatal(err)
	}
	r := s.Ranking(c.ID)
	if len(r) != 1 || r[0].Score != 100 || r[0].Accepted != 1 {
		t.Fatalf("unexpected ranking: %#v", r)
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
}
