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
	case domain.CaseOLE:
		verdict = "output_limit"
	}
	x.Verdict, x.Score, x.JudgedAt = verdict, score, s.clock.Now()
	x.Cases = cases
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
	bySub := map[int]SubtaskConfig{}
	for _, c := range cfgs {
		bySub[c.Subtask] = c
	}
	for i, tc := range p.TestCases {
		if c, ok := bySub[tc.Subtask]; ok {
			if c.Score > 0 {
				tc.Score = c.Score
			}
			if c.TimeLimitMs > 0 {
				tc.TimeLimitMs = c.TimeLimitMs
			}
			if c.MemoryMiB > 0 {
				tc.MemoryMiB = c.MemoryMiB
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
	for _, x := range s.store.ListSubmissions(cid) {
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
	subs := s.store.ListSubmissions(cid)
	st.Submissions = len(subs)
	seen := map[string]bool{}
	for _, x := range subs {
		seen[x.UserID] = true
	}
	st.Users = len(seen)
	for _, p := range s.store.ListProblems(cid) {
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
	return ContestBundle{Contest: c, Problems: s.store.ListProblems(cid)}, nil
}

// ImportBundle 导入 Bundle 为一场新比赛（新 ID，题目重挂）。
func (s *Service) ImportBundle(b ContestBundle) (domain.Contest, error) {
	name := b.Contest.Name
	if name == "" {
		name = "imported"
	}
	c, err := s.CreateContest(name+" (imported)", b.Contest.Description)
	if err != nil {
		return domain.Contest{}, err
	}
	c.Status = string(domain.Draft)
	if b.Contest.RankingMode == "acm" {
		c.RankingMode = "acm"
	}
	c.GroupIDs = append([]string{}, b.Contest.GroupIDs...)
	c.ParticipantUserIDs = append([]string{}, b.Contest.ParticipantUserIDs...)
	if err := s.store.UpdateContest(c); err != nil {
		return domain.Contest{}, err
	}
	for _, p := range b.Problems {
		p.ID = ""
		p.ContestID = c.ID
		if _, err := s.CreateProblem(p); err != nil {
			return domain.Contest{}, err
		}
	}
	return s.store.GetContest(c.ID)
}
