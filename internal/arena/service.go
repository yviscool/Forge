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
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Name      string    `json:"name"`
	Role      string    `json:"role"` // admin, teacher, student
	Groups    []string  `json:"groups"`
	CreatedAt time.Time `json:"createdAt"`
}

type Group struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	UserIDs     []string  `json:"userIds"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Contest struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Description        string    `json:"description"`
	Status             string    `json:"status"` // draft, running, finished
	ProblemIDs         []string  `json:"problemIds"`
	GroupIDs           []string  `json:"groupIds"`
	ParticipantUserIDs []string  `json:"participantUserIds"`
	CreatedAt          time.Time `json:"createdAt"`
	StartedAt          time.Time `json:"startedAt,omitempty"`
	FinishedAt         time.Time `json:"finishedAt,omitempty"`
}

type ProblemLocale struct {
	Title       string `json:"title"`
	Statement   string `json:"statement"`
	Constraints string `json:"constraints"`
	Input       string `json:"input"`
	Output      string `json:"output"`
	Examples    string `json:"examples"`
}

type TestCase struct {
	ID         string `json:"id"`
	InputFile  string `json:"inputFile"`
	OutputFile string `json:"outputFile"`
	Score      int    `json:"score"`
	Subtask    int    `json:"subtask"`
}

type Problem struct {
	ID             string                   `json:"id"`
	ContestID      string                   `json:"contestId"`
	Code           string                   `json:"code"` // e.g. "reverse", "merge", "A"
	Title          string                   `json:"title"`
	Statement      string                   `json:"statement"`
	Constraints    string                   `json:"constraints"`
	Input          string                   `json:"input"`
	Output         string                   `json:"output"`
	Examples       string                   `json:"examples"`
	Locale         string                   `json:"locale"` // default "zh-CN"
	TimeLimitMs    int                      `json:"timeLimitMs"`
	MemoryLimitMiB int                      `json:"memoryLimitMib"`
	Locales        map[string]ProblemLocale `json:"locales,omitempty"`
	TestCases      []TestCase               `json:"testCases,omitempty"`
	UpdatedAt      time.Time                `json:"updatedAt"`
}

type Submission struct {
	ID          string    `json:"id"`
	ContestID   string    `json:"contestId"`
	ProblemID   string    `json:"problemID"`
	UserID      string    `json:"userId"`
	UserName    string    `json:"userName,omitempty"`
	Language    string    `json:"language"`
	Code        string    `json:"code"`
	Verdict     string    `json:"verdict"` // queued, judging, accepted, wrong_answer, time_limit, runtime_error, compile_error
	Score       int       `json:"score"`
	SubmittedAt time.Time `json:"submittedAt"`
	JudgedAt    time.Time `json:"judgedAt,omitempty"`
}

type RankEntry struct {
	UserID        string         `json:"userId"`
	UserName      string         `json:"userName"`
	Score         int            `json:"score"`
	Accepted      int            `json:"accepted"`
	ProblemScores map[string]int `json:"problemScores"`
}

type Event struct {
	Type      string    `json:"type"`
	ContestID string    `json:"contestId,omitempty"`
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
	return &Service{
		users:       map[string]User{},
		groups:      map[string]Group{},
		contests:    map[string]Contest{},
		problems:    map[string]Problem{},
		submissions: map[string]Submission{},
		events:      map[chan Event]struct{}{},
	}
}

func (s *Service) id(prefix string) string {
	s.seq++
	return fmt.Sprintf("%s-%04d", prefix, s.seq)
}

func (s *Service) emit(e Event) {
	for ch := range s.events {
		select {
		case ch <- e:
		default:
		}
	}
}

func (s *Service) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 32)
	s.mu.Lock()
	s.events[ch] = struct{}{}
	s.mu.Unlock()
	return ch, func() {
		s.mu.Lock()
		delete(s.events, ch)
		close(ch)
		s.mu.Unlock()
	}
}

// ================= User & Group Management =================

func (s *Service) CreateUser(name, role string) (User, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return User{}, errors.New("name is required")
	}
	if role != "admin" && role != "teacher" && role != "student" {
		role = "student"
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	username := strings.ToLower(strings.ReplaceAll(name, " ", "_"))
	u := User{
		ID:        s.id("usr"),
		Username:  username,
		Name:      name,
		Role:      role,
		Groups:    []string{},
		CreatedAt: time.Now().UTC(),
	}
	s.users[u.ID] = u
	return u, nil
}

func (s *Service) GetUser(id string) (User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return User{}, errors.New("user not found")
	}
	return u, nil
}

func (s *Service) ListUsers() []User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]User, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (s *Service) CreateGroup(name string) (Group, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Group{}, errors.New("name is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	g := Group{
		ID:        s.id("grp"),
		Name:      name,
		UserIDs:   []string{},
		CreatedAt: time.Now().UTC(),
	}
	s.groups[g.ID] = g
	return g, nil
}

func (s *Service) GetGroup(id string) (Group, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.groups[id]
	if !ok {
		return Group{}, errors.New("group not found")
	}
	return g, nil
}

func (s *Service) ListGroups() []Group {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Group, 0, len(s.groups))
	for _, g := range s.groups {
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
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

func (s *Service) RemoveUserFromGroup(uid, gid string) error {
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

	newUserIDs := []string{}
	for _, x := range g.UserIDs {
		if x != uid {
			newUserIDs = append(newUserIDs, x)
		}
	}
	g.UserIDs = newUserIDs

	newGroups := []string{}
	for _, x := range u.Groups {
		if x != gid {
			newGroups = append(newGroups, x)
		}
	}
	u.Groups = newGroups

	s.groups[gid] = g
	s.users[uid] = u
	return nil
}

// ================= Contest Management =================

func (s *Service) CreateContest(name, desc string) (Contest, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Contest{}, errors.New("name is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c := Contest{
		ID:                 s.id("cnt"),
		Name:               name,
		Description:        strings.TrimSpace(desc),
		Status:             string(Draft),
		ProblemIDs:         []string{},
		GroupIDs:           []string{},
		ParticipantUserIDs: []string{},
		CreatedAt:          time.Now().UTC(),
	}
	s.contests[c.ID] = c
	s.emit(Event{Type: "contest.created", ContestID: c.ID, Data: c, At: time.Now().UTC()})
	return c, nil
}

func (s *Service) GetContest(id string) (Contest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.contests[id]
	if !ok {
		return Contest{}, errors.New("contest not found")
	}
	return c, nil
}

func (s *Service) ListContests() []Contest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Contest, 0, len(s.contests))
	for _, c := range s.contests {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func (s *Service) Contests() []Contest {
	return s.ListContests()
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
	c.StartedAt = time.Now().UTC()
	s.contests[id] = c
	s.emit(Event{Type: "contest.started", ContestID: id, Data: c, At: time.Now().UTC()})
	return c, nil
}

func (s *Service) FinishContest(id string) (Contest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.contests[id]
	if !ok {
		return Contest{}, errors.New("contest not found")
	}
	c.Status = string(Finished)
	c.FinishedAt = time.Now().UTC()
	s.contests[id] = c
	s.emit(Event{Type: "contest.finished", ContestID: id, Data: c, At: time.Now().UTC()})
	return c, nil
}

func (s *Service) AddGroupToContest(cid, gid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.contests[cid]
	if !ok {
		return errors.New("contest not found")
	}
	if _, ok := s.groups[gid]; !ok {
		return errors.New("group not found")
	}
	for _, x := range c.GroupIDs {
		if x == gid {
			return nil
		}
	}
	c.GroupIDs = append(c.GroupIDs, gid)
	s.contests[cid] = c
	s.emit(Event{Type: "contest.updated", ContestID: cid, Data: c, At: time.Now().UTC()})
	return nil
}

func (s *Service) RemoveGroupFromContest(cid, gid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.contests[cid]
	if !ok {
		return errors.New("contest not found")
	}
	newGroupIDs := []string{}
	for _, x := range c.GroupIDs {
		if x != gid {
			newGroupIDs = append(newGroupIDs, x)
		}
	}
	c.GroupIDs = newGroupIDs
	s.contests[cid] = c
	s.emit(Event{Type: "contest.updated", ContestID: cid, Data: c, At: time.Now().UTC()})
	return nil
}

func (s *Service) AddParticipantToContest(cid, uid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.contests[cid]
	if !ok {
		return errors.New("contest not found")
	}
	if _, ok := s.users[uid]; !ok {
		return errors.New("user not found")
	}
	for _, x := range c.ParticipantUserIDs {
		if x == uid {
			return nil
		}
	}
	c.ParticipantUserIDs = append(c.ParticipantUserIDs, uid)
	s.contests[cid] = c
	s.emit(Event{Type: "contest.updated", ContestID: cid, Data: c, At: time.Now().UTC()})
	return nil
}

func (s *Service) RemoveParticipantFromContest(cid, uid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.contests[cid]
	if !ok {
		return errors.New("contest not found")
	}
	newIDs := []string{}
	for _, x := range c.ParticipantUserIDs {
		if x != uid {
			newIDs = append(newIDs, x)
		}
	}
	c.ParticipantUserIDs = newIDs
	s.contests[cid] = c
	s.emit(Event{Type: "contest.updated", ContestID: cid, Data: c, At: time.Now().UTC()})
	return nil
}

func (s *Service) IsUserAllowedInContest(cid, uid string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.contests[cid]
	if !ok {
		return false
	}
	// If no groups and no individual participants are configured, contest is open to all
	if len(c.GroupIDs) == 0 && len(c.ParticipantUserIDs) == 0 {
		return true
	}
	// Check individual participant list
	for _, id := range c.ParticipantUserIDs {
		if id == uid {
			return true
		}
	}
	// Check user group membership
	u, ok := s.users[uid]
	if !ok {
		return false
	}
	for _, gid := range c.GroupIDs {
		for _, ugid := range u.Groups {
			if gid == ugid {
				return true
			}
		}
	}
	return false
}

// ================= Problem Management & Validation =================

func (s *Service) ValidateProblem(p Problem) error {
	if strings.TrimSpace(p.ContestID) == "" {
		return errors.New("contestId is required")
	}
	if strings.TrimSpace(p.Code) == "" {
		return errors.New("problem code/identifier is required")
	}
	if strings.TrimSpace(p.Title) == "" {
		return errors.New("title is required")
	}
	if strings.TrimSpace(p.Statement) == "" {
		return errors.New("statement is required")
	}
	if strings.TrimSpace(p.Input) == "" {
		return errors.New("input format is required")
	}
	if strings.TrimSpace(p.Output) == "" {
		return errors.New("output format is required")
	}
	if strings.TrimSpace(p.Constraints) == "" {
		return errors.New("constraints specification is required")
	}
	return nil
}

func (s *Service) AddProblem(p Problem) (Problem, error) {
	if strings.TrimSpace(p.ContestID) == "" || strings.TrimSpace(p.Title) == "" || strings.TrimSpace(p.Statement) == "" {
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
	if p.TimeLimitMs <= 0 {
		p.TimeLimitMs = 1000
	}
	if p.MemoryLimitMiB <= 0 {
		p.MemoryLimitMiB = 512
	}
	if p.Code == "" {
		p.Code = fmt.Sprintf("%c", 'A'+len(c.ProblemIDs))
	}
	s.problems[p.ID] = p
	c.ProblemIDs = append(c.ProblemIDs, p.ID)
	s.contests[c.ID] = c
	s.emit(Event{Type: "problem.created", ContestID: p.ContestID, Data: p, At: time.Now().UTC()})
	return p, nil
}

func (s *Service) UpdateProblem(p Problem) (Problem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.problems[p.ID]
	if !ok {
		return Problem{}, errors.New("problem not found")
	}
	if p.Title != "" {
		existing.Title = p.Title
	}
	if p.Code != "" {
		existing.Code = p.Code
	}
	if p.Statement != "" {
		existing.Statement = p.Statement
	}
	if p.Input != "" {
		existing.Input = p.Input
	}
	if p.Output != "" {
		existing.Output = p.Output
	}
	if p.Constraints != "" {
		existing.Constraints = p.Constraints
	}
	if p.Examples != "" {
		existing.Examples = p.Examples
	}
	if p.TimeLimitMs > 0 {
		existing.TimeLimitMs = p.TimeLimitMs
	}
	if p.MemoryLimitMiB > 0 {
		existing.MemoryLimitMiB = p.MemoryLimitMiB
	}
	if p.Locales != nil {
		existing.Locales = p.Locales
	}
	if p.TestCases != nil {
		existing.TestCases = p.TestCases
	}
	existing.UpdatedAt = time.Now().UTC()
	s.problems[p.ID] = existing
	s.emit(Event{Type: "problem.updated", ContestID: existing.ContestID, Data: existing, At: time.Now().UTC()})
	return existing, nil
}

func (s *Service) GetProblem(cid, pid string) (Problem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.problems[pid]
	if !ok || p.ContestID != cid {
		return Problem{}, errors.New("problem not found")
	}
	return p, nil
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
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}

// ================= Submissions & Judging =================

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
	u, ok := s.users[x.UserID]
	if !ok {
		return Submission{}, errors.New("user not found")
	}
	if _, ok := s.problems[x.ProblemID]; !ok {
		return Submission{}, errors.New("problem not found")
	}

	// Verify participant eligibility
	allowed := false
	if len(c.GroupIDs) == 0 && len(c.ParticipantUserIDs) == 0 {
		allowed = true
	} else {
		for _, uid := range c.ParticipantUserIDs {
			if uid == x.UserID {
				allowed = true
				break
			}
		}
		if !allowed {
			for _, gid := range c.GroupIDs {
				for _, ugid := range u.Groups {
					if gid == ugid {
						allowed = true
						break
					}
				}
				if allowed {
					break
				}
			}
		}
	}
	if !allowed {
		return Submission{}, errors.New("user is not registered for this contest")
	}

	x.ID = s.id("sub")
	x.UserName = u.Name
	x.Verdict = "queued"
	x.SubmittedAt = time.Now().UTC()
	s.submissions[x.ID] = x
	s.emit(Event{Type: "submission.created", ContestID: x.ContestID, Data: x, At: time.Now().UTC()})
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
	x.JudgedAt = time.Now().UTC()
	s.submissions[id] = x
	s.emit(Event{Type: "submission.judged", ContestID: x.ContestID, Data: x, At: time.Now().UTC()})
	s.emit(Event{Type: "ranking.updated", ContestID: x.ContestID, Data: s.rankingLocked(x.ContestID), At: time.Now().UTC()})
	return x, nil
}

func (s *Service) Submissions(cid string) []Submission {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Submission{}
	for _, x := range s.submissions {
		if x.ContestID == cid {
			out = append(out, x)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SubmittedAt.After(out[j].SubmittedAt) })
	return out
}

func (s *Service) rankingLocked(cid string) []RankEntry {
	// Initialize ranking entries for authorized participants
	scores := map[string]*RankEntry{}
	c, ok := s.contests[cid]
	if !ok {
		return []RankEntry{}
	}

	for _, u := range s.users {
		// Include if user is participant or submitted code
		isPart := false
		if len(c.GroupIDs) == 0 && len(c.ParticipantUserIDs) == 0 {
			isPart = true
		} else {
			for _, uid := range c.ParticipantUserIDs {
				if uid == u.ID {
					isPart = true
					break
				}
			}
			if !isPart {
				for _, gid := range c.GroupIDs {
					for _, ugid := range u.Groups {
						if gid == ugid {
							isPart = true
							break
						}
					}
					if isPart {
						break
					}
				}
			}
		}
		if isPart {
			scores[u.ID] = &RankEntry{
				UserID:        u.ID,
				UserName:      u.Name,
				ProblemScores: map[string]int{},
			}
		}
	}

	// Calculate highest score per problem per user
	for _, x := range s.submissions {
		if x.ContestID != cid {
			continue
		}
		e, exists := scores[x.UserID]
		if !exists {
			e = &RankEntry{
				UserID:        x.UserID,
				UserName:      x.UserName,
				ProblemScores: map[string]int{},
			}
			scores[x.UserID] = e
		}
		curBest := e.ProblemScores[x.ProblemID]
		if x.Score > curBest {
			e.ProblemScores[x.ProblemID] = x.Score
		}
	}

	// Aggregate total score and accepted count
	out := []RankEntry{}
	for _, e := range scores {
		total := 0
		accepted := 0
		for _, s := range e.ProblemScores {
			total += s
			if s == 100 {
				accepted++
			}
		}
		e.Score = total
		e.Accepted = accepted
		out = append(out, *e)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			if out[i].Accepted == out[j].Accepted {
				return out[i].UserName < out[j].UserName
			}
			return out[i].Accepted > out[j].Accepted
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
