package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// SupportedLanguage reports whether the judge has a built-in toolchain for the
// requested submission language. Keep this list aligned with
// internal/judge/compiler.go; a contest must never start with a language the
// judge silently turns into compile errors.
func SupportedLanguage(lang string) bool {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "cpp", "c", "c++", "python", "python3", "py", "go", "golang":
		return true
	default:
		return false
	}
}

// NormalizeLanguage returns the canonical language identifier used by the
// judge and by contest allow-lists.
func NormalizeLanguage(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "c++":
		return "cpp"
	case "python3", "py":
		return "python"
	case "golang":
		return "go"
	default:
		return strings.ToLower(strings.TrimSpace(lang))
	}
}

// ValidateContestWindow validates the optional strict exam window.
func ValidateContestWindow(windowEnabled bool, startTime, endTime *time.Time, durationMinutes int, now time.Time) error {
	if durationMinutes < 0 {
		return errors.New("exam duration cannot be negative")
	}
	if startTime != nil && endTime != nil && !startTime.Before(*endTime) {
		return errors.New("exam start time must be before end time")
	}
	if !windowEnabled {
		return nil
	}
	if startTime == nil || endTime == nil {
		return errors.New("strict exam window requires both startTime and endTime")
	}
	if endTime.Sub(*startTime) < time.Minute {
		return errors.New("exam window must be at least one minute long")
	}
	if !now.IsZero() && !now.Before(*endTime) {
		return errors.New("exam end time is already in the past")
	}
	if durationMinutes <= 0 {
		return errors.New("strict exam window requires a positive durationMinutes")
	}
	return nil
}

// ValidateContestStart is the launch preflight. It runs inside the start
// transaction so a contest can never be published with a broken paper or
// dangling participant bindings.
func ValidateContestStart(c Contest, problems []Problem, groups []Group, users []User, now time.Time) error {
	if Status(c.Status) != Draft {
		return fmt.Errorf("cannot start contest from status %s", c.Status)
	}
	if c.RankingMode != "" && c.RankingMode != "oi" && c.RankingMode != "acm" {
		return errors.New("unknown ranking mode")
	}
	if err := ValidateContestWindow(c.WindowEnabled, c.StartTime, c.EndTime, c.DurationMinutes, now); err != nil {
		return err
	}
	if len(c.AllowedLanguages) > 0 {
		seen := map[string]bool{}
		for _, raw := range c.AllowedLanguages {
			lang := NormalizeLanguage(raw)
			if lang == "" {
				return errors.New("allowed language cannot be empty")
			}
			if !SupportedLanguage(lang) {
				return fmt.Errorf("unsupported language in allow-list: %s", raw)
			}
			if seen[lang] {
				return fmt.Errorf("duplicate language in allow-list: %s", raw)
			}
			seen[lang] = true
		}
	}

	groupSet := make(map[string]Group, len(groups))
	for _, g := range groups {
		groupSet[g.ID] = g
	}
	seenGroups := make(map[string]bool, len(c.GroupIDs))
	for _, gid := range c.GroupIDs {
		if seenGroups[gid] {
			return fmt.Errorf("duplicate participant group binding: %s", gid)
		}
		seenGroups[gid] = true
		g, ok := groupSet[gid]
		if !ok {
			return fmt.Errorf("participant group not found: %s", gid)
		}
		if len(g.UserIDs) == 0 {
			return fmt.Errorf("participant group %s has no students", g.Name)
		}
	}
	userSet := make(map[string]User, len(users))
	for _, u := range users {
		userSet[u.ID] = u
	}
	seenStudents := make(map[string]bool)
	for _, gid := range c.GroupIDs {
		g := groupSet[gid]
		for _, uid := range g.UserIDs {
			if seenStudents[uid] {
				continue
			}
			u, ok := userSet[uid]
			if !ok {
				return fmt.Errorf("group %s contains a missing account: %s", g.Name, uid)
			}
			if NormalizeRole(u.Role) != RoleStudent {
				return fmt.Errorf("group %s contains non-student account %s", g.Name, u.Username)
			}
			seenStudents[uid] = true
		}
	}
	seenParticipants := make(map[string]bool, len(c.ParticipantUserIDs))
	for _, uid := range c.ParticipantUserIDs {
		if seenParticipants[uid] {
			return fmt.Errorf("duplicate participant binding: %s", uid)
		}
		seenParticipants[uid] = true
		u, ok := userSet[uid]
		if !ok {
			return fmt.Errorf("participant account not found: %s", uid)
		}
		if NormalizeRole(u.Role) != RoleStudent {
			return fmt.Errorf("participant %s is not a student account", u.Username)
		}
	}

	if len(problems) == 0 {
		return errors.New("contest needs at least one problem before it can start")
	}
	problemByID := make(map[string]Problem, len(problems))
	for _, p := range problems {
		problemByID[p.ID] = p
	}
	if len(c.ProblemIDs) == 0 {
		return errors.New("contest problem order is empty")
	}
	if len(c.ProblemIDs) != len(problems) {
		return fmt.Errorf("contest problem order lists %d problems but storage contains %d", len(c.ProblemIDs), len(problems))
	}
	seenProblemIDs := make(map[string]bool, len(c.ProblemIDs))
	for _, pid := range c.ProblemIDs {
		if seenProblemIDs[pid] {
			return fmt.Errorf("duplicate problem binding: %s", pid)
		}
		seenProblemIDs[pid] = true
		if _, ok := problemByID[pid]; !ok {
			return fmt.Errorf("problem binding not found: %s", pid)
		}
	}
	seenCodes := map[string]string{}
	for _, p := range problems {
		if p.ContestID != c.ID {
			return fmt.Errorf("problem %s belongs to another contest", p.Code)
		}
		if err := ValidateProblem(p); err != nil {
			return fmt.Errorf("problem %s: %w", p.Code, err)
		}
		if len(p.TestCases) > 0 {
			total := 0
			for _, tc := range p.TestCases {
				total += tc.Score
			}
			if total != 100 {
				return fmt.Errorf(
					"problem %s: test case scores total %d, must equal 100",
					p.Code, total,
				)
			}
		}
		code := strings.ToUpper(strings.TrimSpace(p.Code))
		if other, ok := seenCodes[code]; ok {
			return fmt.Errorf("duplicate problem code %s (used by %s)", p.Code, other)
		}
		seenCodes[code] = p.Title
		switch p.TaskType {
		case TaskInteraction:
			if strings.TrimSpace(p.InteractorCode) == "" {
				return fmt.Errorf("problem %s: interaction task requires interactor source", p.Code)
			}
		case TaskAnswersOnly:
			if len(p.TestCases) != 1 {
				return fmt.Errorf("problem %s: answer-only task requires exactly one answer key", p.Code)
			}
			if strings.TrimSpace(p.TestCases[0].OutputFile) == "" {
				return fmt.Errorf("problem %s: answer-only key cannot be empty", p.Code)
			}
		}
		if p.CompareMode == CompareSpecialJudge && strings.TrimSpace(p.CheckerCode) == "" {
			return fmt.Errorf("problem %s: special judge mode requires checker source", p.Code)
		}
	}
	return nil
}
