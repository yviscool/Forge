package sqlite

import (
	"path/filepath"
	"testing"

	"github.com/yviscool/forge/internal/adapters/store/storetest"
	"github.com/yviscool/forge/internal/domain"
)

func TestStoreConformance(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "forge.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	storetest.Exercise(t, s)
	storetest.PasswordsAndSessions(t, s)
	storetest.TransactionSemantics(t, s)
}

func TestPersistenceAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "forge.db")

	s1, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := s1.CreateUser(domain.User{Name: "Ada", Role: "student"})
	c, _ := s1.CreateContest(domain.Contest{Name: "CSP", Status: "draft"})
	p, _ := s1.CreateProblem(domain.Problem{ContestID: c.ID, Code: "A", Title: "和"})
	x, _ := s1.CreateSubmission(domain.Submission{ContestID: c.ID, ProblemID: p.ID, UserID: u.ID, Score: 42})
	if err := s1.Close(); err != nil {
		t.Fatal(err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if _, err := s2.GetUser(u.ID); err != nil {
		t.Fatalf("user lost: %v", err)
	}
	if cc, err := s2.GetContest(c.ID); err != nil || len(cc.ProblemIDs) != 1 {
		t.Fatalf("contest lost: %v %+v", err, cc)
	}
	if xx, err := s2.GetSubmission(x.ID); err != nil || xx.Score != 42 {
		t.Fatalf("submission lost: %v %+v", err, xx)
	}
	// seq 计数器不断流：新 ID 不得与已有关联。
	if id := s2.NextID("usr"); id == u.ID {
		t.Fatalf("seq reset after reopen: %s", id)
	}
}
