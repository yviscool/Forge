package domain

import (
	"strings"
	"testing"
	"time"
)

func validStartContest() Contest {
	return Contest{
		ID: "c1", Name: "Launch", Status: string(Draft), RankingMode: "oi",
		ProblemIDs: []string{"p1"},
	}
}

func validStartProblem(contestID string) Problem {
	return Problem{
		ID: "p1", ContestID: contestID, Code: "A", Title: "Sum", Statement: "s",
		Input: "i", Output: "o", Constraints: "k",
	}
}

func TestValidateContestStartAcceptsCompletePaper(t *testing.T) {
	now := time.Now().UTC()
	if err := ValidateContestStart(
		validStartContest(),
		[]Problem{validStartProblem("c1")},
		nil, nil, now,
	); err != nil {
		t.Fatalf("complete draft should pass preflight: %v", err)
	}
}

func TestValidateContestStartRejectsEmptyPaper(t *testing.T) {
	err := ValidateContestStart(validStartContest(), nil, nil, nil, time.Now().UTC())
	if err == nil || !strings.Contains(err.Error(), "at least one problem") {
		t.Fatalf("empty paper must be rejected, got %v", err)
	}
}

func TestValidateContestStartRejectsBrokenProblem(t *testing.T) {
	p := validStartProblem("c1")
	p.Statement = ""
	err := ValidateContestStart(
		validStartContest(),
		[]Problem{p},
		nil, nil, time.Now().UTC(),
	)
	if err == nil || !strings.Contains(err.Error(), "statement is required") {
		t.Fatalf("invalid problem must be rejected, got %v", err)
	}
}

func TestValidateContestStartRejectsDuplicateProblemCodes(t *testing.T) {
	c := validStartContest()
	c.ProblemIDs = []string{"p1", "p2"}
	second := validStartProblem("c1")
	second.ID = "p2"
	err := ValidateContestStart(
		c,
		[]Problem{validStartProblem("c1"), second},
		nil, nil, time.Now().UTC(),
	)
	if err == nil || !strings.Contains(err.Error(), "duplicate problem code") {
		t.Fatalf("duplicate codes must be rejected, got %v", err)
	}
}

func TestValidateContestStartRequiresTestCasesToTotalOneHundred(t *testing.T) {
	p := validStartProblem("c1")
	p.TestCases = []TestCase{
		{ID: "a", InputFile: "a.in", OutputFile: "a.out", Score: 40},
		{ID: "b", InputFile: "b.in", OutputFile: "b.out", Score: 50},
	}
	err := ValidateContestStart(
		validStartContest(), []Problem{p}, nil, nil, time.Now().UTC(),
	)
	if err == nil || !strings.Contains(err.Error(), "must equal 100") {
		t.Fatalf("broken score total must be rejected, got %v", err)
	}

	p.TestCases[1].Score = 60
	if err := ValidateContestStart(
		validStartContest(), []Problem{p}, nil, nil, time.Now().UTC(),
	); err != nil {
		t.Fatalf("score total 100 should pass preflight: %v", err)
	}
}

func TestValidateContestStartRejectsUnsupportedLanguage(t *testing.T) {
	c := validStartContest()
	c.AllowedLanguages = []string{"cpp", "brainfuck"}
	err := ValidateContestStart(
		c, []Problem{validStartProblem("c1")}, nil, nil, time.Now().UTC(),
	)
	if err == nil || !strings.Contains(err.Error(), "unsupported language") {
		t.Fatalf("unsupported language must be rejected, got %v", err)
	}
}

func TestValidateContestStartNormalizesDuplicateLanguages(t *testing.T) {
	c := validStartContest()
	c.AllowedLanguages = []string{"cpp", "c++"}
	err := ValidateContestStart(
		c, []Problem{validStartProblem("c1")}, nil, nil, time.Now().UTC(),
	)
	if err == nil || !strings.Contains(err.Error(), "duplicate language") {
		t.Fatalf("normalized duplicate language must be rejected, got %v", err)
	}
}

func TestValidateContestStartRejectsDanglingBindings(t *testing.T) {
	c := validStartContest()
	c.GroupIDs = []string{"missing"}
	err := ValidateContestStart(
		c, []Problem{validStartProblem("c1")}, nil, nil, time.Now().UTC(),
	)
	if err == nil || !strings.Contains(err.Error(), "participant group not found") {
		t.Fatalf("missing group must be rejected, got %v", err)
	}

	c = validStartContest()
	c.ParticipantUserIDs = []string{"teacher-1"}
	err = ValidateContestStart(
		c, []Problem{validStartProblem("c1")}, nil,
		[]User{{ID: "teacher-1", Username: "prof", Role: RoleTeacher}},
		time.Now().UTC(),
	)
	if err == nil || !strings.Contains(err.Error(), "not a student") {
		t.Fatalf("staff participant must be rejected, got %v", err)
	}
}

