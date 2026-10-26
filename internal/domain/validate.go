package domain

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ValidateIdempotencyKey limits retry keys to a compact, header-safe value.
// An empty key is allowed for non-browser clients that do not opt into replay
// protection.
func ValidateIdempotencyKey(key string) error {
	if key == "" {
		return nil
	}
	if len(key) < 8 || len(key) > 128 {
		return errors.New("Idempotency-Key must contain 8 to 128 characters")
	}
	for _, r := range key {
		if r < 0x21 || r > 0x7e {
			return errors.New("Idempotency-Key must use visible ASCII characters")
		}
	}
	return nil
}

const (
	maxProblemCodeRunes   = 32
	maxProblemSlugRunes   = 32
	maxProblemTitleRunes  = 300
	maxProblemTextRunes   = 2 << 20
	maxProblemsTestCases  = 2000
	maxChoiceQuestions    = 500
	maxChoiceOptions      = 8
	maxCaseFileRunes      = 256
	maxJudgeSourceBytes   = 2 << 20
	maxTimeLimitMs        = 10 * 60 * 1000
	maxMemoryLimitMiB     = 16 * 1024
	maxOutputLimitKiB     = 1 << 20
	maxCompileTimeoutSec  = 10 * 60
	maxProblemLocales     = 12
	maxLocaleFieldRunes   = 2 << 20
	maxChoiceTextRunes    = 100_000
	maxOneChoiceOptionLen = 20_000
)

// ValidateProblemSlug 校验题面英文名。CSP 复赛要求考生按
// <准考证号>/<英文名>/<英文名>.cpp 组织答案，这个字符串会直接变成
// 目录名与源文件名，因此只允许小写字母开头、后续小写字母/数字/下划线。
func ValidateProblemSlug(slug string) error {
	s := NormalizeSlug(slug)
	if s == "" {
		return errors.New("problem slug is required")
	}
	if len([]rune(s)) > maxProblemSlugRunes {
		return fmt.Errorf("problem slug exceeds %d characters", maxProblemSlugRunes)
	}
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case i > 0 && (r >= '0' && r <= '9' || r == '_'):
		default:
			return fmt.Errorf(
				"problem slug %q must start with a letter and use only lowercase letters, digits or underscore", s)
		}
	}
	return nil
}

// NormalizeSlug 归一化题面英文名为小写无空白。
func NormalizeSlug(slug string) string {
	return strings.ToLower(strings.TrimSpace(slug))
}

