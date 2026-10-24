// Package auth 身份与会话：bcrypt 凭证 + token 会话 + 种子 admin。
// 密码 hash 永不进 domain.User，只走 Store 凭证端口。
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/ports"
	"golang.org/x/crypto/bcrypt"
)

// Cost bcrypt 轮数（测试可调低）。
var Cost = bcrypt.DefaultCost

// SessionTTL 会话有效期。
var SessionTTL = 7 * 24 * time.Hour

// MinPasswordLength is the launch baseline for every account, including
// teacher bootstrap and reset flows.
const MinPasswordLength = 10

// bcrypt rejects inputs longer than 72 bytes. Enforce the same bound at the
// policy boundary so every password that passes validation is hashable.
const MaxPasswordBytes = 72

// StreamTicketTTL deliberately covers a full contest day. The grant remains
// contest-scoped and is reissued by the browser on every reconnect.
const StreamTicketTTL = 12 * time.Hour

type Service struct {
	store        ports.Store
	clock        ports.Clock
	ticketSecret []byte
	dummyHash    []byte
}

func New(store ports.Store, clock ports.Clock) *Service {
	if clock == nil {
		clock = ports.SystemClock{}
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		// A process without a working CSPRNG cannot safely issue credentials.
		panic("auth: unable to initialize stream ticket secret")
	}
	// Compare against a real bcrypt hash even when the login does not resolve
	// to an account. This removes the easy "unknown username is much faster"
	// timing oracle without relying on a hard-coded cost that tests cannot vary.
	dummyHash, err := bcrypt.GenerateFromPassword([]byte("forge-dummy-password"), Cost)
	if err != nil {
		panic("auth: unable to initialize dummy password hash")
	}
	return &Service{
		store: store, clock: clock, ticketSecret: secret, dummyHash: dummyHash,
	}
}

func newToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// hashSessionToken prevents a database leak from becoming an immediate
// account-takeover primitive. The raw bearer value is returned to the client
// once; persistence only keeps its SHA-256 digest.
func hashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func (a *Service) findByLogin(login string) (domain.User, error) {
	login = strings.TrimSpace(login)
	users, err := a.store.ListUsers()
	if err != nil {
		return domain.User{}, err
	}
	var named []domain.User
	for _, u := range users {
		if u.Username == login {
			return u, nil
		}
		if u.Name == login {
			named = append(named, u)
		}
	}
	// Display names are not unique. Only permit name-based login when the
	// match is unambiguous; otherwise account selection would depend on map
	// iteration order and could sign a student into the wrong identity.
	if len(named) == 1 {
		return named[0], nil
	}
	return domain.User{}, errors.New("invalid login or password")
}

// Login 校验凭证并签发会话。
func (a *Service) Login(login, password string) (domain.Session, error) {
	u, err := a.findByLogin(login)
	if err != nil {
		_ = bcrypt.CompareHashAndPassword(a.dummyHash, []byte(password))
		return domain.Session{}, errors.New("invalid login or password")
	}
	hash, err := a.store.GetPasswordHash(u.ID)
	if err != nil || hash == "" {
		_ = bcrypt.CompareHashAndPassword(a.dummyHash, []byte(password))
		return domain.Session{}, errors.New("invalid login or password")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return domain.Session{}, errors.New("invalid login or password")
	}
	tok, err := newToken()
	if err != nil {
		return domain.Session{}, err
	}
	now := a.clock.Now()
	sess := domain.Session{
		Token: tok, UserID: u.ID, Role: u.Role,
		CreatedAt: now, ExpiresAt: now.Add(SessionTTL),
	}
	stored := sess
	stored.Token = hashSessionToken(tok)
	if err := a.store.SaveSession(stored); err != nil {
		return domain.Session{}, err
	}
	return sess, nil
}

// SessionForUser 免密为既有用户签发会话（桌面壳一次性令牌交换专用；
// 不经过 HTTP 公开路由，调用方必须已完成持有者校验）。
func (a *Service) SessionForUser(userID string) (domain.Session, error) {
	u, err := a.store.GetUser(userID)
	if err != nil {
		return domain.Session{}, err
	}
	tok, err := newToken()
	if err != nil {
		return domain.Session{}, err
	}
	now := a.clock.Now()
	sess := domain.Session{
		Token: tok, UserID: u.ID, Role: u.Role,
		CreatedAt: now, ExpiresAt: now.Add(SessionTTL),
	}
	stored := sess
	stored.Token = hashSessionToken(tok)
	if err := a.store.SaveSession(stored); err != nil {
		return domain.Session{}, err
	}
	return sess, nil
}

