// Package storetest 是 Store 端口的一致性套件：所有实现（memory/sqlite/…）
// 必须通过同一组契约，保证 app 层无感切换。
package storetest

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/ports"
)

// Exercise 在给定 Store 上跑全实体 CRUD + 排序 + 隔离 + ID 契约。
func Exercise(t *testing.T, s ports.Store) {
	t.Helper()

	// ---- users ----
	if _, err := s.GetUser("nope"); err == nil {
		t.Fatal("missing user should error")
	}
	u1, err := s.CreateUser(domain.User{Name: "Zed", Role: "student"})
	if err != nil {
		t.Fatal(err)
	}
	u2, err := s.CreateUser(domain.User{ID: "custom-u2", Name: "Ada", Role: "teacher"})
	if err != nil {
		t.Fatal(err)
	}
	if u2.ID != "custom-u2" {
		t.Fatal("preset ID must be kept")
	}
	if !strings.HasPrefix(u1.ID, "usr-") {
		t.Fatalf("user id prefix broken: %s", u1.ID)
	}
	users, err := s.ListUsers()
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 || users[0].Name != "Ada" {
		t.Fatalf("users sorted by name: %+v", users)
	}
	named, err := s.CreateUser(domain.User{
		ID: "named-u1", Username: "CaseUser", Name: "Named",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUser(domain.User{
		ID: "named-u2", Username: "caseuser", Name: "Duplicate",
	}); err == nil {
		t.Fatal("store must reject case-insensitive duplicate usernames")
	}
	other, err := s.CreateUser(domain.User{
		ID: "named-u3", Username: "OtherUser", Name: "Other",
	})
	if err != nil {
		t.Fatal(err)
	}
	other.Username = "caseuser"
	if err := s.UpdateUser(other); err == nil {
		t.Fatal("store must reject duplicate usernames on update")
	}
	if named.Username != "CaseUser" {
		t.Fatalf("unexpected named user: %+v", named)
	}
	u1.Groups = []string{"g1"}
	if err := s.UpdateUser(u1); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetUser(u1.ID)
	if len(got.Groups) != 1 {
		t.Fatal("user update lost")
	}
	if err := s.UpdateUser(domain.User{ID: "ghost"}); err == nil {
		t.Fatal("update ghost user should error")
	}

	// ---- groups ----
	g1, _ := s.CreateGroup(domain.Group{Name: "Camp"})
	if _, err := s.GetGroup("ghost"); err == nil {
		t.Fatal("missing group should error")
	}
	g1.UserIDs = []string{u1.ID}
	if err := s.UpdateGroup(g1); err != nil {
		t.Fatal(err)
	}
	gs, err := s.ListGroups()
	if err != nil || len(gs) != 1 {
		t.Fatalf("groups: %+v %v", gs, err)
	}

	// ---- contests ----
	now := time.Now().UTC()
	c1, _ := s.CreateContest(domain.Contest{Name: "Morning", Status: "draft", CreatedAt: now})
	c2, _ := s.CreateContest(domain.Contest{Name: "Afternoon", Status: "draft", CreatedAt: now.Add(time.Second)})
	cs, err := s.ListContests()
	if err != nil || len(cs) != 2 || cs[0].Name != "Morning" {
		t.Fatalf("contests sorted by creation: %+v %v", cs, err)
	}
	c1.Status = string(domain.Running)
	if err := s.UpdateContest(c1); err != nil {
		t.Fatal(err)
	}
	if c, _ := s.GetContest(c1.ID); c.Status != string(domain.Running) {
		t.Fatal("contest update lost")
	}
	_ = c2

	// ---- problems（比赛隔离）----
	p1, err := s.CreateProblem(domain.Problem{ContestID: c1.ID, Code: "B", Title: "T2"})
	if err != nil {
		t.Fatal(err)
	}
	p2, err := s.CreateProblem(domain.Problem{ContestID: c1.ID, Code: "A", Title: "T1"})
	if err != nil {
		t.Fatal(err)
	}
	_ = p1
	ps, err := s.ListProblems(c1.ID)
	if err != nil || len(ps) != 2 || ps[0].Code != "A" || ps[1].Code != "B" {
		t.Fatalf("problems sorted by code: %+v %v", ps, err)
	}
	c2Problems, err := s.ListProblems(c2.ID)
	if err != nil || len(c2Problems) != 0 {
		t.Fatalf("contest isolation broken: %+v %v", c2Problems, err)
	}
	p2.Title = "T1-renamed"
	if _, err := s.UpdateProblem(p2); err != nil {
		t.Fatal(err)
	}
	if p, _ := s.GetProblem(p2.ID); p.Title != "T1-renamed" {
		t.Fatal("problem update lost")
	}
	if err := s.DeleteProblem(p1.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetProblem(p1.ID); err == nil {
		t.Fatal("deleted problem should be gone")
	}
	c, _ := s.GetContest(c1.ID)
	if len(c.ProblemIDs) != 2 {
		t.Fatalf("contest.ProblemIDs not synced: %+v", c.ProblemIDs)
	}
	if _, err := s.GetProblem("ghost"); err == nil {
		t.Fatal("missing problem should error")
	}

	// ---- submissions（倒序）----
	base := time.Now().UTC()
	x1, _ := s.CreateSubmission(domain.Submission{ContestID: c1.ID, ProblemID: p2.ID, UserID: u1.ID, Score: 10, SubmittedAt: base})
	x2, _ := s.CreateSubmission(domain.Submission{ContestID: c1.ID, ProblemID: p2.ID, UserID: u1.ID, Score: 20, SubmittedAt: base.Add(time.Second)})
	subs, err := s.ListSubmissions(c1.ID)
	if err != nil || len(subs) != 2 || subs[0].ID != x2.ID || subs[1].ID != x1.ID {
		t.Fatalf("submissions must be newest-first: %+v %v", subs, err)
	}
	x2.Score = 100
	if err := s.UpdateSubmission(x2); err != nil {
		t.Fatal(err)
	}
	if x, _ := s.GetSubmission(x2.ID); x.Score != 100 {
		t.Fatal("submission update lost")
	}
	feed, err := s.ListSubmissionFeed(c1.ID, "", 1)
	if err != nil || len(feed) != 1 || feed[0].ID != x2.ID {
		t.Fatalf("limited submission feed = %+v, %v", feed, err)
	}
	userFeed, err := s.ListSubmissionFeed(c1.ID, u1.ID, 10)
	if err != nil || len(userFeed) != 2 {
		t.Fatalf("user submission feed = %+v, %v", userFeed, err)
	}
	if otherFeed, err := s.ListSubmissionFeed(c1.ID, "other-user", 10); err != nil || len(otherFeed) != 0 {
		t.Fatalf("foreign user feed = %+v, %v", otherFeed, err)
	}
	if _, err := s.GetSubmission("ghost"); err == nil {
		t.Fatal("missing submission should error")
	}
	queued, err := s.CreateSubmission(domain.Submission{
		ContestID:   c1.ID,
		ProblemID:   p2.ID,
		UserID:      u1.ID,
		Verdict:     domain.VerdictQueued,
		SubmittedAt: base.Add(2 * time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateSubmission(domain.Submission{
		ContestID:   c1.ID,
		ProblemID:   p2.ID,
		UserID:      u1.ID,
		Verdict:     domain.VerdictAccepted,
		SubmittedAt: base.Add(3 * time.Second),
	}); err != nil {
		t.Fatal(err)
	}
	queuedSubs, err := s.ListQueuedSubmissions()
	if err != nil || len(queuedSubs) != 1 || queuedSubs[0].ID != queued.ID {
		t.Fatalf("queued submissions = %+v, %v", queuedSubs, err)
	}
	queued.Verdict = domain.VerdictAccepted
	if err := s.UpdateSubmission(queued); err != nil {
		t.Fatal(err)
	}
	if queuedSubs, err := s.ListQueuedSubmissions(); err != nil || len(queuedSubs) != 0 {
		t.Fatalf("judged submission leaked into recovery queue: %+v, %v", queuedSubs, err)
	}
	requestSubmission, err := s.CreateSubmission(domain.Submission{
		ContestID:   c1.ID,
		ProblemID:   p2.ID,
		UserID:      u1.ID,
		RequestID:   "request-1",
		SubmittedAt: base.Add(2 * time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}
	found, err := s.FindSubmissionByRequestID("request-1")
	if err != nil || found == nil || found.ID != requestSubmission.ID {
		t.Fatalf("request id lookup failed: %+v %v", found, err)
	}
	if missing, err := s.FindSubmissionByRequestID("missing-request"); err != nil || missing != nil {
		t.Fatalf("missing request id lookup = %+v, %v", missing, err)
	}
	if _, err := s.CreateSubmission(domain.Submission{
		ContestID:   c1.ID,
		ProblemID:   p2.ID,
		UserID:      u1.ID,
		RequestID:   "request-1",
		SubmittedAt: base.Add(3 * time.Second),
	}); err == nil {
		t.Fatal("duplicate request id must be rejected")
	}

	// ---- NextID 唯一递增 ----
	seen := map[string]bool{}
	for i := 0; i < 5; i++ {
		id := s.NextID("tst")
		if seen[id] {
			t.Fatalf("duplicate id %s", id)
		}
		seen[id] = true
		if !strings.HasPrefix(id, "tst-") {
			t.Fatalf("id prefix broken: %s", id)
		}
	}
}

// PasswordsAndSessions 凭证与会话契约：hash 缺席为空串，会话 CRUD。
func PasswordsAndSessions(t *testing.T, s ports.Store) {
	t.Helper()
	u, err := s.CreateUser(domain.User{Name: "Auth", Role: "student"})
	if err != nil {
		t.Fatal(err)
	}
	if h, err := s.GetPasswordHash(u.ID); err != nil || h != "" {
		t.Fatalf("absent hash should be empty: %q %v", h, err)
	}
	if err := s.SetPasswordHash("ghost", "x"); err == nil {
		t.Fatal("ghost password set should error")
	}
	if err := s.SetPasswordHash(u.ID, "bcrypt-hash"); err != nil {
		t.Fatal(err)
	}
	if h, _ := s.GetPasswordHash(u.ID); h != "bcrypt-hash" {
		t.Fatal("hash round-trip broken")
	}
	if _, err := s.GetSession("ghost"); err == nil {
		t.Fatal("ghost session should error")
	}
	sess := domain.Session{Token: "tok-1", UserID: u.ID, Role: "student"}
	if err := s.SaveSession(sess); err != nil {
		t.Fatal(err)
	}
	if got, err := s.GetSession("tok-1"); err != nil || got.UserID != u.ID {
		t.Fatalf("session round-trip: %+v %v", got, err)
	}
	if err := s.DeleteSession("tok-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetSession("tok-1"); err == nil {
		t.Fatal("deleted session should error")
	}
	_ = s.SaveSession(domain.Session{Token: "a", UserID: u.ID})
	_ = s.SaveSession(domain.Session{Token: "b", UserID: u.ID})
	if err := s.DeleteSessionsForUser(u.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetSession("a"); err == nil {
		t.Fatal("user sessions should be wiped")
	}
	now := time.Now().UTC()
	if err := s.SaveSession(domain.Session{
		Token: "expired", UserID: u.ID, ExpiresAt: now.Add(-time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSession(domain.Session{
		Token: "live", UserID: u.ID, ExpiresAt: now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	deleted, err := s.DeleteExpiredSessions(now)
	if err != nil || deleted != 1 {
		t.Fatalf("expired session prune: deleted=%d err=%v", deleted, err)
	}
	if _, err := s.GetSession("expired"); err == nil {
		t.Fatal("expired session survived prune")
	}
	if _, err := s.GetSession("live"); err != nil {
		t.Fatal("live session was pruned")
	}
	_ = s.DeleteSession("live")
}

// TransactionSemantics 验证事务提交与回滚语义。
func TransactionSemantics(t *testing.T, s ports.Store) {
	t.Helper()
	txer, ok := s.(ports.Transactor)
	if !ok {
		return
	}
	ctx := time.Now()
	_ = ctx

	// 1. 事务回滚：中间出错，之前的操作必须全部撤回
	err := txer.WithTx(nil, func(tx ports.Store) error {
		if _, err := tx.CreateUser(domain.User{ID: "tx-u1", Name: "TxUser1"}); err != nil {
			return err
		}
		return errors.New("abort transaction")
	})
	if err == nil {
		t.Fatal("expected error from WithTx")
	}
	if _, err := s.GetUser("tx-u1"); err == nil {
		t.Fatal("tx-u1 should have been rolled back")
	}

	// 2. 事务提交：成功后持久化
	err = txer.WithTx(nil, func(tx ports.Store) error {
		if _, err := tx.CreateUser(domain.User{ID: "tx-u2", Name: "TxUser2"}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if u, err := s.GetUser("tx-u2"); err != nil || u.Name != "TxUser2" {
		t.Fatalf("tx-u2 should exist after commit: %+v %v", u, err)
	}
}