// ValidateProblem 强制校验题面必要字段与合理性，与 SPEC 对齐。
func ValidateProblem(p Problem) error {
	if strings.TrimSpace(p.ContestID) == "" {
		return errors.New("contestId is required")
	}
	if strings.TrimSpace(p.Code) == "" {
		return errors.New("problem code/identifier is required")
	}
	if strings.TrimSpace(p.Title) == "" {
		return errors.New("title is required")
	}
	if utf8.RuneCountInString(p.Code) > maxProblemCodeRunes {
		return fmt.Errorf("problem code exceeds %d characters", maxProblemCodeRunes)
	}
	if err := validateTextLength("title", p.Title, maxProblemTitleRunes); err != nil {
		return err
	}
	if p.Slug != "" {
		if err := ValidateProblemSlug(p.Slug); err != nil {
			return err
		}
	}
	for field, value := range map[string]string{
		"statement":   p.Statement,
		"input":       p.Input,
		"output":      p.Output,
		"examples":    p.Examples,
		"constraints": p.Constraints,
	} {
		if err := validateTextLength(field, value, maxProblemTextRunes); err != nil {
			return err
		}
	}
	if strings.TrimSpace(p.Statement) == "" {
		return errors.New("statement is required")
	}
	switch p.TaskType {
	case "", TaskTraditional:
	case TaskChoice, TaskAnswersOnly, TaskInteraction:
	default:
		return fmt.Errorf("unknown task type %q", p.TaskType)
	}
	if p.TaskType == TaskChoice {
		if err := ValidateChoiceQuestions(p.ChoiceQuestions); err != nil {
			return err
		}
	} else {
		if strings.TrimSpace(p.Input) == "" {
			return errors.New("input format is required")
		}
		if strings.TrimSpace(p.Output) == "" {
			return errors.New("output format is required")
		}
		if strings.TrimSpace(p.Constraints) == "" {
			return errors.New("constraints specification is required")
		}
	}
	if p.TimeLimitMs < 0 {
		return errors.New("time limit cannot be negative")
	}
	if p.TimeLimitMs > maxTimeLimitMs {
		return fmt.Errorf("time limit cannot exceed %d ms", maxTimeLimitMs)
	}
	if p.MemoryLimitMiB < 0 {
		return errors.New("memory limit cannot be negative")
	}
	if p.MemoryLimitMiB > maxMemoryLimitMiB {
		return fmt.Errorf("memory limit cannot exceed %d MiB", maxMemoryLimitMiB)
	}
	if p.OutputLimitKiB < 0 || p.OutputLimitKiB > maxOutputLimitKiB {
		return fmt.Errorf("output limit must be between 0 and %d KiB", maxOutputLimitKiB)
	}
	if p.CompileTimeoutSec < 0 || p.CompileTimeoutSec > maxCompileTimeoutSec {
		return fmt.Errorf("compile timeout must be between 0 and %d seconds", maxCompileTimeoutSec)
	}
	if p.CompileStd != "" && !validCompileStd(p.CompileStd) {
		return errors.New("compileStd must be a plain token like c++17 or c11")
	}
	if p.TimeLimitRatio < 0 || p.TimeLimitRatio > 100 {
		return errors.New("time limit ratio must be between 0 and 100")
	}
	if p.MemoryLimitRatio < 0 || p.MemoryLimitRatio > 100 {
		return errors.New("memory limit ratio must be between 0 and 100")
	}
	if p.CompareMode != "" {
		switch NormalizeComparisonMode(p.CompareMode) {
		case CompareLineByLine, CompareIgnoreSpace, CompareRealNumber, CompareSpecialJudge,
			CompareLemonSpecialJudge, CompareTestlibSpecialJudge, CompareExternalTool:
		default:
			return errors.New("unknown comparison mode")
		}
	}
	if NormalizeComparisonMode(p.CompareMode) == CompareExternalTool {
		if strings.TrimSpace(p.DiffCommand) == "" {
			return errors.New("external tool compare requires diffCommand")
		}
		if len(p.DiffCommand) > 256 {
			return errors.New("diffCommand is too long")
		}
		if len(p.DiffArgs) > 16 {
			return errors.New("diffArgs accepts at most 16 arguments")
		}
		for _, arg := range p.DiffArgs {
			if len(arg) > 256 {
				return errors.New("diff argument is too long")
			}
		}
	}
	if p.IOMode == IOFile {
		if strings.TrimSpace(p.InFile) == "" || strings.TrimSpace(p.OutFile) == "" {
			return errors.New("file IO mode requires both inFile and outFile")
		}
		if strings.Contains(p.InFile, "..") || strings.Contains(p.OutFile, "..") {
			return errors.New("file IO path cannot contain traversal elements (..)")
		}
	}
	if p.CheckerCode != "" && p.CheckerLang != "" && !SupportedLanguage(p.CheckerLang) {
		return fmt.Errorf("unsupported checker language %q", p.CheckerLang)
	}
	if len(p.CheckerCode) > maxJudgeSourceBytes {
		return fmt.Errorf("checker source exceeds %d bytes", maxJudgeSourceBytes)
	}
	if len(p.InteractorCode) > maxJudgeSourceBytes {
		return fmt.Errorf("interactor source exceeds %d bytes", maxJudgeSourceBytes)
	}
	if p.CompareMode == CompareSpecialJudge && strings.TrimSpace(p.CheckerCode) == "" {
		return errors.New("special judge mode requires checker source")
	}
	if p.TaskType == TaskInteraction {
		if strings.TrimSpace(p.InteractorCode) == "" {
			return errors.New("interaction task requires interactor source")
		}
		if p.InteractorLang != "" && !SupportedLanguage(p.InteractorLang) {
			return fmt.Errorf("unsupported interactor language %q", p.InteractorLang)
		}
	}
	if p.TaskType == TaskAnswersOnly && len(p.TestCases) > 0 {
		if len(p.TestCases) != 1 {
			return errors.New("answer-only task requires exactly one answer key")
		}
		if strings.TrimSpace(p.TestCases[0].OutputFile) == "" {
			return errors.New("answer-only key cannot be empty")
		}
	}
	if err := ValidateSubtaskDependencies(p.TestCases); err != nil {
		return err
	}
	if len(p.TestCases) > maxProblemsTestCases {
		return fmt.Errorf("problem exceeds %d test cases", maxProblemsTestCases)
	}
	for i, tc := range p.TestCases {
		if tc.Score < 0 {
			return errors.New("testcase score cannot be negative")
		}
		if tc.Score > 100_000 {
			return errors.New("testcase score is unreasonably large")
		}
		if utf8.RuneCountInString(tc.ID) > maxCaseFileRunes ||
			utf8.RuneCountInString(tc.InputFile) > maxCaseFileRunes ||
			utf8.RuneCountInString(tc.OutputFile) > maxCaseFileRunes {
			return fmt.Errorf("testcase %d file reference is too long", i+1)
		}
		if tc.TimeLimitMs < 0 || tc.TimeLimitMs > maxTimeLimitMs {
			return fmt.Errorf("testcase %d time limit is out of range", i+1)
		}
		if tc.MemoryMiB < 0 || tc.MemoryMiB > maxMemoryLimitMiB {
			return fmt.Errorf("testcase %d memory limit is out of range", i+1)
		}
		_ = i
	}
	if len(p.Locales) > maxProblemLocales {
		return fmt.Errorf("problem exceeds %d locales", maxProblemLocales)
	}
	for locale, text := range p.Locales {
		if strings.TrimSpace(locale) == "" || len(locale) > 32 {
			return errors.New("invalid locale key")
		}
		for field, value := range map[string]string{
			"title":       text.Title,
			"statement":   text.Statement,
			"constraints": text.Constraints,
			"input":       text.Input,
			"output":      text.Output,
			"examples":    text.Examples,
		} {
			if err := validateTextLength(locale+"."+field, value, maxLocaleFieldRunes); err != nil {
				return err
			}
		}
	}
	return nil
}