// Authenticate 校验 token 有效性（含过期惰性清理）。
func (a *Service) Authenticate(token string) (domain.Session, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return domain.Session{}, errors.New("unauthorized")
	}
	hashed := hashSessionToken(token)
	sess, err := a.store.GetSession(hashed)
	if err != nil {
		return domain.Session{}, errors.New("unauthorized")
	}
	if sess.Expired(a.clock.Now()) {
		_ = a.store.DeleteSession(hashed)
		return domain.Session{}, errors.New("session expired")
	}
	// Sessions are authentication facts, not an authorization cache. Reload the
	// account on every request so a role change or account removal takes effect
	// immediately instead of surviving until the seven-day session expires.
	u, err := a.store.GetUser(sess.UserID)
	if err != nil {
		_ = a.store.DeleteSession(hashed)
		return domain.Session{}, errors.New("unauthorized")
	}
	sess.Role = domain.NormalizeRole(u.Role)
	// Callers hold the presented credential, not the persisted digest. Keeping
	// it in the request context lets logout revoke the exact session.
	sess.Token = token
	return sess, nil
}

// Logout 注销单会话（幂等）。
func (a *Service) Logout(token string) error {
	return a.store.DeleteSession(hashSessionToken(token))
}

// StreamClaims is a short-lived, contest-scoped authorization grant for the
// browser EventSource transport. It is intentionally independent of the
// long-lived session token so credentials never appear in URLs or logs.
type StreamClaims struct {
	UserID    string `json:"uid"`
	ContestID string `json:"cid"`
	SessionID string `json:"sid"`
	ExpiresAt int64  `json:"exp"`
}

// ZipClaims authorizes one candidate to read a protected contest paper. It is
// separate from the stream ticket so an exam unlock cannot open the SSE feed
// or vice versa.
type ZipClaims struct {
	UserID    string `json:"uid"`
	ContestID string `json:"cid"`
	SessionID string `json:"sid"`
	Binding   string `json:"bind"`
	ExpiresAt int64  `json:"exp"`
}

// ZipUnlockTTL is deliberately short. Candidates must re-enter the exam ZIP
// password after a long break, and staff can rotate the password to revoke
// every outstanding grant.
const ZipUnlockTTL = 8 * time.Hour

func (a *Service) ticketSignature(payload string) string {
	mac := hmac.New(sha256.New, a.ticketSecret)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// IssueStreamTicket returns a signed grant valid for ttl. The value belongs in
// an HttpOnly cookie, never in localStorage or a query string. Binding the
// grant to the session digest makes logout and password reset revoke it.
func (a *Service) IssueStreamTicket(userID, contestID, sessionToken string, ttl time.Duration) (string, time.Time) {
	if ttl <= 0 || ttl > 24*time.Hour {
		ttl = StreamTicketTTL
	}
	expires := a.clock.Now().Add(ttl)
	claims := StreamClaims{
		UserID: userID, ContestID: contestID,
		SessionID: hashSessionToken(sessionToken), ExpiresAt: expires.Unix(),
	}
	raw, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(raw)
	return payload + "." + a.ticketSignature(payload), expires
}

// ValidateStreamTicket verifies the signature, expiry, and optional contest
// binding. A mismatched contest is rejected exactly like a bad signature.
func (a *Service) ValidateStreamTicket(ticket, contestID string) (StreamClaims, error) {
	invalid := errors.New("invalid stream ticket")
	parts := strings.Split(ticket, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return StreamClaims{}, invalid
	}
	expected := a.ticketSignature(parts[0])
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return StreamClaims{}, invalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return StreamClaims{}, invalid
	}
	var claims StreamClaims
	if err := json.Unmarshal(raw, &claims); err != nil {
		return StreamClaims{}, invalid
	}
	if claims.UserID == "" || claims.SessionID == "" || claims.ExpiresAt <= a.clock.Now().Unix() {
		return StreamClaims{}, invalid
	}
	if contestID != "" && claims.ContestID != contestID {
		return StreamClaims{}, invalid
	}
	if err := a.requireLiveSession(claims.SessionID, claims.UserID); err != nil {
		return StreamClaims{}, invalid
	}
	return claims, nil
}

func (a *Service) requireLiveSession(sessionID, userID string) error {
	if sessionID == "" || userID == "" {
		return errors.New("invalid session binding")
	}
	sess, err := a.store.GetSession(sessionID)
	if err != nil || sess.UserID != userID || sess.Expired(a.clock.Now()) {
		return errors.New("invalid session binding")
	}
	return nil
}

// ticketFor signs an arbitrary claims payload with a domain prefix so grants
// cannot be replayed across purposes even though they share a process key.
func (a *Service) ticketFor(scope string, claims any) (string, error) {
	raw, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, a.ticketSecret)
	_, _ = mac.Write([]byte(scope))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (a *Service) validateTicket(scope, ticket, contestID string) (ZipClaims, error) {
	invalid := errors.New("invalid exam grant")
	parts := strings.Split(ticket, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ZipClaims{}, invalid
	}
	mac := hmac.New(sha256.New, a.ticketSecret)
	_, _ = mac.Write([]byte(scope))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write([]byte(parts[0]))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return ZipClaims{}, invalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ZipClaims{}, invalid
	}
	var claims ZipClaims
	if err := json.Unmarshal(raw, &claims); err != nil {
		return ZipClaims{}, invalid
	}
	if claims.UserID == "" || claims.ContestID == "" || claims.SessionID == "" ||
		claims.ExpiresAt <= a.clock.Now().Unix() ||
		(contestID != "" && claims.ContestID != contestID) {
		return ZipClaims{}, invalid
	}
	if err := a.requireLiveSession(claims.SessionID, claims.UserID); err != nil {
		return ZipClaims{}, invalid
	}
	return claims, nil
}

