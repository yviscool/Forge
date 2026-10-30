package domain

import "testing"

func TestValidateProblem(t *testing.T) {
	if err := ValidateProblem(Problem{ContestID: "c", Code: "A", Title: "T", Statement: "s", Input: "i", Output: "o", Constraints: "k"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateProblem(Problem{Code: "A"}); err == nil {
		t.Fatal("expected validation error")
	}
	if err := ValidateProblem(Problem{
		ContestID: "c", Code: "A", Title: "T", Statement: "s", Input: "i",
		Output: "o", Constraints: "k", TaskType: "essay",
	}); err == nil {
		t.Fatal("unknown task type must fail closed")
	}
	if err := ValidateProblem(Problem{
		ContestID: "c", Code: "A", Title: "T", Statement: "s", Input: "i",
		Output: "o", Constraints: "k", CheckerCode: "x", CheckerLang: "brainfuck",
	}); err == nil {
		t.Fatal("unsupported checker language must fail closed")
	}
	// 题目级高级编译参数校验测试
	validOpt := Problem{
		ContestID: "c", Code: "A", Title: "T", Statement: "s", Input: "i",
		Output: "o", Constraints: "k", CompileStd: "c++17", CompileOpt: "O3",
		CompileWarn: "extra", CompileFlags: []string{"-DLOCAL_JUDGE", "-fno-ms-extensions"},
	}
	if err := ValidateProblem(validOpt); err != nil {
		t.Fatalf("valid compile options rejected: %v", err)
	}
	invalidOpt := validOpt
	invalidOpt.CompileOpt = "O999"
	if err := ValidateProblem(invalidOpt); err == nil {
		t.Fatal("invalid compileOpt should be rejected")
	}
	invalidWarn := validOpt
	invalidWarn.CompileWarn = "unknown"
	if err := ValidateProblem(invalidWarn); err == nil {
		t.Fatal("invalid compileWarn should be rejected")
	}
	invalidFlags := validOpt
	invalidFlags.CompileFlags = []string{"-DTEST;rm -rf /"}
	if err := ValidateProblem(invalidFlags); err == nil {
		t.Fatal("malicious compileFlags should be rejected")
	}
}

func TestValidateSubtaskDependencies(t *testing.T) {
	valid := []TestCase{
		{ID: "a", Subtask: 1},
		{ID: "b", Subtask: 2, DependsOn: []int{1}},
	}
	if err := ValidateSubtaskDependencies(valid); err != nil {
		t.Fatal(err)
	}
	cases := map[string][]TestCase{
		"self": {
			{ID: "a", Subtask: 1, DependsOn: []int{1}},
		},
		"dangling": {
			{ID: "a", Subtask: 1, DependsOn: []int{2}},
		},
		"duplicate": {
			{ID: "a", Subtask: 1},
			{ID: "b", Subtask: 2, DependsOn: []int{1, 1}},
		},
		"cycle": {
			{ID: "a", Subtask: 1, DependsOn: []int{2}},
			{ID: "b", Subtask: 2, DependsOn: []int{1}},
		},
	}
	for name, testCases := range cases {
		t.Run(name, func(t *testing.T) {
			if err := ValidateSubtaskDependencies(testCases); err == nil {
				t.Fatal("invalid subtask graph accepted")
			}
		})
	}
}

func TestValidateChoiceQuestions(t *testing.T) {
	valid := []ChoiceQuestion{
		{
			ID: 1, Type: "single", Stem: "single",
			Options: []string{"A. alpha", "B. beta"}, Answer: "A", Score: 2,
		},
		{
			ID: 2, Type: "multiple", Stem: "multiple",
			Options: []string{"A. one", "B. two", "C. three", "D. four"},
			Answer:  "AC", Score: 4,
		},
		{
			ID: 3, Type: "judge", Stem: "judge", Answer: "T", Score: 2,
		},
	}
	if err := ValidateChoiceQuestions(valid); err != nil {
		t.Fatalf("valid paper rejected: %v", err)
	}

	cases := []struct {
		name string
		edit func(*[]ChoiceQuestion)
	}{
		{"empty paper", func(q *[]ChoiceQuestion) { *q = nil }},
		{"duplicate id", func(q *[]ChoiceQuestion) { (*q)[1].ID = 1 }},
		{"empty stem", func(q *[]ChoiceQuestion) { (*q)[0].Stem = " " }},
		{"non-positive score", func(q *[]ChoiceQuestion) { (*q)[0].Score = 0 }},
		{"too few options", func(q *[]ChoiceQuestion) { (*q)[0].Options = []string{"A. only"} }},
		{"unknown option", func(q *[]ChoiceQuestion) { (*q)[0].Answer = "Z" }},
		{"multiple unknown option", func(q *[]ChoiceQuestion) { (*q)[1].Answer = "AE" }},
		{"invalid judge answer", func(q *[]ChoiceQuestion) { (*q)[2].Answer = "yes-please" }},
		{"unknown question type", func(q *[]ChoiceQuestion) { (*q)[0].Type = "essay" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			paper := make([]ChoiceQuestion, len(valid))
			for i := range valid {
				paper[i] = valid[i]
				paper[i].Options = append([]string(nil), valid[i].Options...)
			}
			tc.edit(&paper)
			if err := ValidateChoiceQuestions(paper); err == nil {
				t.Fatal("invalid paper accepted")
			}
		})
	}
}

func TestIsUserAllowed(t *testing.T) {
	open := Contest{ID: "c"}
	if !IsUserAllowed(open, User{ID: "u"}) {
		t.Fatal("open contest should allow all")
	}
	restricted := Contest{ID: "c", ParticipantUserIDs: []string{"u1"}, GroupIDs: []string{"g1"}}
	if !IsUserAllowed(restricted, User{ID: "u1"}) {
		t.Fatal("direct participant should pass")
	}
	if !IsUserAllowed(restricted, User{ID: "u2", Groups: []string{"g1"}}) {
		t.Fatal("group member should pass")
	}
	if IsUserAllowed(restricted, User{ID: "u3"}) {
		t.Fatal("outsider should be rejected")
	}
}

func TestComputeRanking(t *testing.T) {
	c := Contest{ID: "c"}
	users := []User{{ID: "u1", Name: "Bob"}, {ID: "u2", Name: "Ada"}}
	subs := []Submission{
		{ContestID: "c", ProblemID: "p1", UserID: "u1", UserName: "Bob", Score: 60},
		{ContestID: "c", ProblemID: "p1", UserID: "u1", UserName: "Bob", Score: 100},
		{ContestID: "c", ProblemID: "p1", UserID: "u2", UserName: "Ada", Score: 100},
	}
	r := ComputeRanking(users, c, subs)
	if len(r) != 2 || r[0].UserName != "Ada" {
		t.Fatalf("deterministic order broken: %#v", r)
	}
}

func TestComputeRankingExcludesStaffAndDeletedProblems(t *testing.T) {
	c := Contest{ID: "c", ParticipantUserIDs: []string{"u1", "teacher"}}
	users := []User{
		{ID: "u1", Name: "Ada", Role: RoleStudent},
		{ID: "teacher", Name: "Prof", Role: RoleTeacher},
		{ID: "admin", Name: "Admin", Role: RoleAdmin},
	}
	subs := []Submission{
		{ContestID: "c", ProblemID: "p1", UserID: "u1", Score: 80},
		{ContestID: "c", ProblemID: "gone", UserID: "u1", Score: 100},
		{ContestID: "c", ProblemID: "p1", UserID: "teacher", Score: 100},
	}
	r := ComputeRanking(users, c, subs, map[string]int{"p1": 100})
	if len(r) != 1 || r[0].UserID != "u1" || r[0].Score != 80 || r[0].Accepted != 0 {
		t.Fatalf("staff/deleted problem must not shape the board: %+v", r)
	}
}
