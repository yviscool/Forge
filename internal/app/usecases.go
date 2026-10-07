package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/ports"
)

// 补全用例：查询 / 结束 / 绑定 / 题目 CRUD / 名单 / 提交流 / 按点判题。

func (s *Service) ListContests() []domain.Contest {
	cs, err := s.store.ListContests()
	if err != nil || cs == nil {
		return []domain.Contest{}
	}
	return cs
}

func (s *Service) GetContest(id string) (domain.Contest, error) {
	return s.store.GetContest(id)
}

func (s *Service) FinishContest(id string) (domain.Contest, error) {
	var c domain.Contest
	err := s.withTx(func(tx ports.Store) error {
		var err error
		c, err = tx.GetContest(id)
		if err != nil {
			return err
		}
		if !c.CanTransitionTo(domain.Finished) {
			return fmt.Errorf("contest cannot transition from %s to finished", c.Status)
		}
		c.Status = string(domain.Finished)
		c.FinishedAt = s.clock.Now()
		return tx.UpdateContest(c)
	})
	if err != nil {
		return domain.Contest{}, err
	}
	s.emit("contest.finished", id, c)
	return c, nil
}

func (s *Service) AddGroupToContest(cid, gid string) error {
	var c domain.Contest
	err := s.withTx(func(tx ports.Store) error {
		var err error
		c, err = tx.GetContest(cid)
		if err != nil {
			return err
		}
		if _, err := tx.GetGroup(gid); err != nil {
			return err
		}
		for _, x := range c.GroupIDs {
			if x == gid {
				return nil
			}
		}
		c.GroupIDs = append(c.GroupIDs, gid)
		return tx.UpdateContest(c)
	})
	if err != nil {
		return err
	}
	s.emit("contest.updated", cid, c)
	return nil
}

func (s *Service) AddParticipantToContest(cid, uid string) error {
	var c domain.Contest
	err := s.withTx(func(tx ports.Store) error {
		var err error
		c, err = tx.GetContest(cid)
		if err != nil {
			return err
		}
		if _, err := tx.GetUser(uid); err != nil {
			return err
		}
		for _, x := range c.ParticipantUserIDs {
			if x == uid {
				return nil
			}
		}
		c.ParticipantUserIDs = append(c.ParticipantUserIDs, uid)
		return tx.UpdateContest(c)
	})
	if err != nil {
		return err
	}
	s.emit("contest.updated", cid, c)
	return nil
}

func (s *Service) ListUsers() []domain.User {
	us, err := s.store.ListUsers()
	if err != nil || us == nil {
		return []domain.User{}
	}
	return us
}

func (s *Service) ListGroups() []domain.Group {
	gs, err := s.store.ListGroups()
	if err != nil || gs == nil {
		return []domain.Group{}
	}
	return gs
}

func (s *Service) RemoveUserFromGroup(uid, gid string) error {
	return s.withTx(func(tx ports.Store) error {
		u, err := tx.GetUser(uid)
		if err != nil {
			return err
		}
		g, err := tx.GetGroup(gid)
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
		if err := tx.UpdateGroup(g); err != nil {
			return err
		}
		return tx.UpdateUser(u)
	})
}

