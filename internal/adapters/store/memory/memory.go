// Package memory 提供 ports.Store 的内存实现：进程内 Map + 序号 ID。
// Phase1 的 sqlite 将实现同一接口，app 层无感切换。
package memory

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/ports"
)

type Store struct {
	mu          sync.RWMutex
	seq         int
	users       map[string]domain.User
	groups      map[string]domain.Group
	contests    map[string]domain.Contest
	problems    map[string]domain.Problem
	submissions map[string]domain.Submission
	subByReqID  map[string]string
	passwords   map[string]string
	sessions    map[string]domain.Session
	// provinceStates 省份启用状态覆盖（编号 → 是否启用）；nil 视为全部启用。
	provinceStates map[string]bool
	// codeVersions 代码版本仓，按 "cid|pid|uid" 分桶，追加序即 Seq。
	codeVersions map[string][]domain.CodeVersion
}

// Ping always succeeds: an in-memory store has no external connection to lose.
func (s *Store) Ping(context.Context) error { return nil }

func New() *Store {
	return &Store{
		users:          map[string]domain.User{},
		groups:         map[string]domain.Group{},
		contests:       map[string]domain.Contest{},
		problems:       map[string]domain.Problem{},
		submissions:    map[string]domain.Submission{},
		subByReqID:     map[string]string{},
		passwords:      map[string]string{},
		sessions:       map[string]domain.Session{},
		provinceStates: map[string]bool{},
		codeVersions:   map[string][]domain.CodeVersion{},
	}
}

func (s *Store) NextID(prefix string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	return fmt.Sprintf("%s-%04d", prefix, s.seq)
}

func (s *Store) nextLocked(prefix string) string {
	s.seq++
	return fmt.Sprintf("%s-%04d", prefix, s.seq)
}

func (s *Store) CreateUser(u domain.User) (domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if u.ID == "" {
		u.ID = s.nextLocked("usr")
	}
	if s.usernameTakenLocked(u.Username, u.ID) {
		return domain.User{}, errors.New("username already exists")
	}
	s.users[u.ID] = u
	return u, nil
}

func (s *Store) usernameTakenLocked(username, excludeID string) bool {
	username = strings.TrimSpace(username)
	if username == "" {
		return false
	}
	for id, existing := range s.users {
		if id != excludeID && strings.EqualFold(existing.Username, username) {
			return true
		}
	}
	return false
}

func (s *Store) GetUser(id string) (domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return domain.User{}, errors.New("user not found")
	}
	return u, nil
}

func (s *Store) WithTx(ctx context.Context, fn func(tx ports.Store) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 拍摄快照到独立工作副本，避免同一个 Store 递归锁死
	tx := &Store{
		seq:            s.seq,
		users:          cloneMap(s.users),
		groups:         cloneMap(s.groups),
		contests:       cloneMap(s.contests),
		problems:       cloneMap(s.problems),
		submissions:    cloneMap(s.submissions),
		subByReqID:     cloneMap(s.subByReqID),
		passwords:      cloneMap(s.passwords),
		sessions:       cloneMap(s.sessions),
		provinceStates: cloneMap(s.provinceStates),
		codeVersions:   cloneCodeVersions(s.codeVersions),
	}

	if err := fn(tx); err != nil {
		return err
	}

	// 提交：将工作副本数据合并回主存储
	s.seq = tx.seq
	s.users = tx.users
	s.groups = tx.groups
	s.contests = tx.contests
	s.problems = tx.problems
	s.submissions = tx.submissions
	s.subByReqID = tx.subByReqID
	s.passwords = tx.passwords
	s.sessions = tx.sessions
	s.provinceStates = tx.provinceStates
	s.codeVersions = tx.codeVersions
	return nil
}

