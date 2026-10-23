// Package app 应用层：业务用例编排，只依赖 ports 接口，手动构造函数注入。
// 当前为地基版（创建/绑定/提交/判题/榜单），后续增量扩展鉴权与沙箱判题。
package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/ports"
)

var ErrIdempotencyConflict = errors.New("idempotency key was already used for a different submission")

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
	return s.CreateUserWithUsername(name, "", role)
}

// CreateUserWithUsername 旧签名入口（不带省份/准考证号）。
func (s *Service) CreateUserWithUsername(name, username, role string) (domain.User, error) {
	return s.createUserRecord(name, username, role, "", "")
}

// createUserRecord 账号创建核心：登录名冲突时自动加后缀；
// 省份与准考证号由调用方先行归一化校验（见 provinces.go）。
func (s *Service) createUserRecord(name, username, role, province, examTicket string) (domain.User, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.User{}, errors.New("name is required")
	}
	username = strings.TrimSpace(username)
	if username == "" {
		username = strings.ToLower(strings.ReplaceAll(name, " ", "_"))
	}
	var created domain.User
	err := s.withTx(func(tx ports.Store) error {
		existing, err := tx.ListUsers()
		if err != nil {
			return err
		}
		base := username
		idx := 1
		for {
			collision := false
			for _, u := range existing {
				if strings.EqualFold(u.Username, username) {
					collision = true
					break
				}
			}
			if !collision {
				break
			}
			idx++
			username = fmt.Sprintf("%s_%d", base, idx)
		}
		created, err = tx.CreateUser(domain.User{
			ID: tx.NextID("usr"), Username: username, Name: name,
			Role: domain.NormalizeRole(role), Province: province, ExamTicket: examTicket,
			Groups: []string{}, CreatedAt: s.clock.Now(),
		})
		return err
	})
	if err != nil {
		return domain.User{}, err
	}
	return created, nil
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
		if domain.NormalizeRole(u.Role) != domain.RoleStudent {
			return errors.New("only student accounts can join participant groups")
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
	ranking := ""
	if len(mode) > 0 {
		ranking = mode[0]
	}
	return s.CreateContestWithMode(name, desc, ranking, "")
}

// CreateContestWithMode 创建比赛：rankingMode 为 oi/acm 榜单赛制，
// contestMode 为组织方式（domain.ModeOnline / ModeNOI / ModeGESP，空=online）。
func (s *Service) CreateContestWithMode(name, desc, rankingMode, contestMode string) (domain.Contest, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Contest{}, errors.New("name is required")
	}
	ranking := "oi"
	if strings.TrimSpace(rankingMode) != "" {
		ranking = rankingMode
	}
	if ranking != "oi" && ranking != "acm" {
		return domain.Contest{}, errors.New("unknown ranking mode")
	}
	if !domain.ValidMode(contestMode) {
		return domain.Contest{}, fmt.Errorf("unknown contest mode %q", contestMode)
	}
	c := domain.Contest{
		ID: s.store.NextID("cnt"), Name: name, Description: strings.TrimSpace(desc),
		Status: string(domain.Draft), RankingMode: ranking, Mode: domain.NormalizeMode(contestMode),
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
	var c domain.Contest
	err := s.withTx(func(tx ports.Store) error {
		var err error
		c, err = tx.GetContest(id)
		if err != nil {
			return err
		}
		if !domain.Status(c.Status).CanTransitionTo(domain.Running) {
			return fmt.Errorf("cannot start contest from status %s", c.Status)
		}
		problems, err := tx.ListProblems(id)
		if err != nil {
			return err
		}
		groups, err := tx.ListGroups()
		if err != nil {
			return err
		}
		users, err := tx.ListUsers()
		if err != nil {
			return err
		}
		if err := domain.ValidateContestStart(c, problems, groups, users, s.clock.Now()); err != nil {
			return err
		}
		c.Status = string(domain.Running)
		c.StartedAt = s.clock.Now()
		return tx.UpdateContest(c)
	})
	if err != nil {
		return domain.Contest{}, err
	}
	s.emit("contest.started", id, c)
	return c, nil
}

func (s *Service) Submit(contestID, problemID, userID, lang, code string) (domain.Submission, error) {
	return s.SubmitWithClient(contestID, problemID, userID, lang, code, "")
}

func (s *Service) SubmitWithClient(contestID, problemID, userID, lang, code, clientIP string) (domain.Submission, error) {
	created, _, err := s.submit(contestID, problemID, userID, lang, code, clientIP, "")
	return created, err
}