// validCompileStd 校验题目级 -std 覆盖：只允许单 token（字母数字与少量
// 安全符号），作为单个 argv 元素传给编译器，杜绝参数注入。
func validCompileStd(std string) bool {
	if len(std) == 0 || len(std) > 32 {
		return false
	}
	for _, r := range std {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '+' || r == '-' || r == '=' || r == '.' || r == '_':
		default:
			return false
		}
	}
	return true
}

func validateTextLength(field, value string, limit int) error {
	if utf8.RuneCountInString(value) > limit {
		return fmt.Errorf("%s exceeds %d characters", field, limit)
	}
	return nil
}

// ValidateSubtaskDependencies rejects dangling edges and cycles in the
// subtask DAG. The judge relies on this acyclic graph to skip unsolved
// prerequisite groups without repeatedly re-evaluating a cycle.
func ValidateSubtaskDependencies(testCases []TestCase) error {
	if len(testCases) == 0 {
		return nil
	}
	exists := map[int]bool{}
	deps := map[int][]int{}
	for _, tc := range testCases {
		exists[tc.Subtask] = true
		if tc.DependsOn != nil {
			deps[tc.Subtask] = append([]int(nil), tc.DependsOn...)
		}
	}
	for subtask, list := range deps {
		seen := map[int]bool{}
		for _, dep := range list {
			if dep == subtask {
				return errors.New("subtask cannot depend on itself")
			}
			if !exists[dep] {
				return fmt.Errorf("subtask %d depends on missing subtask %d", subtask, dep)
			}
			if seen[dep] {
				return fmt.Errorf("subtask %d lists duplicate dependency %d", subtask, dep)
			}
			seen[dep] = true
		}
	}

	const (
		unseen = iota
		visiting
		done
	)
	state := map[int]int{}
	var visit func(int) error
	visit = func(subtask int) error {
		switch state[subtask] {
		case visiting:
			return fmt.Errorf("subtask dependency cycle includes subtask %d", subtask)
		case done:
			return nil
		}
		state[subtask] = visiting
		for _, dep := range deps[subtask] {
			if err := visit(dep); err != nil {
				return err
			}
		}
		state[subtask] = done
		return nil
	}
	for subtask := range exists {
		if err := visit(subtask); err != nil {
			return err
		}
	}
	return nil
}