func (s *Service) CreateProblem(p domain.Problem) (domain.Problem, error) {
	if err := domain.ValidateProblem(p); err != nil {
		return domain.Problem{}, err
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

	var created domain.Problem
	err := s.withTx(func(tx ports.Store) error {
		c, err := tx.GetContest(p.ContestID)
		if err != nil {
			return errors.New("contest not found")
		}
		if !c.CanModifySettings() {
			return errors.New("cannot add problems to finished contest")
		}
		created, err = tx.CreateProblem(p)
		if err != nil {
			return err
		}
		has := false
		for _, x := range c.ProblemIDs {
			if x == created.ID {
				has = true
				break
			}
		}
		if !has {
			c.ProblemIDs = append(c.ProblemIDs, created.ID)
			if err := tx.UpdateContest(c); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return domain.Problem{}, err
	}
	s.emit("problem.created", p.ContestID, created)
	return created, nil
}

func (s *Service) ListProblems(cid string) []domain.Problem {
	ps, err := s.store.ListProblems(cid)
	if err != nil || ps == nil {
		return []domain.Problem{}
	}
	return ps
}

func (s *Service) GetProblem(cid, pid string) (domain.Problem, error) {
	p, err := s.store.GetProblem(pid)
	if err != nil || p.ContestID != cid {
		return domain.Problem{}, errors.New("problem not found")
	}
	return p, nil
}

func (s *Service) ListSubmissions(cid string) []domain.Submission {
	subs, err := s.store.ListSubmissions(cid)
	if err != nil || subs == nil {
		return []domain.Submission{}
	}
	return subs
}

func (s *Service) GetSubmission(id string) (domain.Submission, error) {
	return s.store.GetSubmission(id)
}

// JudgeCases 按点回写：聚合子任务分（LemonLime dependence 语义）后落总分，
// verdict 取最差点结论映射；编译信息（CE 原文）一并落库。
func (s *Service) JudgeCases(subID string, oc domain.JudgeOutcome, subtaskOf func(int) int, fullOf func(int) int) (domain.Submission, error) {
	x, err := s.store.GetSubmission(subID)
	if err != nil {
		return domain.Submission{}, err
	}
	cases := oc.Cases
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
		verdict = domain.VerdictWrongAnswer
	case domain.CaseTLE:
		verdict = domain.VerdictTimeLimit
	case domain.CaseMLE:
		verdict = domain.VerdictMemoryLimit
	case domain.CaseRE:
		verdict = domain.VerdictRuntimeError
	case domain.CaseCE:
		verdict = domain.VerdictCompileError
	case domain.CaseOLE:
		verdict = domain.VerdictOutputLimit
	case domain.CasePE:
		verdict = domain.VerdictPresentationError
	case domain.CaseCheckerError:
		verdict = domain.VerdictCheckerError
	case domain.CaseSystemError:
		verdict = domain.VerdictSystemError
	case domain.CaseSkipped:
		verdict = domain.VerdictSkipped
	}
	x.Verdict, x.Score, x.JudgedAt = verdict, score, s.clock.Now()
	x.Cases = cases
	x.CompileMessage = oc.CompileMessage
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

func (s *Service) UpdateProblem(p domain.Problem) (domain.Problem, error) {
	cur, err := s.store.GetProblem(p.ID)
	if err != nil {
		return domain.Problem{}, err
	}
	if p.ContestID != "" && p.ContestID != cur.ContestID {
		return domain.Problem{}, errors.New("contestId is immutable")
	}
	// 合并语义：只覆盖非零字段（表单局部更新不洗掉整题）。
	if p.Title != "" {
		cur.Title = p.Title
	}
	if p.Code != "" {
		cur.Code = p.Code
	}
	if p.Statement != "" {
		cur.Statement = p.Statement
	}
	if p.Input != "" {
		cur.Input = p.Input
	}
	if p.Output != "" {
		cur.Output = p.Output
	}
	if p.Constraints != "" {
		cur.Constraints = p.Constraints
	}
	if p.Examples != "" {
		cur.Examples = p.Examples
	}
	if p.Locale != "" {
		cur.Locale = p.Locale
	}
	if p.TimeLimitMs > 0 {
		cur.TimeLimitMs = p.TimeLimitMs
	}
	if p.MemoryLimitMiB > 0 {
		cur.MemoryLimitMiB = p.MemoryLimitMiB
	}
	if p.CompareMode != "" {
		cur.CompareMode = p.CompareMode
	}
	if p.RealEps != 0 {
		cur.RealEps = p.RealEps
	}
	if p.TaskType != "" {
		cur.TaskType = p.TaskType
	}
	if p.IOMode != "" {
		cur.IOMode = p.IOMode
	}
	if p.InFile != "" {
		cur.InFile = p.InFile
	}
	if p.OutFile != "" {
		cur.OutFile = p.OutFile
	}
	if p.CheckerCode != "" {
		cur.CheckerCode = p.CheckerCode
		cur.CheckerLang = p.CheckerLang
	}
	if p.InteractorCode != "" {
		cur.InteractorCode = p.InteractorCode
		cur.InteractorLang = p.InteractorLang
	}
	if p.Locales != nil {
		cur.Locales = p.Locales
	}
	if p.TestCases != nil {
		cur.TestCases = p.TestCases
	}
	c, err := s.store.GetContest(cur.ContestID)
	if err != nil {
		return domain.Problem{}, err
	}
	if !c.CanModifySettings() {
		return domain.Problem{}, errors.New("cannot update problem of finished contest")
	}
	cur.UpdatedAt = s.clock.Now()
	updated, err := s.store.UpdateProblem(cur)
	if err != nil {
		return domain.Problem{}, err
	}
	s.emit("problem.updated", cur.ContestID, updated)
	return updated, nil
}

// SubtaskConfig 子任务配置项（分数/分点限额打包设置）。
type SubtaskConfig struct {
	Subtask     int `json:"subtask"`
	Score       int `json:"score"`
	TimeLimitMs int `json:"timeLimitMs"`
	MemoryMiB   int `json:"memoryMib"`
}

// ConfigureSubtasks 按子任务号批量设置测试点分数与限额。
func (s *Service) ConfigureSubtasks(pid string, cfgs []SubtaskConfig) (domain.Problem, error) {
	p, err := s.store.GetProblem(pid)
	if err != nil {
		return domain.Problem{}, err
	}
	c, err := s.store.GetContest(p.ContestID)
	if err != nil {
		return domain.Problem{}, err
	}
	if !c.CanModifySettings() {
		return domain.Problem{}, errors.New("cannot configure subtasks of finished contest")
	}
	bySub := map[int]SubtaskConfig{}
	for _, cfg := range cfgs {
		bySub[cfg.Subtask] = cfg
	}
	for i, tc := range p.TestCases {
		if cfg, ok := bySub[tc.Subtask]; ok {
			if cfg.Score > 0 {
				tc.Score = cfg.Score
			}
			if cfg.TimeLimitMs > 0 {
				tc.TimeLimitMs = cfg.TimeLimitMs
			}
			if cfg.MemoryMiB > 0 {
				tc.MemoryMiB = cfg.MemoryMiB
			}
			p.TestCases[i] = tc
		}
	}
	p.UpdatedAt = s.clock.Now()
	updated, err := s.store.UpdateProblem(p)
	if err != nil {
		return domain.Problem{}, err
	}
	s.emit("problem.updated", p.ContestID, updated)
	return updated, nil
}

// RejudgeProblem 重判某题全部提交（对标 LemonLime needRejudge）。
// 返回待重判提交，调用方逐个入 worker 队列。
func (s *Service) RejudgeProblem(cid, pid string) ([]domain.Submission, error) {
	if _, err := s.GetProblem(cid, pid); err != nil {
		return nil, err
	}
	var out []domain.Submission
	subs, err := s.store.ListSubmissions(cid)
	if err != nil {
		return nil, err
	}
	for _, x := range subs {
		if x.ProblemID == pid {
			out = append(out, x)
		}
	}
	s.emit("problem.rejudge", cid, map[string]any{"problemId": pid, "count": len(out)})
	return out, nil
}

// ProblemStats 单题统计。
type ProblemStats struct {
	ProblemID   string  `json:"problemId"`
	Code        string  `json:"code"`
	Title       string  `json:"title"`
	Submissions int     `json:"submissions"`
	Accepted    int     `json:"accepted"`
	AcceptRate  float64 `json:"acceptRate"`
	BestScore   int     `json:"bestScore"`
	FirstACUser string  `json:"firstACUser,omitempty"`
}

// ContestStats 比赛统计（对标 LemonLime statisticsbrowser 轻量版）。
type ContestStats struct {
	ContestID   string         `json:"contestId"`
	Users       int            `json:"users"`
	Submissions int            `json:"submissions"`
	Problems    []ProblemStats `json:"problems"`
}

func (s *Service) Statistics(cid string) (ContestStats, error) {
	if _, err := s.store.GetContest(cid); err != nil {
		return ContestStats{}, err
	}
	st := ContestStats{ContestID: cid}
	subs, err := s.store.ListSubmissions(cid)
	if err != nil {
		return ContestStats{}, err
	}
	st.Submissions = len(subs)
	seen := map[string]bool{}
	for _, x := range subs {
		seen[x.UserID] = true
	}
	st.Users = len(seen)
	probs, err := s.store.ListProblems(cid)
	if err != nil {
		return ContestStats{}, err
	}
	for _, p := range probs {
		ps := ProblemStats{ProblemID: p.ID, Code: p.Code, Title: p.Title}
		var firstAt int64 = -1
		for _, x := range subs {
			if x.ProblemID != p.ID {
				continue
			}
			ps.Submissions++
			if x.Score > ps.BestScore {
				ps.BestScore = x.Score
			}
			if x.Verdict == domain.VerdictAccepted {
				ps.Accepted++
				at := x.JudgedAt.Unix()
				if x.JudgedAt.IsZero() {
					at = x.SubmittedAt.Unix()
				}
				if firstAt < 0 || at < firstAt {
					firstAt = at
					ps.FirstACUser = x.UserName
				}
			}
		}
		if ps.Submissions > 0 {
			ps.AcceptRate = float64(ps.Accepted) / float64(ps.Submissions)
		}
		st.Problems = append(st.Problems, ps)
	}
	return st, nil
}

// ContestBundle 比赛Bundle（对标 LemonLime .cdf 读写往返）：比赛 + 题目全量，
// 不含提交与会话。导入时重发 ID，原数据只读。
type ContestBundle struct {
	Contest  domain.Contest   `json:"contest"`
	Problems []domain.Problem `json:"problems"`
}

// ExportContest 导出比赛 Bundle。
func (s *Service) ExportContest(cid string) (ContestBundle, error) {
	c, err := s.store.GetContest(cid)
	if err != nil {
		return ContestBundle{}, err
	}
	probs, err := s.store.ListProblems(cid)
	if err != nil {
		return ContestBundle{}, err
	}
	return ContestBundle{Contest: c, Problems: probs}, nil
}

// ImportBundle 导入 Bundle 为一场新比赛（新 ID，题目重挂，原子事务）。
func (s *Service) ImportBundle(b ContestBundle) (domain.Contest, error) {
	name := b.Contest.Name
	if name == "" {
		name = "imported"
	}
	var out domain.Contest
	err := s.withTx(func(tx ports.Store) error {
		c, err := tx.CreateContest(domain.Contest{
			Name:               name + " (imported)",
			Description:        b.Contest.Description,
			Status:             string(domain.Draft),
			RankingMode:        b.Contest.RankingMode,
			GroupIDs:           append([]string{}, b.Contest.GroupIDs...),
			ParticipantUserIDs: append([]string{}, b.Contest.ParticipantUserIDs...),
			AllowedLanguages:   append([]string{}, b.Contest.AllowedLanguages...),
			CreatedAt:          s.clock.Now(),
		})
		if err != nil {
			return err
		}
		for _, p := range b.Problems {
			p.ID = ""
			p.ContestID = c.ID
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
			if err := domain.ValidateProblem(p); err != nil {
				return err
			}
			p.UpdatedAt = s.clock.Now()
			createdP, err := tx.CreateProblem(p)
			if err != nil {
				return err
			}
			c.ProblemIDs = append(c.ProblemIDs, createdP.ID)
		}
		if err := tx.UpdateContest(c); err != nil {
			return err
		}
		out = c
		return nil
	})
	if err != nil {
		return domain.Contest{}, err
	}
	s.emit("contest.created", out.ID, out)
	return out, nil
}

// SetContestLanguages 设置比赛语言白名单（空表示不限；CSP 复赛填 ["cpp"]）。
func (s *Service) SetContestLanguages(cid string, langs []string) (domain.Contest, error) {
	c, err := s.store.GetContest(cid)
	if err != nil {
		return domain.Contest{}, err
	}
	if !c.CanModifySettings() {
		return domain.Contest{}, errors.New("cannot modify settings of finished contest")
	}
	var clean []string
	for _, l := range langs {
		if l = strings.TrimSpace(l); l != "" {
			clean = append(clean, l)
		}
	}
	c.AllowedLanguages = clean
	if err := s.store.UpdateContest(c); err != nil {
		return domain.Contest{}, err
	}
	s.emit("contest.updated", cid, c)
	return c, nil
}

// SetRankingMode 切换榜单模式（oi/acm）。
func (s *Service) SetRankingMode(cid, mode string) (domain.Contest, error) {
	if mode != "oi" && mode != "acm" {
		return domain.Contest{}, errors.New("unknown ranking mode")
	}
	c, err := s.store.GetContest(cid)
	if err != nil {
		return domain.Contest{}, err
	}
	if !c.CanModifySettings() {
		return domain.Contest{}, errors.New("cannot modify settings of finished contest")
	}
	c.RankingMode = mode
	if err := s.store.UpdateContest(c); err != nil {
		return domain.Contest{}, err
	}
	s.emit("contest.updated", cid, c)
	return c, nil
}