// SubmitIdempotent returns the existing submission when the same client
// request is retried. The key is scoped to both the candidate and contest so
// clients cannot collide across users or exams.
func (s *Service) SubmitIdempotent(
	contestID, problemID, userID, lang, code, clientIP, requestKey string,
) (domain.Submission, bool, error) {
	return s.submit(contestID, problemID, userID, lang, code, clientIP, requestKey)
}

func submissionRequestID(contestID, userID, requestKey string) string {
	if strings.TrimSpace(requestKey) == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(contestID + "\x00" + userID + "\x00" + requestKey))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func (s *Service) submit(
	contestID, problemID, userID, lang, code, clientIP, requestKey string,
) (domain.Submission, bool, error) {
	var created domain.Submission
	var ipBound bool
	autoJudged := false
	replayed := false
	requestID := submissionRequestID(contestID, userID, requestKey)
	err := s.withTx(func(tx ports.Store) error {
		c, err := tx.GetContest(contestID)
		if err != nil {
			return err
		}
		u, err := tx.GetUser(userID)
		if err != nil {
			return err
		}
		ps, err := tx.ListProblems(contestID)
		if err != nil {
			return err
		}
		var targetProb domain.Problem
		found := false
		for _, p := range ps {
			if p.ID == problemID {
				targetProb = p
				found = true
				break
			}
		}
		if !found {
			return errors.New("problem not found")
		}

		expectedLanguage := lang
		if targetProb.TaskType != domain.TaskChoice &&
			targetProb.TaskType != domain.TaskAnswersOnly {
			expectedLanguage = domain.NormalizeLanguage(lang)
		}
		if targetProb.TaskType == domain.TaskAnswersOnly {
			expectedLanguage = "answer"
		}

		// A retry must return the original durable decision even after the
		// contest closes or an operator changes policy. The request key is
		// already scoped to contest + account, and the payload comparison below
		// prevents a key from being reused for different work.
		if requestID != "" {
			existing, err := tx.FindSubmissionByRequestID(requestID)
			if err != nil {
				return err
			}
			if existing != nil {
				if existing.ProblemID != problemID ||
					existing.Language != expectedLanguage ||
					existing.Code != code {
					return ErrIdempotencyConflict
				}
				created = *existing
				replayed = true
				return nil
			}
		}

		if !domain.IsUserAllowed(c, u) {
			return errors.New("user is not registered for this contest")
		}
		if strings.TrimSpace(code) == "" {
			return errors.New("submission cannot be empty")
		}
		if len(code) > domain.MaxSubmissionBytes {
			return fmt.Errorf(
				"submission_too_large: submission exceeds %d bytes", domain.MaxSubmissionBytes,
			)
		}
		if targetProb.TaskType != domain.TaskChoice &&
			targetProb.TaskType != domain.TaskAnswersOnly &&
			len(c.AllowedLanguages) > 0 {
			allowedLanguage := false
			for _, allowed := range c.AllowedLanguages {
				if domain.NormalizeLanguage(allowed) == expectedLanguage {
					allowedLanguage = true
					break
				}
			}
			if !allowedLanguage {
				return errors.New("language not allowed in this contest")
			}
		}

		if !c.CanSubmit() {
			return errors.New("contest is not running")
		}
		if c.WindowEnabled {
			now := s.clock.Now()
			if c.StartTime != nil && now.Before(*c.StartTime) {
				return errors.New("exam has not started; submissions are disabled")
			}
			if c.EndTime != nil {
				deadline := *c.EndTime
				if ext, ok := c.Extensions[userID]; ok && ext > 0 {
					deadline = deadline.Add(time.Duration(ext) * time.Minute)
				}
				if now.After(deadline) {
					return errors.New("exam time has ended; submissions are disabled")
				}
			}
		}

		// Bind and submit in the same transaction. Two simultaneous first
		// submissions from different seats can no longer both pass the check.
		if c.BindIPEnabled && clientIP != "" {
			bindings := make(map[string]string, len(c.BoundIPs)+1)
			for uid, ip := range c.BoundIPs {
				bindings[uid] = ip
			}
			if bound := bindings[userID]; bound != "" {
				if bound != clientIP {
					return fmt.Errorf("ip_mismatch: account is bound to %s, current IP is %s",
						bound, clientIP)
				}
			} else {
				bindings[userID] = clientIP
				c.BoundIPs = bindings
				if err := tx.UpdateContest(c); err != nil {
					return err
				}
				ipBound = true
			}
		}

		x := domain.Submission{
			ID: tx.NextID("sub"), ContestID: contestID, ProblemID: problemID,
			UserID: userID, UserName: u.Name, Language: expectedLanguage, Code: code,
			RequestID: requestID,
			Verdict:   domain.VerdictQueued, SubmittedAt: s.clock.Now(),
		}
		if targetProb.TaskType == domain.TaskChoice ||
			expectedLanguage == "choice_json" ||
			(expectedLanguage == "json" && len(targetProb.ChoiceQuestions) > 0) {
			verdict, score, cases, jerr := JudgeChoice(targetProb, code)
			if jerr != nil {
				x.Verdict = domain.VerdictCompileError
				x.CompileMessage = jerr.Error()
			} else {
				x.Verdict = verdict
				x.Score = score
				x.Cases = cases
			}
			x.JudgedAt = s.clock.Now()
			autoJudged = true
		}
		created, err = tx.CreateSubmission(x)
		return err
	})
	if err != nil {
		return domain.Submission{}, false, err
	}
	if replayed {
		return created, true, nil
	}
	if ipBound {
		s.emit("contest.ip_bound", contestID,
			map[string]string{"userId": userID, "ip": clientIP})
	}
	s.emit("submission.created", contestID, created)
	if autoJudged {
		s.emit("submission.judged", contestID, created)
		s.emit("ranking.updated", contestID, s.Ranking(contestID))
	}
	return created, false, nil
}