func cloneMap[K comparable, V any](m map[K]V) map[K]V {
	out := make(map[K]V, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func cloneCodeVersions(m map[string][]domain.CodeVersion) map[string][]domain.CodeVersion {
	out := make(map[string][]domain.CodeVersion, len(m))
	for k, v := range m {
		out[k] = append([]domain.CodeVersion(nil), v...)
	}
	return out
}

func codeVersionBucket(contestID, problemID, userID string) string {
	return contestID + "|" + problemID + "|" + userID
}

// CreateCodeVersion 追加版本：Seq 按桶内条数递增，超出上限裁剪最旧版本。
func (s *Store) CreateCodeVersion(v domain.CodeVersion) (domain.CodeVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v.ID == "" {
		v.ID = s.nextLocked("ver")
	}
	key := codeVersionBucket(v.ContestID, v.ProblemID, v.UserID)
	bucket := s.codeVersions[key]
	// 裁剪后桶长会小于最大 seq；seq 必须取桶内最大值 +1，避免与被保留的旧版本撞号。
	maxSeq := 0
	for _, x := range bucket {
		if x.Seq > maxSeq {
			maxSeq = x.Seq
		}
	}
	v.Seq = maxSeq + 1
	bucket = append(bucket, v)
	if len(bucket) > domain.MaxCodeVersionsPerProblem {
		bucket = bucket[len(bucket)-domain.MaxCodeVersionsPerProblem:]
	}
	s.codeVersions[key] = bucket
	return v, nil
}

// ListCodeVersions 按 Seq 降序（最新在前）返回至多 limit 个版本。
func (s *Store) ListCodeVersions(contestID, problemID, userID string, limit int) ([]domain.CodeVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	bucket := s.codeVersions[codeVersionBucket(contestID, problemID, userID)]
	out := make([]domain.CodeVersion, 0, len(bucket))
	for i := len(bucket) - 1; i >= 0 && (limit <= 0 || len(out) < limit); i-- {
		out = append(out, bucket[i])
	}
	return out, nil
}

func (s *Store) ListUsers() ([]domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.User, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *Store) UpdateUser(u domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[u.ID]; !ok {
		return errors.New("user not found")
	}
	if s.usernameTakenLocked(u.Username, u.ID) {
		return errors.New("username already exists")
	}
	s.users[u.ID] = u
	return nil
}

// ProvinceStates 返回省份启用状态覆盖表；调用方拿到的是副本。
func (s *Store) ProvinceStates() (map[string]bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]bool, len(s.provinceStates))
	for code, enabled := range s.provinceStates {
		out[code] = enabled
	}
	return out, nil
}

// SetProvinceState 记录单个省份的启用状态；非内置省份直接拒绝。
func (s *Store) SetProvinceState(code string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	canonical, ok := domain.ProvinceByCode(code)
	if !ok {
		return fmt.Errorf("unknown province %q", code)
	}
	s.provinceStates[canonical.Code] = enabled
	return nil
}

func (s *Store) CreateGroup(g domain.Group) (domain.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if g.ID == "" {
		g.ID = s.nextLocked("grp")
	}
	s.groups[g.ID] = g
	return g, nil
}

func (s *Store) GetGroup(id string) (domain.Group, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.groups[id]
	if !ok {
		return domain.Group{}, errors.New("group not found")
	}
	return g, nil
}

func (s *Store) ListGroups() ([]domain.Group, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Group, 0, len(s.groups))
	for _, g := range s.groups {
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *Store) UpdateGroup(g domain.Group) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.groups[g.ID]; !ok {
		return errors.New("group not found")
	}
	s.groups[g.ID] = g
	return nil
}

func (s *Store) CreateContest(c domain.Contest) (domain.Contest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.ID == "" {
		c.ID = s.nextLocked("cnt")
	}
	stored := cloneContest(c)
	s.contests[c.ID] = stored
	return cloneContest(stored), nil
}

func (s *Store) GetContest(id string) (domain.Contest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.contests[id]
	if !ok {
		return domain.Contest{}, errors.New("contest not found")
	}
	return cloneContest(c), nil
}

