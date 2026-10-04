package auth

import (
	"testing"
	"time"

	"github.com/yviscool/forge/internal/adapters/store/memory"
	"github.com/yviscool/forge/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

func init() { Cost = bcrypt.MinCost }

func setup(t *testing.T) (*Service, domain.User) {
	t.Helper()
	svc := New(memory.New(), nil)
	u, err := svc.store.CreateUser(domain.User{Username: "alice", Name: "Alice", Role: "student"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ResetPassword(u.ID, "secret1"); err != nil {
		t.Fatal(err)
	}
	return svc, u
}

func TestLogin(t *testing.T) {
	svc, u := setup(t)
	sess, err := svc.Login("alice", "secret1")
	if err != nil {
		t.Fatal(err)
	}
	if sess.UserID != u.ID || sess.Token == "" {
		t.Fatalf("bad session: %+v", sess)
	}
	// 按姓名也可登。
	if _, err := svc.Login("Alice", "secret1"); err != nil {
		t.Fatalf("login by name: %v", err)
	}
	if _, err := svc.Login("alice", "wrong"); err == nil {
		t.Fatal("wrong password must fail")
	}
	if _, err := svc.Login("ghost", "x"); err == nil {
		t.Fatal("unknown user must fail")
	}
}

func TestLoginNoPassword(t *testing.T) {
	svc := New(memory.New(), nil)
	u, _ := svc.store.CreateUser(domain.User{Username: "nopw", Name: "NoPw"})
	if _, err := svc.Login(u.ID, "x"); err == nil {
		t.Fatal("unset password must fail")
	}
	_ = u
}

func TestAuthenticateExpiry(t *testing.T) {
	svc, _ := setup(t)
	sess, _ := svc.Login("alice", "secret1")
	if _, err := svc.Authenticate(sess.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate("bogus"); err == nil {
		t.Fatal("bogus token must fail")
	}
	// 过期惰性清理。
	old := SessionTTL
	SessionTTL = -time.Second
	defer func() { SessionTTL = old }()
	s2, _ := svc.Login("alice", "secret1")
	if _, err := svc.Authenticate(s2.Token); err == nil {
		t.Fatal("expired must fail")
	}
	if _, err := svc.store.GetSession(s2.Token); err == nil {
		t.Fatal("expired session should be cleaned")
	}
}

func TestChangeAndResetPassword(t *testing.T) {
	svc, u := setup(t)
	sess, _ := svc.Login("alice", "secret1")
	if err := svc.ChangePassword(u.ID, "nope", "newpw1"); err == nil {
		t.Fatal("wrong old password must fail")
	}
	if err := svc.ChangePassword(u.ID, "secret1", "abc"); err == nil {
		t.Fatal("short password must fail")
	}
	if err := svc.ChangePassword(u.ID, "secret1", "newpw12"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate(sess.Token); err == nil {
		t.Fatal("old session must be wiped after change")
	}
	if _, err := svc.Login("alice", "newpw12"); err != nil {
		t.Fatalf("new password: %v", err)
	}
	if err := svc.ResetPassword(u.ID, "reset123"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Logout("whatever"); err != nil {
		t.Fatal("logout must be idempotent")
	}
}

func TestEnsureAdmin(t *testing.T) {
	svc := New(memory.New(), nil)
	a1, err := svc.EnsureAdmin("pw-admin")
	if err != nil {
		t.Fatal(err)
	}
	a2, err := svc.EnsureAdmin("pw-admin")
	if err != nil || a1.ID != a2.ID {
		t.Fatal("ensure must be idempotent")
	}
	if _, err := svc.Login("admin", "pw-admin"); err != nil {
		t.Fatalf("admin login: %v", err)
	}
}
