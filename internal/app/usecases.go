package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
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
		// Snapshot the board before marking the contest closed. Once finished,
		// all read paths return this immutable result rather than recomputing
		// from mutable user/profile state.
		c.FinalRanking = s.finalizeRanking(tx, c)
		c.FinalACM = s.finalizeACM(tx, c)
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
		if !c.CanModifySettings() {
			return errors.New("participant groups and participants are frozen once the contest starts")
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
		if !c.CanModifySettings() {
			return errors.New("participant groups and participants are frozen once the contest starts")
		}
		u, err := tx.GetUser(uid)
		if err != nil {
			return err
		}
		if domain.NormalizeRole(u.Role) != domain.RoleStudent {
			return errors.New("only student accounts can be contest participants")
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

// HasPassword 报告账号是否已设置登录口令。口令哈希永不离开 store：
// 用户列表只回一个布尔值，避免凭据随快照、导出或日志扩散。
func (s *Service) HasPassword(uid string) bool {
	hash, err := s.store.GetPasswordHash(uid)
	return err == nil && hash != ""
}

// PasswordPlain 返回教师可见的明文口令。考点统一发号时教师需要当面
// 念给考生，因此凭据表额外留了一份；鉴权仍然只比对哈希。
func (s *Service) PasswordPlain(uid string) string {
	plain, err := s.store.GetPasswordPlain(uid)
	if err != nil {
		return ""
	}
	return plain
}

// DeleteUser 删除账号：先把引用从所有分组与比赛名单里摘掉，再物理删除
// 账号、凭据与会话，避免留下"名单里有个不存在的人"这种打不开的比赛。
func (s *Service) DeleteUser(actorRole, actorID, targetID string) error {
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return errors.New("user id is required")
	}
	if targetID == actorID {
		return errors.New("cannot delete the account you are signed in with")
	}
	return s.withTx(func(tx ports.Store) error {
		target, err := tx.GetUser(targetID)
		if err != nil {
			return err
		}
		if !domain.CanManageUser(actorRole, target.Role) {
			return fmt.Errorf("forbidden: %s cannot delete %s accounts", actorRole, target.Role)
		}
		if domain.NormalizeRole(target.Role) == domain.RoleAdmin {
			users, err := tx.ListUsers()
			if err != nil {
				return err
			}
			admins := 0
			for _, u := range users {
				if domain.NormalizeRole(u.Role) == domain.RoleAdmin {
					admins++
				}
			}
			if admins <= 1 {
				return errors.New("cannot delete the last administrator")
			}
		}

		groups, err := tx.ListGroups()
		if err != nil {
			return err
		}
		for _, g := range groups {
			if !slices.Contains(g.UserIDs, targetID) {
				continue
			}
			g.UserIDs = slices.DeleteFunc(g.UserIDs, func(id string) bool { return id == targetID })
			if err := tx.UpdateGroup(g); err != nil {
				return err
			}
		}

		contests, err := tx.ListContests()
		if err != nil {
			return err
		}
		for _, c := range contests {
			if !slices.Contains(c.ParticipantUserIDs, targetID) {
				continue
			}
			c.ParticipantUserIDs = slices.DeleteFunc(c.ParticipantUserIDs,
				func(id string) bool { return id == targetID })
			if err := tx.UpdateContest(c); err != nil {
				return err
			}
		}

		// 准考证随账号一起注销：留着会让（赛季, 组别）号段里出现永远
		// 无法对号的空号，重新报名时教师也没法给同一个人补发。
		tickets, err := tx.ListTickets()
		if err != nil {
			return err
		}
		for _, t := range tickets {
			if t.UserID == targetID {
				if err := tx.DeleteTicket(t.ID); err != nil {
					return err
				}
			}
		}

		return tx.DeleteUser(targetID)
	})
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
	// Problem IDs are routing and filesystem identifiers. Never accept a
	// client-supplied value: a forged global ID could overwrite another
	// contest's problem, and traversal-shaped values would reach the data layer.
	p.ID = ""
	if p.CompareMode != "" {
		p.CompareMode = domain.NormalizeComparisonMode(p.CompareMode)
	}
	if p.TaskType == domain.TaskChoice && p.ChoiceKeyJSON == "" {
		p.ChoiceKeyJSON = choiceKeyJSON(p.ChoiceQuestions)
	}
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
			return errors.New("problem paper is frozen once the contest starts")
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

// DeleteProblem removes a problem from a contest and detaches it from the
// contest manifest. Historical submissions stay intact for audit/statistics.
func (s *Service) DeleteProblem(cid, pid string) error {
	var c domain.Contest
	err := s.withTx(func(tx ports.Store) error {
		var err error
		c, err = tx.GetContest(cid)
		if err != nil {
			return err
		}
		p, err := tx.GetProblem(pid)
		if err != nil || p.ContestID != cid {
			return errors.New("problem not found")
		}
		if !c.CanModifySettings() {
			return errors.New("problem paper is frozen once the contest starts")
		}
		kept := make([]string, 0, len(c.ProblemIDs))
		for _, id := range c.ProblemIDs {
			if id != pid {
				kept = append(kept, id)
			}
		}
		c.ProblemIDs = kept
		if err := tx.UpdateContest(c); err != nil {
			return err
		}
		return tx.DeleteProblem(pid)
	})
	if err != nil {
		return err
	}
	s.emit("problem.deleted", cid, map[string]string{"problemId": pid})
	s.emit("contest.updated", cid, c)
	return nil
}

func (s *Service) ListSubmissions(cid string) []domain.Submission {
	subs, err := s.store.ListSubmissions(cid)
	if err != nil || subs == nil {
		return []domain.Submission{}
	}
	return subs
}

func (s *Service) ListSubmissionFeed(cid, userID string, limit int) []domain.Submission {
	subs, err := s.store.ListSubmissionFeed(cid, userID, limit)
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
	var x domain.Submission
	stale := false
	err := s.withTx(func(tx ports.Store) error {
		current, err := tx.GetSubmission(subID)
		if err != nil {
			return err
		}
		// A manual decision or a newer rejudge generation wins over a late
		// worker result. This check and the write share one transaction.
		if current.Verdict != domain.VerdictQueued {
			x = current
			stale = true
			return nil
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
		case domain.CasePartly:
			verdict = domain.VerdictPartlyCorrect
		}
		current.Verdict = verdict
		current.Score = score
		current.JudgedAt = s.clock.Now()
		current.Cases = cases
		current.CompileMessage = oc.CompileMessage
		x = current
		return tx.UpdateSubmission(current)
	})
	if err != nil {
		return domain.Submission{}, err
	}
	if stale {
		return x, nil
	}
	s.emit("submission.judged", x.ContestID, x)
	s.emit("ranking.updated", x.ContestID, s.Ranking(x.ContestID))
	return x, nil
}

// Subscribe 暴露事件订阅给传输层。
func (s *Service) Subscribe(contestID, userID string) (<-chan domain.Event, func()) {
	if s.bus == nil {
		ch := make(chan domain.Event)
		return ch, func() {}
	}
	return s.bus.Subscribe(contestID, userID)
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
	if p.OutputLimitKiB > 0 {
		cur.OutputLimitKiB = p.OutputLimitKiB
	}
	if p.CompileTimeoutSec > 0 {
		cur.CompileTimeoutSec = p.CompileTimeoutSec
	}
	if p.CompareMode != "" {
		cur.CompareMode = domain.NormalizeComparisonMode(p.CompareMode)
		// Switching away from a custom checker must remove the old source;
		// otherwise BuildRequest would keep forcing special-judge mode.
		if cur.CompareMode != domain.CompareSpecialJudge {
			cur.CheckerCode = ""
			cur.CheckerLang = ""
		}
	}
	if p.RealEps != 0 {
		cur.RealEps = p.RealEps
	}
	if p.TaskType != "" {
		cur.TaskType = p.TaskType
		if p.TaskType != domain.TaskInteraction {
			cur.InteractorCode = ""
			cur.InteractorLang = ""
		}
	}
	if p.IOMode != "" {
		cur.IOMode = p.IOMode
		if p.IOMode == domain.IOStdio {
			cur.InFile = ""
			cur.OutFile = ""
		}
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
	if p.ChoiceQuestions != nil {
		cur.ChoiceQuestions = p.ChoiceQuestions
		// Keep the legacy key JSON in sync so bundles and older clients that
		// still read choiceKeyJson grade the same paper.
		cur.ChoiceKeyJSON = choiceKeyJSON(p.ChoiceQuestions)
	}
	if p.ChoiceQuestions == nil && p.ChoiceKeyJSON != "" {
		cur.ChoiceKeyJSON = p.ChoiceKeyJSON
	}
	if p.TestCases != nil {
		cur.TestCases = p.TestCases
	}
	c, err := s.store.GetContest(cur.ContestID)
	if err != nil {
		return domain.Problem{}, err
	}
	if !c.CanModifySettings() {
		return domain.Problem{}, errors.New("problem paper is frozen once the contest starts")
	}
	if err := domain.ValidateProblem(cur); err != nil {
		return domain.Problem{}, err
	}
	cur.UpdatedAt = s.clock.Now()
	updated, err := s.store.UpdateProblem(cur)
	if err != nil {
		return domain.Problem{}, err
	}
	s.emit("problem.updated", cur.ContestID, updated)
	return updated, nil
}

// choiceKeyJSON serializes only question IDs and answers. It is a compatibility
// artifact for import/export, never returned to students.
func choiceKeyJSON(questions []domain.ChoiceQuestion) string {
	keys := make(map[string]any, len(questions))
	for _, q := range questions {
		keys[strconv.Itoa(q.ID)] = q.Answer
	}
	b, err := json.Marshal(keys)
	if err != nil {
		return ""
	}
	return string(b)
}

// ReorderProblems 比赛内试题顺序重排（自动更新 A/B/C/D 编号与 Contest.ProblemIDs）。
func (s *Service) ReorderProblems(cid string, problemIDs []string) ([]domain.Problem, error) {
	var ordered []domain.Problem
	err := s.withTx(func(tx ports.Store) error {
		c, err := tx.GetContest(cid)
		if err != nil {
			return err
		}
		if !c.CanModifySettings() {
			return errors.New("problem order is frozen once the contest starts")
		}
		if len(problemIDs) == 0 || len(problemIDs) > 26 {
			return errors.New("problem order must contain between 1 and 26 problems")
		}
		current := make(map[string]bool, len(c.ProblemIDs))
		for _, pid := range c.ProblemIDs {
			current[pid] = true
		}
		seen := make(map[string]bool, len(problemIDs))
		for _, pid := range problemIDs {
			if !current[pid] {
				return fmt.Errorf("problem %s does not belong to this contest", pid)
			}
			if seen[pid] {
				return fmt.Errorf("duplicate problem in order: %s", pid)
			}
			seen[pid] = true
		}
		if len(seen) != len(current) {
			return errors.New("problem order must contain every problem exactly once")
		}
		// Validate ownership and mutate problem codes before publishing the
		// manifest. The surrounding transaction rolls every change back if a
		// later item fails.
		for i, pid := range problemIDs {
			p, err := tx.GetProblem(pid)
			if err != nil {
				return err
			}
			if p.ContestID != cid {
				return fmt.Errorf("problem %s belongs to another contest", pid)
			}
			p.Code = string(rune('A' + i))
			p.UpdatedAt = s.clock.Now()
			updated, err := tx.UpdateProblem(p)
			if err != nil {
				return err
			}
			ordered = append(ordered, updated)
		}
		c.ProblemIDs = append([]string(nil), problemIDs...)
		if err := tx.UpdateContest(c); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.emit("problems.reordered", cid, ordered)
	return ordered, nil
}

// SubtaskConfig 子任务配置项（分数/分点限额打包设置）。
type SubtaskConfig struct {
	Subtask     int   `json:"subtask"`
	Score       int   `json:"score"`
	TimeLimitMs int   `json:"timeLimitMs"`
	MemoryMiB   int   `json:"memoryMib"`
	DependsOn   []int `json:"dependsOn,omitempty"`
}

// ConfigureSubtasks 按子任务号批量设置测试点分数与限额。
func (s *Service) ConfigureSubtasks(pid string, cfgs []SubtaskConfig) (domain.Problem, error) {
	p, err := s.store.GetProblem(pid)
	if err != nil {
		return domain.Problem{}, err
	}
	// Store implementations may return a shallow copy. Work on a detached
	// slice so a rejected dependency edit cannot leak into memory storage.
	p.TestCases = append([]domain.TestCase(nil), p.TestCases...)
	c, err := s.store.GetContest(p.ContestID)
	if err != nil {
		return domain.Problem{}, err
	}
	if !c.CanModifySettings() {
		return domain.Problem{}, errors.New("subtask scoring is frozen once the contest starts")
	}
	bySub := map[int]SubtaskConfig{}
	for _, cfg := range cfgs {
		if cfg.Score < 0 || cfg.TimeLimitMs < 0 || cfg.MemoryMiB < 0 {
			return domain.Problem{}, errors.New("subtask score and limits cannot be negative")
		}
		if _, duplicate := bySub[cfg.Subtask]; duplicate {
			return domain.Problem{}, fmt.Errorf("duplicate subtask %d", cfg.Subtask)
		}
		bySub[cfg.Subtask] = cfg
	}
	present := map[int]int{}
	for _, tc := range p.TestCases {
		present[tc.Subtask]++
	}
	for subtask := range bySub {
		if present[subtask] == 0 {
			return domain.Problem{}, fmt.Errorf("subtask %d has no test cases", subtask)
		}
	}
	// SubtaskConfig.Score is the total score for the whole subtask. Split it
	// deterministically across its test points instead of multiplying the score
	// by the number of cases.
	scoreByCase := map[int]int{}
	for subtask, cfg := range bySub {
		if cfg.Score <= 0 {
			continue
		}
		count := present[subtask]
		base, remainder := cfg.Score/count, cfg.Score%count
		seen := 0
		for i, tc := range p.TestCases {
			if tc.Subtask != subtask {
				continue
			}
			scoreByCase[i] = base
			if seen < remainder {
				scoreByCase[i]++
			}
			seen++
		}
	}
	for i, tc := range p.TestCases {
		if cfg, ok := bySub[tc.Subtask]; ok {
			if cfg.Score > 0 {
				tc.Score = scoreByCase[i]
			}
			if cfg.TimeLimitMs > 0 {
				tc.TimeLimitMs = cfg.TimeLimitMs
			}
			if cfg.MemoryMiB > 0 {
				tc.MemoryMiB = cfg.MemoryMiB
			}
			if cfg.DependsOn != nil {
				tc.DependsOn = append([]int(nil), cfg.DependsOn...)
			}
			p.TestCases[i] = tc
		}
	}
	if err := domain.ValidateProblem(p); err != nil {
		return domain.Problem{}, err
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
	var out []domain.Submission
	err := s.withTx(func(tx ports.Store) error {
		problem, err := tx.GetProblem(pid)
		if err != nil {
			return err
		}
		if problem.ContestID != cid {
			return errors.New("problem not found in contest")
		}
		subs, err := tx.ListSubmissions(cid)
		if err != nil {
			return err
		}
		for _, x := range subs {
			if x.ProblemID != pid {
				continue
			}
			// Preserve the previous score and case detail for audit and a stable
			// OI board while the submission is visibly queued for re-evaluation.
			x.Verdict = domain.VerdictQueued
			if err := tx.UpdateSubmission(x); err != nil {
				return err
			}
			out = append(out, x)
		}
		return nil
	})
	if err != nil {
		return nil, err
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
	st := ContestStats{ContestID: cid, Problems: []ProblemStats{}}
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
			Name:             name + " (imported)",
			Description:      b.Contest.Description,
			Status:           string(domain.Draft),
			RankingMode:      b.Contest.RankingMode,
			AllowedLanguages: append([]string{}, b.Contest.AllowedLanguages...),
			CreatedAt:        s.clock.Now(),
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
			p.CompareMode = domain.NormalizeComparisonMode(p.CompareMode)
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
		return domain.Contest{}, errors.New("contest settings are frozen once the contest starts")
	}
	var clean []string
	seen := map[string]bool{}
	for _, l := range langs {
		l = domain.NormalizeLanguage(l)
		if l == "" {
			continue
		}
		if !domain.SupportedLanguage(l) {
			return domain.Contest{}, fmt.Errorf("unsupported language: %s", l)
		}
		if seen[l] {
			continue
		}
		seen[l] = true
		clean = append(clean, l)
	}
	c.AllowedLanguages = clean
	if err := s.store.UpdateContest(c); err != nil {
		return domain.Contest{}, err
	}
	s.emit("contest.updated", cid, c)
	return c, nil
}

// SetContestTicketScope 设置本场比赛使用的赛季与组别。
//
// 准考证是"人"的属性而不是"场次"的属性：同一个人在同一赛季的同一组别里
// 只有一张准考证，一整季的模拟赛、常规赛、正式赛都用同一张。
// 比赛侧只声明"我要收哪一批人的题"，导出收题包时按 (赛季, 组别) 反查准考证。
// 空赛季表示这场不按准考证收题（普通在线赛），导出时退回用户名目录。
func (s *Service) SetContestTicketScope(cid, season, group string) (domain.Contest, error) {
	c, err := s.store.GetContest(cid)
	if err != nil {
		return domain.Contest{}, err
	}
	if !c.CanModifySettings() {
		return domain.Contest{}, errors.New("contest settings are frozen once the contest starts")
	}
	season = domain.NormalizeSeason(season)
	if season != "" {
		if err := domain.ValidateSeason(season); err != nil {
			return domain.Contest{}, err
		}
	}
	group = strings.ToUpper(strings.TrimSpace(group))
	if group != "" && group != domain.TicketGroupJunior && group != domain.TicketGroupSenior {
		return domain.Contest{}, fmt.Errorf("unknown ticket group %q", group)
	}
	if group != "" && season == "" {
		return domain.Contest{}, errors.New("ticket group requires a season")
	}
	c.Season = season
	c.TicketGroup = group
	if err := s.store.UpdateContest(c); err != nil {
		return domain.Contest{}, err
	}
	s.emit("contest.updated", cid, c)
	return c, nil
}

// SetContestMode 切换比赛组织方式（online/noi/gesp）。
// 与榜单赛制一样属于开赛前可调的设置：启动后冻结，保证全场口径一致。
func (s *Service) SetContestMode(cid, mode string) (domain.Contest, error) {
	if !domain.ValidMode(mode) {
		return domain.Contest{}, fmt.Errorf("unknown contest mode %q", mode)
	}
	c, err := s.store.GetContest(cid)
	if err != nil {
		return domain.Contest{}, err
	}
	if !c.CanModifySettings() {
		return domain.Contest{}, errors.New("contest settings are frozen once the contest starts")
	}
	c.Mode = domain.NormalizeMode(mode)
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
		return domain.Contest{}, errors.New("contest settings are frozen once the contest starts")
	}
	c.RankingMode = mode
	if err := s.store.UpdateContest(c); err != nil {
		return domain.Contest{}, err
	}
	s.emit("contest.updated", cid, c)
	return c, nil
}
