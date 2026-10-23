package auth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yviscool/forge/internal/adapters/store/memory"
	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/ports"
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
	if err := svc.ResetPassword(u.ID, "secret-pass-1"); err != nil {
		t.Fatal(err)
	}
	return svc, u
}

func TestLogin(t *testing.T) {
	svc, u := setup(t)
	sess, err := svc.Login("alice", "secret-pass-1")
	if err != nil {
		t.Fatal(err)
	}
	if sess.UserID != u.ID || sess.Token == "" {
		t.Fatalf("bad session: %+v", sess)
	}
	if _, err := svc.store.GetSession(sess.Token); err == nil {
		t.Fatal("raw session token must never be persisted")
	}
	if stored, err := svc.store.GetSession(hashSessionToken(sess.Token)); err != nil ||
		stored.UserID != u.ID {
		t.Fatalf("hashed session missing: %+v %v", stored, err)
	}
	// 按姓名也可登。
	if _, err := svc.Login("Alice", "secret-pass-1"); err != nil {
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

func TestNameLoginRequiresUniqueMatch(t *testing.T) {
	svc := New(memory.New(), nil)
	first, err := svc.store.CreateUser(domain.User{Username: "s001", Name: "张伟", Role: "student"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.store.CreateUser(domain.User{Username: "s002", Name: "张伟", Role: "student"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ResetPassword(first.ID, "first-password"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ResetPassword(second.ID, "second-password"); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Login("张伟", "first-password"); err == nil {
		t.Fatal("ambiguous display name must not sign into an arbitrary account")
	}
	if sess, err := svc.Login("s002", "second-password"); err != nil || sess.UserID != second.ID {
		t.Fatalf("unique username login failed: %+v %v", sess, err)
	}
}

func TestAuthenticateExpiry(t *testing.T) {
	svc, _ := setup(t)
	sess, _ := svc.Login("alice", "secret-pass-1")
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
	s2, _ := svc.Login("alice", "secret-pass-1")
	if _, err := svc.Authenticate(s2.Token); err == nil {
		t.Fatal("expired must fail")
	}
	if _, err := svc.store.GetSession(hashSessionToken(s2.Token)); err == nil {
		t.Fatal("expired session should be cleaned")
	}
}

func TestAuthenticateUsesCurrentUserRole(t *testing.T) {
	svc, u := setup(t)
	if err := svc.store.UpdateUser(domain.User{
		ID: u.ID, Username: u.Username, Name: u.Name, Role: domain.RoleTeacher,
		Groups: u.Groups, CreatedAt: u.CreatedAt,
	}); err != nil {
		t.Fatal(err)
	}
	sess, err := svc.Login("alice", "secret-pass-1")
	if err != nil {
		t.Fatal(err)
	}
	if sess.Role != domain.RoleTeacher {
		t.Fatalf("login role = %q, want teacher", sess.Role)
	}

	if err := svc.store.UpdateUser(domain.User{
		ID: u.ID, Username: u.Username, Name: u.Name, Role: domain.RoleStudent,
		Groups: u.Groups, CreatedAt: u.CreatedAt,
	}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Authenticate(sess.Token)
	if err != nil {
		t.Fatal(err)
	}
	if got.Role != domain.RoleStudent {
		t.Fatalf("existing session kept stale role %q", got.Role)
	}
}

type missingUserStore struct {
	ports.Store
	userID string
}

func (s missingUserStore) GetUser(id string) (domain.User, error) {
	if id == s.userID {
		return domain.User{}, errors.New("user not found")
	}
	return s.Store.GetUser(id)
}

func TestAuthenticateDeletesSessionForRemovedUser(t *testing.T) {
	svc, u := setup(t)
	sess, err := svc.Login("alice", "secret-pass-1")
	if err != nil {
		t.Fatal(err)
	}
	base := svc.store
	svc.store = missingUserStore{Store: base, userID: u.ID}

	if _, err := svc.Authenticate(sess.Token); err == nil {
		t.Fatal("removed user session must fail authentication")
	}
	svc.store = base
	if _, err := svc.store.GetSession(hashSessionToken(sess.Token)); err == nil {
		t.Fatal("removed user session must be deleted")
	}
}

func TestChangeAndResetPassword(t *testing.T) {
	svc, u := setup(t)
	sess, _ := svc.Login("alice", "secret-pass-1")
	if err := svc.ChangePassword(u.ID, "nope", "newpw1"); err == nil {
		t.Fatal("wrong old password must fail")
	}
	if err := svc.ChangePassword(u.ID, "secret-pass-1", "abc"); err == nil {
		t.Fatal("short password must fail")
	}
	if err := svc.ChangePassword(u.ID, "secret-pass-1", "new-password-12"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate(sess.Token); err == nil {
		t.Fatal("old session must be wiped after change")
	}
	if _, err := svc.Login("alice", "new-password-12"); err != nil {
		t.Fatalf("new password: %v", err)
	}
	if err := svc.ResetPassword(u.ID, "reset-password-123"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Logout("whatever"); err != nil {
		t.Fatal("logout must be idempotent")
	}
}

func TestPasswordPolicyRejectsBcryptOversizeInput(t *testing.T) {
	if err := ValidatePassword(strings.Repeat("a", MaxPasswordBytes)); err != nil {
		t.Fatalf("%d-byte password should be valid: %v", MaxPasswordBytes, err)
	}
	if err := ValidatePassword(strings.Repeat("a", MaxPasswordBytes+1)); err == nil {
		t.Fatal("password beyond bcrypt's 72-byte limit must be rejected")
	}
}

func TestEnsureAdmin(t *testing.T) {
	svc := New(memory.New(), nil)
	a1, err := svc.EnsureAdmin("admin-pass-123")
	if err != nil {
		t.Fatal(err)
	}
	a2, err := svc.EnsureAdmin("admin-pass-123")
	if err != nil || a1.ID != a2.ID {
		t.Fatal("ensure must be idempotent")
	}
	if _, err := svc.Login("admin", "admin-pass-123"); err != nil {
		t.Fatalf("admin login: %v", err)
	}
}

func TestEnsureAdminRejectsShortPassword(t *testing.T) {
	svc := New(memory.New(), nil)
	if _, err := svc.EnsureAdmin("short"); err == nil {
		t.Fatal("bootstrap admin must enforce the password policy")
	}
	users, err := svc.store.ListUsers()
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 0 {
		t.Fatalf("rejected bootstrap must not create an account: %+v", users)
	}
}

func TestEnsureAdminDoesNotRevalidateExistingAccount(t *testing.T) {
	svc := New(memory.New(), nil)
	original, err := svc.EnsureAdmin("admin-pass-123")
	if err != nil {
		t.Fatal(err)
	}
	again, err := svc.EnsureAdmin("short")
	if err != nil {
		t.Fatalf("existing admin should be returned without password validation: %v", err)
	}
	if again.ID != original.ID {
		t.Fatalf("unexpected admin account: %+v", again)
	}
	if _, err := svc.Login("admin", "admin-pass-123"); err != nil {
		t.Fatalf("existing password must remain unchanged: %v", err)
	}
}