func (s *Service) Judge(id, verdict string, score int) (domain.Submission, error) {
	x, err := s.store.GetSubmission(id)
	if err != nil {
		return domain.Submission{}, err
	}
	verdict = domain.NormalizeSubmissionVerdict(verdict)
	if !domain.ValidManualVerdict(verdict) {
		return domain.Submission{}, errors.New("invalid verdict")
	}
	p, err := s.GetProblem(x.ContestID, x.ProblemID)
	if err != nil {
		return domain.Submission{}, err
	}
	if score < 0 || score > domain.ProblemTotalScore(p) {
		return domain.Submission{}, fmt.Errorf("score must be between 0 and %d",
			domain.ProblemTotalScore(p))
	}
	x.Verdict, x.Score, x.JudgedAt = verdict, score, s.clock.Now()
	if err := s.store.UpdateSubmission(x); err != nil {
		return domain.Submission{}, err
	}
	s.emit("submission.judged", x.ContestID, x)
	s.emit("ranking.updated", x.ContestID, s.Ranking(x.ContestID))
	return x, nil
}

// MarkSubmissionSystemError closes a submission that was persisted but could
// not be handed to the judge. Only queued work is changed so a late queue
// failure cannot overwrite a result produced by a concurrent worker.
func (s *Service) MarkSubmissionSystemError(id string) (domain.Submission, error) {
	var x domain.Submission
	changed := false
	err := s.withTx(func(tx ports.Store) error {
		current, err := tx.GetSubmission(id)
		if err != nil {
			return err
		}
		x = current
		if current.Verdict != domain.VerdictQueued {
			return nil
		}
		current.Verdict, current.Score, current.Cases = domain.VerdictSystemError, 0, nil
		current.JudgedAt = s.clock.Now()
		x = current
		changed = true
		return tx.UpdateSubmission(current)
	})
	if err != nil {
		return domain.Submission{}, err
	}
	if !changed {
		return x, nil
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
	return s.rankingFor(c)
}

// rankingFor computes or returns the frozen board for a specific contest
// snapshot. It is used both when finishing a contest (inside the transaction)
// and by all public ranking reads.
func (s *Service) rankingFor(c domain.Contest) []domain.RankEntry {
	if c.Status == string(domain.Finished) && c.FinalRanking != nil {
		return c.FinalRanking
	}
	users, _ := s.store.ListUsers()
	subs, _ := s.store.ListSubmissions(c.ID)
	probs, _ := s.store.ListProblems(c.ID)
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

// finalizeRanking computes the closing snapshot from an explicit store facade
// while a transaction holds the memory adapter lock. Calling the public
// Ranking path there would re-enter the locked store and deadlock.
func (s *Service) finalizeRanking(tx ports.Store, c domain.Contest) []domain.RankEntry {
	users, _ := tx.ListUsers()
	subs, _ := tx.ListSubmissions(c.ID)
	probs, _ := tx.ListProblems(c.ID)
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
	if c.Status == string(domain.Finished) && c.FinalACM != nil {
		return c.FinalACM
	}
	users, _ := s.store.ListUsers()
	subs, _ := s.store.ListSubmissions(contestID)
	return domain.ComputeRankingACM(users, c, subs)
}

// finalizeACM computes the closing ACM board against the transaction facade.
func (s *Service) finalizeACM(tx ports.Store, c domain.Contest) []domain.ACMRankEntry {
	users, _ := tx.ListUsers()
	subs, _ := tx.ListSubmissions(c.ID)
	return domain.ComputeRankingACM(users, c, subs)
}

func (s *Service) Health() map[string]any {
	return map[string]any{"status": "ok", "time": time.Now().UTC()}
}
