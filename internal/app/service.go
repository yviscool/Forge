// Package app 应用层：业务用例编排，只依赖 ports 接口，手动构造函数注入。
// 当前为地基版（创建/绑定/提交/判题/榜单），后续增量扩展鉴权与沙箱判题。
package app

import (
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/ports"
)

type Service struct {
	store ports.Store
	bus   ports.Broadcaster
	clock ports.Clock
	log   *slog.Logger
}

// NewService 手动注入装配：store/bus 必填，clock/log 可空（自动补缺省）。
func NewService(store ports.Store, bus ports.Broadcaster, clock ports.Clock, log *slog.Logger) *Service {
	if clock == nil {
		clock = ports.SystemClock{}
	}
	if log == nil {
		log = slog.Default()
	}
	return &Service{store: store, bus: bus, clock: clock, log: log}
}

func (s *Service) emit(t, cid string, data any) {
	if s.bus == nil {
		return
	}
	s.bus.Publish(domain.Event{Type: t, ContestID: cid, Data: data, At: s.clock.Now()})
}

func (s *Service) CreateUser(name, role string) (domain.User, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.User{}, errors.New("name is required")
	}
	username := strings.ToLower(strings.ReplaceAll(name, " ", "_"))
	u := domain.User{
		ID: s.store.NextID("usr"), Username: username, Name: name,
		Role: domain.NormalizeRole(role), Groups: []string{}, CreatedAt: s.clock.Now(),
	}
	return s.store.CreateUser(u)
}

func (s *Service) CreateGroup(name string) (domain.Group, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Group{}, errors.New("name is required")
	}
	g := domain.Group{ID: s.store.NextID("grp"), Name: name, UserIDs: []string{}, CreatedAt: s.clock.Now()}
	return s.store.CreateGroup(g)
}

func (s *Service) AddUserToGroup(uid, gid string) error {
	u, err := s.store.GetUser(uid)
	if err != nil {
		return err
	}
	g, err := s.store.GetGroup(gid)
	if err != nil {
		return err
	}
	for _, x := range g.UserIDs {
		if x == uid {
			return nil
		}
	}
	g.UserIDs = append(g.UserIDs, uid)
	u.Groups = append(u.Groups, gid)
	if err := s.store.UpdateGroup(g); err != nil {
		return err
	}
	return s.store.UpdateUser(u)
}

func (s *Service) CreateContest(name, desc string) (domain.Contest, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Contest{}, errors.New("name is required")
	}
	c := domain.Contest{
		ID: s.store.NextID("cnt"), Name: name, Description: strings.TrimSpace(desc),
		Status: string(domain.Draft), ProblemIDs: []string{}, GroupIDs: []string{},
		ParticipantUserIDs: []string{}, CreatedAt: s.clock.Now(),
	}
	created, err := s.store.CreateContest(c)
	if err != nil {
		return domain.Contest{}, err
	}
	s.emit("contest.created", created.ID, created)
	return created, nil
}

func (s *Service) StartContest(id string) (domain.Contest, error) {
	c, err := s.store.GetContest(id)
	if err != nil {
		return domain.Contest{}, err
	}
	if c.Status == string(domain.Finished) {
		return domain.Contest{}, errors.New("contest already finished")
	}
	c.Status = string(domain.Running)
	c.StartedAt = s.clock.Now()
	if err := s.store.UpdateContest(c); err != nil {
		return domain.Contest{}, err
	}
	s.emit("contest.started", id, c)
	return c, nil
}

func (s *Service) Submit(contestID, problemID, userID, lang, code string) (domain.Submission, error) {
	c, err := s.store.GetContest(contestID)
	if err != nil {
		return domain.Submission{}, err
	}
	if c.Status != string(domain.Running) {
		return domain.Submission{}, errors.New("contest is not running")
	}
	u, err := s.store.GetUser(userID)
	if err != nil {
		return domain.Submission{}, err
	}
	ps := s.store.ListProblems(contestID)
	found := false
	for _, p := range ps {
		if p.ID == problemID {
			found = true
			break
		}
	}
	if !found {
		return domain.Submission{}, errors.New("problem not found")
	}
	if !domain.IsUserAllowed(c, u) {
		return domain.Submission{}, errors.New("user is not registered for this contest")
	}
	x := domain.Submission{
		ID: s.store.NextID("sub"), ContestID: contestID, ProblemID: problemID,
		UserID: userID, UserName: u.Name, Language: lang, Code: code,
		Verdict: domain.VerdictQueued, SubmittedAt: s.clock.Now(),
	}
	created, err := s.store.CreateSubmission(x)
	if err != nil {
		return domain.Submission{}, err
	}
	s.emit("submission.created", contestID, created)
	return created, nil
}

func (s *Service) Judge(id, verdict string, score int) (domain.Submission, error) {
	x, err := s.store.GetSubmission(id)
	if err != nil {
		return domain.Submission{}, err
	}
	x.Verdict, x.Score, x.JudgedAt = verdict, score, s.clock.Now()
	if err := s.store.UpdateSubmission(x); err != nil {
		return domain.Submission{}, err
	}
	s.emit("submission.judged", x.ContestID, x)
	s.emit("ranking.updated", x.ContestID, s.Ranking(x.ContestID), )
	return x, nil
}

func (s *Service) Ranking(contestID string) []domain.RankEntry {
	c, err := s.store.GetContest(contestID)
	if err != nil {
		return []domain.RankEntry{}
	}
	return domain.ComputeRanking(s.store.ListUsers(), c, s.store.ListSubmissions(contestID))
}

func (s *Service) Health() map[string]any {
	return map[string]any{"status": "ok", "time": time.Now().UTC()}
}