// ValidateChoiceQuestions enforces a complete, gradeable objective paper.
// Server-side redaction replaces answers with nil for students, so this
// validator only runs on the teacher write path.
func ValidateChoiceQuestions(questions []ChoiceQuestion) error {
	if len(questions) == 0 {
		return errors.New("choice question paper requires at least one question")
	}
	if len(questions) > maxChoiceQuestions {
		return fmt.Errorf("choice paper exceeds %d questions", maxChoiceQuestions)
	}
	seen := make(map[int]bool, len(questions))
	for i, q := range questions {
		if q.ID <= 0 {
			return fmt.Errorf("choice question %d: id must be positive", i+1)
		}
		if seen[q.ID] {
			return fmt.Errorf("choice question %d: duplicate id %d", i+1, q.ID)
		}
		seen[q.ID] = true
		if strings.TrimSpace(q.Stem) == "" {
			return fmt.Errorf("choice question %d: stem is required", q.ID)
		}
		if err := validateTextLength(
			fmt.Sprintf("choice question %d stem", q.ID), q.Stem, maxChoiceTextRunes,
		); err != nil {
			return err
		}
		if err := validateTextLength(
			fmt.Sprintf("choice question %d explanation", q.ID),
			q.Explanation, maxChoiceTextRunes,
		); err != nil {
			return err
		}
		if q.Score <= 0 {
			return fmt.Errorf("choice question %d: score must be positive", q.ID)
		}

		switch q.Type {
		case "single", "multiple":
			if len(q.Options) < 2 {
				return fmt.Errorf("choice question %d: at least two options are required", q.ID)
			}
			if len(q.Options) > maxChoiceOptions {
				return fmt.Errorf("choice question %d: at most %d options are allowed",
					q.ID, maxChoiceOptions)
			}
			valid := make(map[string]bool, len(q.Options))
			for j, option := range q.Options {
				if strings.TrimSpace(option) == "" {
					return fmt.Errorf("choice question %d: option %d is empty", q.ID, j+1)
				}
				if err := validateTextLength(
					fmt.Sprintf("choice question %d option %d", q.ID, j+1),
					option, maxOneChoiceOptionLen,
				); err != nil {
					return err
				}
				valid[choiceOptionKey(option, j)] = true
			}
			answers := choiceAnswerTokens(q.Answer)
			if len(answers) == 0 {
				return fmt.Errorf("choice question %d: answer is required", q.ID)
			}
			if q.Type == "single" && len(answers) != 1 {
				return fmt.Errorf("choice question %d: single choice requires exactly one answer", q.ID)
			}
			if q.Type == "multiple" {
				answers = expandCompactChoiceAnswers(answers)
			}
			for _, answer := range answers {
				if !valid[answer] {
					return fmt.Errorf(
						"choice question %d: answer %q does not match any option",
						q.ID, answer,
					)
				}
			}
		case "judge":
			answers := choiceAnswerTokens(q.Answer)
			if len(answers) != 1 || (answers[0] != "T" && answers[0] != "F") {
				return fmt.Errorf("choice question %d: judge answer must be T or F", q.ID)
			}
		default:
			return fmt.Errorf("choice question %d: unknown type %q", q.ID, q.Type)
		}
	}
	return nil
}

