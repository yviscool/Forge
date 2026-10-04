// Package memory 提供 ports.Store 的内存实现：进程内 Map + 序号 ID。
// Phase1 的 sqlite 将实现同一接口，app 层无感切换。
package memory

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/yviscool/forge/internal/domain"
)

type Store struct {
	mu          sync.RWMutex
	seq         int
	users       map[string]domain.User
	groups      map[string]domain.Group
	contests    map[string]domain.Contest
	problems    map[string]domain.Problem
	submissions map[string]domain.Submission
}

func New() *Store {
	return &Store{
		users:       map[string]domain.User{},
		groups:      map[string]domain.Group{},
		contests:    map[string]domain.Contest{},
		problems:    map[string]domain.Problem{},
		submissions: map[string]domain.Submission{},
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
	s.users[u.ID] = u
	return u, nil
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

func (s *Store) ListUsers() []domain.User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.User, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (s *Store) UpdateUser(u domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[u.ID]; !ok {
		return errors.New("user not found")
	}
	s.users[u.ID] = u
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

func (s *Store) ListGroups() []domain.Group {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Group, 0, len(s.groups))
	for _, g := range s.groups {
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
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
	s.contests[c.ID] = c
	return c, nil
}

func (s *Store) GetContest(id string) (domain.Contest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.contests[id]
	if !ok {
		return domain.Contest{}, errors.New("contest not found")
	}
	return c, nil
}

func (s *Store) ListContests() []domain.Contest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Contest, 0, len(s.contests))
	for _, c := range s.contests {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func (s *Store) UpdateContest(c domain.Contest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.contests[c.ID]; !ok {
		return errors.New("contest not found")
	}
	s.contests[c.ID] = c
	return nil
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

func (s *Store) ListProblems(contestID string) []domain.Problem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []domain.Problem{}
	for _, p := range s.problems {
		if p.ContestID == contestID {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
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

func (s *Store) CreateSubmission(x domain.Submission) (domain.Submission, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if x.ID == "" {
		x.ID = s.nextLocked("sub")
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

func (s *Store) ListSubmissions(contestID string) []domain.Submission {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []domain.Submission{}
	for _, x := range s.submissions {
		if x.ContestID == contestID {
			out = append(out, x)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SubmittedAt.After(out[j].SubmittedAt) })
	return out
}

func (s *Store) UpdateSubmission(x domain.Submission) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.submissions[x.ID]; !ok {
		return errors.New("submission not found")
	}
	s.submissions[x.ID] = x
	return nil
}