func (s *Store) ListContests() ([]domain.Contest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Contest, 0, len(s.contests))
	for _, c := range s.contests {
		out = append(out, cloneContest(c))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (s *Store) UpdateContest(c domain.Contest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.contests[c.ID]; !ok {
		return errors.New("contest not found")
	}
	s.contests[c.ID] = cloneContest(c)
	return nil
}

func cloneContest(c domain.Contest) domain.Contest {
	c.AllowedLanguages = append([]string(nil), c.AllowedLanguages...)
	c.ProblemIDs = append([]string(nil), c.ProblemIDs...)
	c.GroupIDs = append([]string(nil), c.GroupIDs...)
	c.ParticipantUserIDs = append([]string(nil), c.ParticipantUserIDs...)
	c.Broadcasts = append([]domain.BroadcastMessage(nil), c.Broadcasts...)
	if c.BoundIPs != nil {
		c.BoundIPs = cloneMap(c.BoundIPs)
	}
	if c.Extensions != nil {
		c.Extensions = cloneMap(c.Extensions)
	}
	if c.FinalRanking != nil {
		ranking := make([]domain.RankEntry, len(c.FinalRanking))
		for i, entry := range c.FinalRanking {
			entry.ProblemScores = cloneMap(entry.ProblemScores)
			ranking[i] = entry
		}
		c.FinalRanking = ranking
	}
	c.FinalACM = append([]domain.ACMRankEntry(nil), c.FinalACM...)
	return c
}

func (s *Store) CreateProblem(p domain.Problem) (domain.Problem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.ID == "" {
		p.ID = s.nextLocked("prb")
	}
	s.problems[p.ID] = p
	if c, ok := s.contests[p.ContestID]; ok {
		c.ProblemIDs = append(c.ProblemIDs, p.ID)
		s.contests[c.ID] = c
	}
	return p, nil
}

func (s *Store) GetProblem(id string) (domain.Problem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.problems[id]
	if !ok {
		return domain.Problem{}, errors.New("problem not found")
	}
	return p, nil
}

func (s *Store) ListProblems(contestID string) ([]domain.Problem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []domain.Problem{}
	for _, p := range s.problems {
		if p.ContestID == contestID {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out, nil
}

func (s *Store) UpdateProblem(p domain.Problem) (domain.Problem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.problems[p.ID]; !ok {
		return domain.Problem{}, errors.New("problem not found")
	}
	s.problems[p.ID] = p
	return p, nil
}

func (s *Store) DeleteProblem(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.problems[id]; !ok {
		return errors.New("problem not found")
	}
	delete(s.problems, id)
	return nil
}

func (s *Store) CreateSubmission(x domain.Submission) (domain.Submission, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if x.ID == "" {
		x.ID = s.nextLocked("sub")
	}
	if x.RequestID != "" {
		if _, exists := s.subByReqID[x.RequestID]; exists {
			return domain.Submission{}, errors.New("request id already exists")
		}
		s.subByReqID[x.RequestID] = x.ID
	}
	s.submissions[x.ID] = x
	return x, nil
}

func (s *Store) GetSubmission(id string) (domain.Submission, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	x, ok := s.submissions[id]
	if !ok {
		return domain.Submission{}, errors.New("submission not found")
	}
	return x, nil
}

func (s *Store) FindSubmissionByRequestID(requestID string) (*domain.Submission, error) {
	if requestID == "" {
		return nil, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.subByReqID[requestID]
	if !ok {
		return nil, nil
	}
	x, ok := s.submissions[id]
	if !ok {
		return nil, nil
	}
	return &x, nil
}

func (s *Store) ListSubmissions(contestID string) ([]domain.Submission, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []domain.Submission{}
	for _, x := range s.submissions {
		if x.ContestID == contestID {
			out = append(out, x)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SubmittedAt.After(out[j].SubmittedAt) })
	return out, nil
}

func (s *Store) ListSubmissionFeed(contestID, userID string, limit int) ([]domain.Submission, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []domain.Submission{}
	for _, x := range s.submissions {
		if x.ContestID != contestID || (userID != "" && x.UserID != userID) {
			continue
		}
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SubmittedAt.Equal(out[j].SubmittedAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].SubmittedAt.After(out[j].SubmittedAt)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *Store) ListQueuedSubmissions() ([]domain.Submission, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []domain.Submission{}
	for _, x := range s.submissions {
		if x.Verdict == domain.VerdictQueued {
			out = append(out, x)
		}
	}
	// Recovery is FIFO so an old submission cannot be starved by a busy queue.
	sort.Slice(out, func(i, j int) bool {
		if out[i].SubmittedAt.Equal(out[j].SubmittedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].SubmittedAt.Before(out[j].SubmittedAt)
	})
	return out, nil
}

func (s *Store) UpdateSubmission(x domain.Submission) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.submissions[x.ID]
	if !ok {
		return errors.New("submission not found")
	}
	if x.RequestID == "" {
		x.RequestID = existing.RequestID
	}
	if existing.RequestID != x.RequestID {
		if existing.RequestID != "" {
			delete(s.subByReqID, existing.RequestID)
		}
		if x.RequestID != "" {
			if owner, exists := s.subByReqID[x.RequestID]; exists && owner != x.ID {
				return errors.New("request id already exists")
			}
			s.subByReqID[x.RequestID] = x.ID
		}
	}
	s.submissions[x.ID] = x
	return nil
}

func (s *Store) SetPasswordHash(userID, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[userID]; !ok {
		return errors.New("user not found")
	}
	s.passwords[userID] = hash
	return nil
}

func (s *Store) GetPasswordHash(userID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.users[userID]; !ok {
		return "", errors.New("user not found")
	}
	return s.passwords[userID], nil
}

func (s *Store) SaveSession(sess domain.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sess.Token] = sess
	return nil
}

func (s *Store) GetSession(token string) (domain.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[token]
	if !ok {
		return domain.Session{}, errors.New("session not found")
	}
	return sess, nil
}

func (s *Store) DeleteSession(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
	return nil
}

func (s *Store) DeleteSessionsForUser(userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for tok, sess := range s.sessions {
		if sess.UserID == userID {
			delete(s.sessions, tok)
		}
	}
	return nil
}

func (s *Store) DeleteExpiredSessions(now time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	deleted := 0
	for token, sess := range s.sessions {
		if sess.Expired(now) {
			delete(s.sessions, token)
			deleted++
		}
	}
	return deleted, nil
}
