package app

import (
	"errors"
	"strings"

	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/ports"
)

// 补全用例：查询 / 结束 / 绑定 / 题目 CRUD / 名单 / 提交流 / 按点判题。

func (s *Service) ListContests() []domain.Contest { return s.store.ListContests() }

func (s *Service) GetContest(id string) (domain.Contest, error) {
	return s.store.GetContest(id)
}

func (s *Service) FinishContest(id string) (domain.Contest, error) {
	c, err := s.store.GetContest(id)
	if err != nil {
		return domain.Contest{}, err
	}
	c.Status = string(domain.Finished)
	c.FinishedAt = s.clock.Now()
	if err := s.store.UpdateContest(c); err != nil {
		return domain.Contest{}, err
	}
	s.emit("contest.finished", id, c)
	return c, nil
}

func (s *Service) AddGroupToContest(cid, gid string) error {
	c, err := s.store.GetContest(cid)
	if err != nil {
		return err
	}
	if _, err := s.store.GetGroup(gid); err != nil {
		return err
	}
	for _, x := range c.GroupIDs {
		if x == gid {
			return nil
		}
	}
	c.GroupIDs = append(c.GroupIDs, gid)
	if err := s.store.UpdateContest(c); err != nil {
		return err
	}
	s.emit("contest.updated", cid, c)
	return nil
}

func (s *Service) AddParticipantToContest(cid, uid string) error {
	c, err := s.store.GetContest(cid)
	if err != nil {
		return err
	}
	if _, err := s.store.GetUser(uid); err != nil {
		return err
	}
	for _, x := range c.ParticipantUserIDs {
		if x == uid {
			return nil
		}
	}
	c.ParticipantUserIDs = append(c.ParticipantUserIDs, uid)
	if err := s.store.UpdateContest(c); err != nil {
		return err
	}
	s.emit("contest.updated", cid, c)
	return nil
}

func (s *Service) ListUsers() []domain.User { return s.store.ListUsers() }

func (s *Service) ListGroups() []domain.Group { return s.store.ListGroups() }

func (s *Service) RemoveUserFromGroup(uid, gid string) error {
	u, err := s.store.GetUser(uid)
	if err != nil {
		return err
	}
	g, err := s.store.GetGroup(gid)
	if err != nil {
		return err
	}
	keepU := g.UserIDs[:0]
	for _, x := range g.UserIDs {
		if x != uid {
			keepU = append(keepU, x)
		}
	}
	g.UserIDs = keepU
	keepG := u.Groups[:0]
	for _, x := range u.Groups {
		if x != gid {
			keepG = append(keepG, x)
		}
	}
	u.Groups = keepG
	if err := s.store.UpdateGroup(g); err != nil {
		return err
	}
	return s.store.UpdateUser(u)
}

func (s *Service) CreateProblem(p domain.Problem) (domain.Problem, error) {
	if err := domain.ValidateProblem(p); err != nil {
		return domain.Problem{}, err
	}
	if _, err := s.store.GetContest(p.ContestID); err != nil {
		return domain.Problem{}, errors.New("contest not found")
	}
	if strings.TrimSpace(p.Code) == "" {
		p.Code = "A"
	}
	if p.Locale == "" {
		p.Locale = "zh-CN"
	}
	if p.TimeLimitMs <= 0 {
		p.TimeLimitMs = 1000
	}
	if p.MemoryLimitMiB <= 0 {
		p.MemoryLimitMiB = 512
	}
	if p.CompareMode == "" {
		p.CompareMode = domain.CompareIgnoreSpace
	}
	p.UpdatedAt = s.clock.Now()
	created, err := s.store.CreateProblem(p)
	if err != nil {
		return domain.Problem{}, err
	}
	s.emit("problem.created", p.ContestID, created)
	return created, nil
}

func (s *Service) ListProblems(cid string) []domain.Problem {
	return s.store.ListProblems(cid)
}

func (s *Service) GetProblem(cid, pid string) (domain.Problem, error) {
	p, err := s.store.GetProblem(pid)
	if err != nil || p.ContestID != cid {
		return domain.Problem{}, errors.New("problem not found")
	}
	return p, nil
}

func (s *Service) ListSubmissions(cid string) []domain.Submission {
	return s.store.ListSubmissions(cid)
}

func (s *Service) GetSubmission(id string) (domain.Submission, error) {
	return s.store.GetSubmission(id)
}

// JudgeCases 按点回写：聚合子任务分（LemonLime dependence 语义）后落总分，
// verdict 取最差点结论映射。
func (s *Service) JudgeCases(subID string, cases []domain.CaseResult, subtaskOf func(int) int, fullOf func(int) int) (domain.Submission, error) {
	x, err := s.store.GetSubmission(subID)
	if err != nil {
		return domain.Submission{}, err
	}
	score := domain.AggregateScore(cases, subtaskOf, fullOf)
	verdict := domain.VerdictAccepted
	for _, c := range cases {
		if c.Verdict != domain.CaseAC {
			verdict = string(c.Verdict)
			break
		}
	}
	// CaseVerdict(AC/WA/...) → 服务 verdict 命名归一。
	switch domain.CaseVerdict(verdict) {
	case domain.CaseAC:
		verdict = domain.VerdictAccepted
	case domain.CaseWA:
		verdict = "wrong_answer"
	case domain.CaseTLE:
		verdict = "time_limit"
	case domain.CaseMLE:
		verdict = "memory_limit"
	case domain.CaseRE:
		verdict = "runtime_error"
	case domain.CaseCE:
		verdict = "compile_error"
	}
	x.Verdict, x.Score, x.JudgedAt = verdict, score, s.clock.Now()
	if err := s.store.UpdateSubmission(x); err != nil {
		return domain.Submission{}, err
	}
	s.emit("submission.judged", x.ContestID, x)
	s.emit("ranking.updated", x.ContestID, s.Ranking(x.ContestID))
	return x, nil
}

// Subscribe 暴露事件订阅给传输层。
func (s *Service) Subscribe(contestID string) (<-chan domain.Event, func()) {
	if s.bus == nil {
		ch := make(chan domain.Event)
		return ch, func() {}
	}
	return s.bus.Subscribe(contestID)
}

// Store 暴露底层存储（auth 会话等端口用；业务仍走用例方法）。
func (s *Service) Store() ports.Store { return s.store }