func choiceOptionKey(option string, index int) string {
	trimmed := strings.TrimSpace(option)
	if trimmed != "" {
		first, size := utf8.DecodeRuneInString(trimmed)
		key := unicode.ToUpper(first)
		if key >= 'A' && key <= 'H' {
			if size == len(trimmed) {
				return string(key)
			}
			separator, _ := utf8.DecodeRuneInString(trimmed[size:])
			switch separator {
			case '.', '、', ':', '：', ')', '．':
				return string(key)
			}
		}
	}
	if index >= 0 && index < 8 {
		return string(rune('A' + index))
	}
	return ""
}

func choiceAnswerTokens(value any) []string {
	var raw []string
	switch v := value.(type) {
	case string:
		raw = []string{v}
	case []string:
		raw = append(raw, v...)
	case []any:
		for _, item := range v {
			raw = append(raw, fmt.Sprint(item))
		}
	default:
		return nil
	}

	var out []string
	for _, item := range raw {
		items := strings.FieldsFunc(strings.ToUpper(strings.TrimSpace(item)), func(r rune) bool {
			return r == ',' || r == '，' || r == ';' || r == '；' || r == ' ' || r == '\t'
		})
		for _, token := range items {
			switch token {
			case "TRUE", "YES", "1", "对", "正确", "V", "√":
				token = "T"
			case "FALSE", "NO", "0", "错", "错误", "X", "×":
				token = "F"
			}
			if token != "" {
				out = append(out, token)
			}
		}
	}
	return out
}

// compactChoiceAnswers accepts the common "AC" shorthand for multi-select
// answers and expands it to option keys before validation.
func expandCompactChoiceAnswers(tokens []string) []string {
	var out []string
	for _, token := range tokens {
		if len(token) <= 1 {
			out = append(out, token)
			continue
		}
		compact := true
		for _, r := range token {
			if r < 'A' || r > 'H' {
				compact = false
				break
			}
		}
		if !compact {
			out = append(out, token)
			continue
		}
		for _, r := range token {
			out = append(out, string(r))
		}
	}
	sort.Strings(out)
	return out
}

// IsUserAllowed 参赛准入：无绑定名单时公开；否则个人名单或组交集命中其一即可。
func IsUserAllowed(c Contest, u User) bool {
	if len(c.GroupIDs) == 0 && len(c.ParticipantUserIDs) == 0 {
		return true
	}
	for _, id := range c.ParticipantUserIDs {
		if id == u.ID {
			return true
		}
	}
	inGroup := map[string]struct{}{}
	for _, g := range u.Groups {
		inGroup[g] = struct{}{}
	}
	for _, g := range c.GroupIDs {
		if _, ok := inGroup[g]; ok {
			return true
		}
	}
	return false
}

// IsExplicitlyInvited reports whether the user is named directly or through a
// contest group. Unlike IsUserAllowed, an open contest does not count.
func IsExplicitlyInvited(c Contest, u User) bool {
	for _, id := range c.ParticipantUserIDs {
		if id == u.ID {
			return true
		}
	}
	for _, gid := range c.GroupIDs {
		for _, ug := range u.Groups {
			if ug == gid {
				return true
			}
		}
	}
	return false
}

// CanAccessContest applies launch access policy. Staff can access all
// contests; students can access only contests they are eligible for. Drafts
// are hidden from students unless they are explicitly invited.
func CanAccessContest(c Contest, u User, staff bool) bool {
	if staff {
		return true
	}
	if Status(c.Status) == Draft {
		return IsExplicitlyInvited(c, u)
	}
	return IsUserAllowed(c, u)
}

// NormalizeRole 非法角色回落为 student，保证入库值收敛。
func NormalizeRole(role string) string {
	switch role {
	case RoleAdmin, RoleTeacher, RoleStudent:
		return role
	default:
		return RoleStudent
	}
}