// IssueZipGrant returns a signed, user-and-contest-bound unlock grant.
func (a *Service) IssueZipGrant(
	userID, contestID, sessionToken, passwordBinding string,
	ttl time.Duration,
) (string, time.Time, error) {
	if ttl <= 0 || ttl > 24*time.Hour {
		ttl = ZipUnlockTTL
	}
	expires := a.clock.Now().Add(ttl)
	ticket, err := a.ticketFor("zip", ZipClaims{
		UserID: userID, ContestID: contestID,
		SessionID: hashSessionToken(sessionToken), Binding: passwordBinding,
		ExpiresAt: expires.Unix(),
	})
	return ticket, expires, err
}

// ValidateZipGrant verifies signature, expiry, user, and contest binding.
func (a *Service) ValidateZipGrant(ticket, userID, contestID, passwordBinding string) (ZipClaims, error) {
	claims, err := a.validateTicket("zip", ticket, contestID)
	if err != nil {
		return ZipClaims{}, err
	}
	if claims.UserID != userID || !hmac.Equal([]byte(claims.Binding), []byte(passwordBinding)) {
		return ZipClaims{}, errors.New("invalid exam grant")
	}
	return claims, nil
}

// ZipPasswordBinding turns the current verifier into an opaque revocation key.
// Rotating the password generates a new salt/verifier and invalidates every
// outstanding grant without exposing the verifier to the client.
func (a *Service) ZipPasswordBinding(verifier string) string {
	mac := hmac.New(sha256.New, a.ticketSecret)
	_, _ = mac.Write([]byte("zip-password."))
	_, _ = mac.Write([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// ChangePassword 本人改密（需旧密码，成功后踢掉其他会话）。
func (a *Service) ChangePassword(uid, oldPassword, newPassword string) error {
	if err := ValidatePassword(newPassword); err != nil {
		return err
	}
	hash, err := a.store.GetPasswordHash(uid)
	if err != nil || hash == "" {
		return errors.New("password not set, ask a teacher to reset it")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(oldPassword)); err != nil {
		return errors.New("old password incorrect")
	}
	return a.setPassword(uid, newPassword)
}

// ResetPassword 教师/管理员重置他人密码（踢掉该用户全部会话）。
func (a *Service) ResetPassword(uid, newPassword string) error {
	if err := ValidatePassword(newPassword); err != nil {
		return err
	}
	return a.setPassword(uid, newPassword)
}

// ValidatePassword applies the shared credential policy to bootstrap,
// teacher resets, CSV imports, and self-service changes.
func ValidatePassword(password string) error {
	if len([]rune(strings.TrimSpace(password))) < MinPasswordLength {
		return fmt.Errorf("password too short (min %d characters)", MinPasswordLength)
	}
	if len([]byte(password)) > MaxPasswordBytes {
		return fmt.Errorf("password too long (max %d bytes)", MaxPasswordBytes)
	}
	return nil
}

func (a *Service) setPassword(uid, newPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), Cost)
	if err != nil {
		return err
	}
	if err := a.store.SetPasswordHash(uid, string(hash)); err != nil {
		return err
	}
	return a.store.DeleteSessionsForUser(uid)
}

// EnsureAdmin 种子管理员：不存在则创建并设密（首次启动用）。
func (a *Service) EnsureAdmin(password string) (domain.User, error) {
	users, err := a.store.ListUsers()
	if err != nil {
		return domain.User{}, err
	}
	for _, u := range users {
		if u.Username == "admin" && u.Role == domain.RoleAdmin {
			return u, nil
		}
	}
	if err := ValidatePassword(password); err != nil {
		return domain.User{}, err
	}
	u, err := a.store.CreateUser(domain.User{
		Username: "admin", Name: "admin", Role: domain.RoleAdmin,
		CreatedAt: a.clock.Now(),
	})
	if err != nil {
		// Another process may have won the first-run bootstrap race. Re-read
		// once and return that account instead of turning a healthy startup
		// into a permanent failure.
		if users, listErr := a.store.ListUsers(); listErr == nil {
			for _, existing := range users {
				if existing.Username == "admin" && existing.Role == domain.RoleAdmin {
					return existing, nil
				}
			}
		}
		return domain.User{}, err
	}
	if err := a.setPassword(u.ID, password); err != nil {
		return domain.User{}, err
	}
	return u, nil
}
