// Package auth 身份与会话：bcrypt 凭证 + token 会话 + 种子 admin。
// 密码 hash 永不进 domain.User，只走 Store 凭证端口。
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
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

type Service struct {
	store ports.Store
	clock ports.Clock
}

func New(store ports.Store, clock ports.Clock) *Service {
	if clock == nil {
		clock = ports.SystemClock{}
	}
	return &Service{store: store, clock: clock}
}

func newToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func (a *Service) findByLogin(login string) (domain.User, error) {
	login = strings.TrimSpace(login)
	for _, u := range a.store.ListUsers() {
		if u.Username == login || u.Name == login {
			return u, nil
		}
	}
	return domain.User{}, errors.New("invalid login or password")
}

// Login 校验凭证并签发会话。
func (a *Service) Login(login, password string) (domain.Session, error) {
	u, err := a.findByLogin(login)
	if err != nil {
		return domain.Session{}, err
	}
	hash, err := a.store.GetPasswordHash(u.ID)
	if err != nil || hash == "" {
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
	if err := a.store.SaveSession(sess); err != nil {
		return domain.Session{}, err
	}
	return sess, nil
}

// Authenticate 校验 token 有效性（含过期惰性清理）。
func (a *Service) Authenticate(token string) (domain.Session, error) {
	sess, err := a.store.GetSession(strings.TrimSpace(token))
	if err != nil {
		return domain.Session{}, errors.New("unauthorized")
	}
	if sess.Expired(a.clock.Now()) {
		_ = a.store.DeleteSession(sess.Token)
		return domain.Session{}, errors.New("session expired")
	}
	return sess, nil
}

// Logout 注销单会话（幂等）。
func (a *Service) Logout(token string) error {
	return a.store.DeleteSession(strings.TrimSpace(token))
}

// ChangePassword 本人改密（需旧密码，成功后踢掉其他会话）。
func (a *Service) ChangePassword(uid, oldPassword, newPassword string) error {
	if len(newPassword) < 4 {
		return errors.New("password too short (min 4)")
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
	if len(newPassword) < 4 {
		return errors.New("password too short (min 4)")
	}
	return a.setPassword(uid, newPassword)
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
	for _, u := range a.store.ListUsers() {
		if u.Username == "admin" && u.Role == domain.RoleAdmin {
			return u, nil
		}
	}
	u, err := a.store.CreateUser(domain.User{
		Username: "admin", Name: "admin", Role: domain.RoleAdmin,
		CreatedAt: a.clock.Now(),
	})
	if err != nil {
		return domain.User{}, err
	}
	if err := a.setPassword(u.ID, password); err != nil {
		return domain.User{}, err
	}
	return u, nil
}