func TestValidateContestStartRejectsBrokenGroupMembershipAndDuplicates(t *testing.T) {
	student := User{ID: "s1", Username: "student", Role: RoleStudent}
	teacher := User{ID: "t1", Username: "teacher", Role: RoleTeacher}
	tests := []struct {
		name    string
		contest Contest
		groups  []Group
		users   []User
		want    string
	}{
		{
			name: "missing group member",
			contest: func() Contest {
				c := validStartContest()
				c.GroupIDs = []string{"g1"}
				return c
			}(),
			groups: []Group{{ID: "g1", Name: "Class", UserIDs: []string{"missing"}}},
			users:  []User{student},
			want:   "missing account",
		},
		{
			name: "non-student group member",
			contest: func() Contest {
				c := validStartContest()
				c.GroupIDs = []string{"g1"}
				return c
			}(),
			groups: []Group{{ID: "g1", Name: "Class", UserIDs: []string{teacher.ID}}},
			users:  []User{teacher},
			want:   "non-student account",
		},
		{
			name: "duplicate group binding",
			contest: func() Contest {
				c := validStartContest()
				c.GroupIDs = []string{"g1", "g1"}
				return c
			}(),
			groups: []Group{{ID: "g1", Name: "Class", UserIDs: []string{student.ID}}},
			users:  []User{student},
			want:   "duplicate participant group",
		},
		{
			name: "duplicate participant binding",
			contest: func() Contest {
				c := validStartContest()
				c.ParticipantUserIDs = []string{student.ID, student.ID}
				return c
			}(),
			users: []User{student},
			want:  "duplicate participant",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateContestStart(
				tt.contest, []Problem{validStartProblem(tt.contest.ID)}, tt.groups, tt.users, time.Now().UTC(),
			)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want error containing %q", err, tt.want)
			}
		})
	}
}

func TestValidateContestStartRejectsInconsistentProblemOrder(t *testing.T) {
	c := validStartContest()
	c.ProblemIDs = nil
	err := ValidateContestStart(c, []Problem{validStartProblem(c.ID)}, nil, nil, time.Now().UTC())
	if err == nil || !strings.Contains(err.Error(), "problem order is empty") {
		t.Fatalf("empty problem order must be rejected, got %v", err)
	}

	c.ProblemIDs = []string{"p1", "p2"}
	err = ValidateContestStart(c, []Problem{validStartProblem(c.ID)}, nil, nil, time.Now().UTC())
	if err == nil || !strings.Contains(err.Error(), "storage contains") {
		t.Fatalf("problem count mismatch must be rejected, got %v", err)
	}

	c.ProblemIDs = []string{"p1", "missing"}
	other := validStartProblem(c.ID)
	other.ID = "p2"
	other.Code = "B"
	err = ValidateContestStart(c, []Problem{validStartProblem(c.ID), other}, nil, nil, time.Now().UTC())
	if err == nil || !strings.Contains(err.Error(), "problem binding not found") {
		t.Fatalf("dangling problem binding must be rejected, got %v", err)
	}
}

func TestValidateContestStartRequiresTaskSpecificJudgeSource(t *testing.T) {
	interaction := validStartProblem("c1")
	interaction.TaskType = TaskInteraction
	err := ValidateContestStart(
		validStartContest(), []Problem{interaction}, nil, nil, time.Now().UTC(),
	)
	if err == nil || !strings.Contains(err.Error(), "requires interactor source") {
		t.Fatalf("interaction without interactor must be rejected, got %v", err)
	}

	answers := validStartProblem("c1")
	answers.TaskType = TaskAnswersOnly
	err = ValidateContestStart(
		validStartContest(), []Problem{answers}, nil, nil, time.Now().UTC(),
	)
	if err == nil || !strings.Contains(err.Error(), "requires exactly one answer key") {
		t.Fatalf("answer-only paper without keys must be rejected, got %v", err)
	}
	answers.TestCases = []TestCase{
		{ID: "a", OutputFile: "42", Score: 50},
		{ID: "b", OutputFile: "43", Score: 50},
	}
	err = ValidateContestStart(
		validStartContest(), []Problem{answers}, nil, nil, time.Now().UTC(),
	)
	if err == nil || !strings.Contains(err.Error(), "exactly one answer key") {
		t.Fatalf("answer-only paper with ambiguous keys must be rejected, got %v", err)
	}

	special := validStartProblem("c1")
	special.CompareMode = CompareSpecialJudge
	err = ValidateContestStart(
		validStartContest(), []Problem{special}, nil, nil, time.Now().UTC(),
	)
	if err == nil || !strings.Contains(err.Error(), "requires checker source") {
		t.Fatalf("special judge without checker must be rejected, got %v", err)
	}
}

func TestValidateContestStartWindow(t *testing.T) {
	now := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	start := now.Add(time.Hour)
	end := start.Add(2 * time.Hour)
	c := validStartContest()
	c.WindowEnabled = true
	c.StartTime = &start
	c.EndTime = &end
	c.DurationMinutes = 120
	if err := ValidateContestStart(
		c, []Problem{validStartProblem("c1")}, nil, nil, now,
	); err != nil {
		t.Fatalf("valid window should pass: %v", err)
	}

	bad := c
	bad.DurationMinutes = 0
	if err := ValidateContestStart(
		bad, []Problem{validStartProblem("c1")}, nil, nil, now,
	); err == nil || !strings.Contains(err.Error(), "durationMinutes") {
		t.Fatalf("zero duration must be rejected, got %v", err)
	}

	bad = c
	bad.EndTime = &start
	bad.StartTime = &end
	if err := ValidateContestStart(
		bad, []Problem{validStartProblem("c1")}, nil, nil, now,
	); err == nil || !strings.Contains(err.Error(), "before end time") {
		t.Fatalf("reversed window must be rejected, got %v", err)
	}
}
