package auth

import (
	"testing"
	"time"

	"github.com/yviscool/forge/internal/adapters/store/memory"
	"github.com/yviscool/forge/internal/domain"
)

func newGrantFixture(t *testing.T) (*Service, string, string) {
	t.Helper()
	store := memory.New()
	u, err := store.CreateUser(domain.User{Username: "alice", Name: "Alice", Role: domain.RoleStudent})
	if err != nil {
		t.Fatal(err)
	}
	rawToken := "session-token"
	if err := store.SaveSession(domain.Session{
		Token: hashSessionToken(rawToken), UserID: u.ID, Role: u.Role,
		ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	return New(store, nil), rawToken, u.ID
}

func TestStreamTicketRoundTripAndBindings(t *testing.T) {
	svc, sessionToken, userID := newGrantFixture(t)
	ticket, expires := svc.IssueStreamTicket(userID, "cnt-1", sessionToken, time.Minute)
	if !expires.After(time.Now()) {
		t.Fatalf("ticket expiry was not in the future: %v", expires)
	}

	claims, err := svc.ValidateStreamTicket(ticket, "cnt-1")
	if err != nil {
		t.Fatalf("valid ticket rejected: %v", err)
	}
	if claims.UserID != userID || claims.ContestID != "cnt-1" {
		t.Fatalf("unexpected claims: %#v", claims)
	}

	if _, err := svc.ValidateStreamTicket(ticket, "cnt-2"); err == nil {
		t.Fatal("ticket was accepted for a different contest")
	}
	if _, err := svc.ValidateStreamTicket(ticket+"x", "cnt-1"); err == nil {
		t.Fatal("tampered ticket was accepted")
	}
	if _, err := New(memory.New(), nil).ValidateStreamTicket(ticket, "cnt-1"); err == nil {
		t.Fatal("ticket signed by another process secret was accepted")
	}
}

type ticketClock struct {
	now time.Time
}

func (c *ticketClock) Now() time.Time { return c.now }

func TestStreamTicketExpires(t *testing.T) {
	clock := &ticketClock{now: time.Date(2026, 10, 21, 8, 0, 0, 0, time.UTC)}
	store := memory.New()
	u, _ := store.CreateUser(domain.User{Username: "alice", Role: domain.RoleStudent})
	rawToken := "session-token"
	_ = store.SaveSession(domain.Session{
		Token: hashSessionToken(rawToken), UserID: u.ID, Role: u.Role,
		ExpiresAt: clock.now.Add(time.Hour),
	})
	svc := New(store, clock)
	ticket, _ := svc.IssueStreamTicket(u.ID, "cnt-1", rawToken, time.Minute)
	clock.now = clock.now.Add(2 * time.Minute)
	if _, err := svc.ValidateStreamTicket(ticket, "cnt-1"); err == nil {
		t.Fatal("expired ticket was accepted")
	}
}

func TestDefaultStreamTicketCoversAFullExamDay(t *testing.T) {
	svc, sessionToken, userID := newGrantFixture(t)
	_, expires := svc.IssueStreamTicket(userID, "cnt-1", sessionToken, 0)
	if remaining := time.Until(expires); remaining < 11*time.Hour || remaining > 12*time.Hour {
		t.Fatalf("default ticket TTL should cover a full contest day, got %v", remaining)
	}
}

func TestGrantsAreRevokedByLogoutAndPasswordRotation(t *testing.T) {
	svc, sessionToken, userID := newGrantFixture(t)
	streamTicket, _ := svc.IssueStreamTicket(userID, "cnt-1", sessionToken, time.Hour)
	passwordBinding := svc.ZipPasswordBinding("zip-secret")
	zipTicket, _, err := svc.IssueZipGrant(
		userID, "cnt-1", sessionToken, passwordBinding, time.Hour,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ValidateZipGrant(
		zipTicket, userID, "cnt-1", svc.ZipPasswordBinding("zip-secret"),
	); err != nil {
		t.Fatalf("valid zip grant rejected: %v", err)
	}
	if _, err := svc.ValidateZipGrant(
		zipTicket, userID, "cnt-1", svc.ZipPasswordBinding("rotated"),
	); err == nil {
		t.Fatal("zip grant survived password rotation")
	}

	if err := svc.Logout(sessionToken); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ValidateStreamTicket(streamTicket, "cnt-1"); err == nil {
		t.Fatal("stream grant survived logout")
	}
	if _, err := svc.ValidateZipGrant(
		zipTicket, userID, "cnt-1", svc.ZipPasswordBinding("zip-secret"),
	); err == nil {
		t.Fatal("zip grant survived logout")
	}
}
