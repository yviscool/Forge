// Package app 应用层：业务用例编排，只依赖 ports 接口，手动构造函数注入。
// 当前为地基版（创建/绑定/提交/判题/榜单），后续增量扩展鉴权与沙箱判题。
package app

import (
	"context"
	"errors"
	"fmt"
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

func (s *Service) withTx(fn func(tx ports.Store) error) error {
	if txer, ok := s.store.(ports.Transactor); ok {
		return txer.WithTx(context.Background(), fn)
	}
	return fn(s.store)
}

func (s *Service) AddUserToGroup(uid, gid string) error {
	return s.withTx(func(tx ports.Store) error {
		u, err := tx.GetUser(uid)
		if err != nil {
			return err
		}
		g, err := tx.GetGroup(gid)
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
		if err := tx.UpdateGroup(g); err != nil {
			return err
		}
		return tx.UpdateUser(u)
	})
}

func (s *Service) CreateContest(name, desc string, mode ...string) (domain.Contest, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Contest{}, errors.New("name is required")
	}
	ranking := "oi"
	if len(mode) > 0 && mode[0] != "" {
		ranking = mode[0]
	}
	if ranking != "oi" && ranking != "acm" {
		return domain.Contest{}, errors.New("unknown ranking mode")
	}
	c := domain.Contest{
		ID: s.store.NextID("cnt"), Name: name, Description: strings.TrimSpace(desc),
		Status: string(domain.Draft), RankingMode: ranking,
		ProblemIDs: []string{}, GroupIDs: []string{},
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
	if !domain.Status(c.Status).CanTransitionTo(domain.Running) {
		return domain.Contest{}, fmt.Errorf("cannot start contest from status %s", c.Status)
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
	if !c.CanSubmit() {
		return domain.Submission{}, errors.New("contest is not running")
	}
	if len(c.AllowedLanguages) > 0 {
		ok := false
		for _, l := range c.AllowedLanguages {
			if l == lang {
				ok = true
				break
			}
		}
		if !ok {
			return domain.Submission{}, errors.New("language not allowed in this contest")
		}
	}
	u, err := s.store.GetUser(userID)
	if err != nil {
		return domain.Submission{}, err
	}
	ps, err := s.store.ListProblems(contestID)
	if err != nil {
		return domain.Submission{}, err
	}
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
	s.emit("ranking.updated", x.ContestID, s.Ranking(x.ContestID))
	return x, nil
}

func (s *Service) Ranking(contestID string) []domain.RankEntry {
	c, err := s.store.GetContest(contestID)
	if err != nil {
		return []domain.RankEntry{}
	}
	users, _ := s.store.ListUsers()
	subs, _ := s.store.ListSubmissions(contestID)
	probs, _ := s.store.ListProblems(contestID)
	fullMap := map[string]int{}
	for _, p := range probs {
		full := 0
		for _, tc := range p.TestCases {
			full += tc.Score
		}
		if full > 0 {
			fullMap[p.ID] = full
		}
	}
	return domain.ComputeRanking(users, c, subs, fullMap)
}

// RankingACM ACM 赛制榜（解题数/罚时），contest.RankingMode=="acm" 时传输层选用。
func (s *Service) RankingACM(contestID string) []domain.ACMRankEntry {
	c, err := s.store.GetContest(contestID)
	if err != nil {
		return []domain.ACMRankEntry{}
	}
	users, _ := s.store.ListUsers()
	subs, _ := s.store.ListSubmissions(contestID)
	return domain.ComputeRankingACM(users, c, subs)
}

func (s *Service) Health() map[string]any {
	return map[string]any{"status": "ok", "time": time.Now().UTC()}
}
