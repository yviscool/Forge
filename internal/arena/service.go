package arena

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type Status string

const (
	Draft    Status = "draft"
	Running  Status = "running"
	Finished Status = "finished"
)

type User struct {
	ID, Name, Role string
	Groups         []string
}
type Group struct {
	ID, Name string
	UserIDs  []string
}
type Contest struct {
	ID, Name, Description, Status string
	ProblemIDs, GroupIDs          []string
	CreatedAt                     time.Time
}
type Problem struct {
	ID, ContestID, Code, Title, Statement, Constraints, Input, Output, Examples, Locale string
	UpdatedAt                                                                           time.Time
}
type Submission struct {
	ID, ContestID, ProblemID, UserID, Language, Code, Verdict string
	Score                                                     int
	SubmittedAt                                               time.Time
}
type RankEntry struct {
	UserID, UserName string
	Score, Accepted  int
}

type Event struct {
	Type      string    `json:"type"`
	ContestID string    `json:"contestId"`
	Data      any       `json:"data"`
	At        time.Time `json:"at"`
}

type Service struct {
	mu          sync.RWMutex
	seq         int
	users       map[string]User
	groups      map[string]Group
	contests    map[string]Contest
	problems    map[string]Problem
	submissions map[string]Submission
	events      map[chan Event]struct{}
}

func NewService() *Service {
	return &Service{users: map[string]User{}, groups: map[string]Group{}, contests: map[string]Contest{}, problems: map[string]Problem{}, submissions: map[string]Submission{}, events: map[chan Event]struct{}{}}
}
func (s *Service) id(prefix string) string { s.seq++; return fmt.Sprintf("%s-%04d", prefix, s.seq) }
func (s *Service) emit(e Event) {
	for ch := range s.events {
		select {
		case ch <- e:
		default:
		}
	}
}
func (s *Service) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 16)
	s.mu.Lock()
	s.events[ch] = struct{}{}
	s.mu.Unlock()
	return ch, func() { s.mu.Lock(); delete(s.events, ch); close(ch); s.mu.Unlock() }
}

func (s *Service) CreateUser(name, role string) (User, error) {
	if strings.TrimSpace(name) == "" {
		return User{}, errors.New("name is required")
	}
	if role != "teacher" && role != "student" {
		role = "student"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	u := User{ID: s.id("usr"), Name: strings.TrimSpace(name), Role: role}
	s.users[u.ID] = u
	return u, nil
}
func (s *Service) CreateGroup(name string) (Group, error) {
	if strings.TrimSpace(name) == "" {
		return Group{}, errors.New("name is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	g := Group{ID: s.id("grp"), Name: strings.TrimSpace(name)}
	s.groups[g.ID] = g
	return g, nil
}
func (s *Service) AddUserToGroup(uid, gid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[uid]
	if !ok {
		return errors.New("user not found")
	}
	g, ok := s.groups[gid]
	if !ok {
		return errors.New("group not found")
	}
	for _, x := range g.UserIDs {
		if x == uid {
			return nil
		}
	}
	g.UserIDs = append(g.UserIDs, uid)
	u.Groups = append(u.Groups, gid)
	s.groups[gid] = g
	s.users[uid] = u
	return nil
}
func (s *Service) CreateContest(name, desc string) (Contest, error) {
	if strings.TrimSpace(name) == "" {
		return Contest{}, errors.New("name is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c := Contest{ID: s.id("cnt"), Name: strings.TrimSpace(name), Description: desc, Status: string(Draft), CreatedAt: time.Now().UTC()}
	s.contests[c.ID] = c
	return c, nil
}
func (s *Service) StartContest(id string) (Contest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.contests[id]
	if !ok {
		return Contest{}, errors.New("contest not found")
	}
	if c.Status == string(Finished) {
		return Contest{}, errors.New("contest already finished")
	}
	c.Status = string(Running)
	s.contests[id] = c
	s.emit(Event{Type: "contest.started", ContestID: id, Data: c, At: time.Now().UTC()})
	return c, nil
}
func (s *Service) AddProblem(p Problem) (Problem, error) {
	if p.ContestID == "" || p.Title == "" || p.Statement == "" {
		return Problem{}, errors.New("contestId, title and statement are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.contests[p.ContestID]
	if !ok {
		return Problem{}, errors.New("contest not found")
	}
	p.ID = s.id("prb")
	p.UpdatedAt = time.Now().UTC()
	if p.Locale == "" {
		p.Locale = "zh-CN"
	}
	s.problems[p.ID] = p
	c.ProblemIDs = append(c.ProblemIDs, p.ID)
	s.contests[c.ID] = c
	return p, nil
}
func (s *Service) Submit(x Submission) (Submission, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.contests[x.ContestID]
	if !ok {
		return Submission{}, errors.New("contest not found")
	}
	if c.Status != string(Running) {
		return Submission{}, errors.New("contest is not running")
	}
	if _, ok := s.users[x.UserID]; !ok {
		return Submission{}, errors.New("user not found")
	}
	if _, ok := s.problems[x.ProblemID]; !ok {
		return Submission{}, errors.New("problem not found")
	}
	x.ID = s.id("sub")
	x.Verdict = "queued"
	x.SubmittedAt = time.Now().UTC()
	s.submissions[x.ID] = x
	e := Event{Type: "submission.created", ContestID: x.ContestID, Data: x, At: time.Now().UTC()}
	s.emit(e)
	return x, nil
}
func (s *Service) Judge(id, verdict string, score int) (Submission, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, ok := s.submissions[id]
	if !ok {
		return Submission{}, errors.New("submission not found")
	}
	x.Verdict = verdict
	x.Score = score
	s.submissions[id] = x
	s.emit(Event{Type: "submission.judged", ContestID: x.ContestID, Data: x, At: time.Now().UTC()})
	s.emit(Event{Type: "ranking.updated", ContestID: x.ContestID, Data: s.rankingLocked(x.ContestID), At: time.Now().UTC()})
	return x, nil
}
func (s *Service) rankingLocked(cid string) []RankEntry {
	scores := map[string]*RankEntry{}
	for _, u := range s.users {
		scores[u.ID] = &RankEntry{UserID: u.ID, UserName: u.Name}
	}
	for _, x := range s.submissions {
		if x.ContestID == cid && x.Verdict == "accepted" {
			e := scores[x.UserID]
			if e != nil {
				e.Score += x.Score
				e.Accepted++
			}
		}
	}
	out := []RankEntry{}
	for _, e := range scores {
		if e.Score > 0 {
			out = append(out, *e)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].UserName < out[j].UserName
		}
		return out[i].Score > out[j].Score
	})
	return out
}
func (s *Service) Ranking(cid string) []RankEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.rankingLocked(cid)
}
func (s *Service) Contests() []Contest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Contest, 0, len(s.contests))
	for _, c := range s.contests {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}
func (s *Service) Problems(cid string) []Problem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Problem{}
	for _, p := range s.problems {
		if p.ContestID == cid {
			out = append(out, p)
		}
	}
	return out
}
