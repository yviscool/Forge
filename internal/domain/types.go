package domain

import "time"

// Status 比赛生命周期。
type Status string

const (
	Draft    Status = "draft"
	Running  Status = "running"
	Finished Status = "finished"
)

// Roles 角色常量。
const (
	RoleAdmin   = "admin"
	RoleTeacher = "teacher"
	RoleStudent = "student"
)

// Verdicts 判题结果规范值。
const (
	VerdictQueued       = "queued"
	VerdictJudging      = "judging"
	VerdictAccepted     = "accepted"
	VerdictWrongAnswer  = "wrong_answer"
	VerdictTimeLimit    = "time_limit"
	VerdictRuntimeError = "runtime_error"
	VerdictCompileError = "compile_error"
)

type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	Groups    []string  `json:"groups"`
	CreatedAt time.Time `json:"createdAt"`
}

type Group struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	UserIDs     []string  `json:"userIds"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Contest struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Description        string    `json:"description"`
	Status             string    `json:"status"`
	RankingMode        string    `json:"rankingMode,omitempty"`
	ProblemIDs         []string  `json:"problemIds"`
	GroupIDs           []string  `json:"groupIds"`
	ParticipantUserIDs []string  `json:"participantUserIds"`
	CreatedAt          time.Time `json:"createdAt"`
	StartedAt          time.Time `json:"startedAt,omitempty"`
	FinishedAt         time.Time `json:"finishedAt,omitempty"`
}

type ProblemLocale struct {
	Title       string `json:"title"`
	Statement   string `json:"statement"`
	Constraints string `json:"constraints"`
	Input       string `json:"input"`
	Output      string `json:"output"`
	Examples    string `json:"examples"`
}

type TestCase struct {
	ID          string `json:"id"`
	InputFile   string `json:"inputFile"`
	OutputFile  string `json:"outputFile"`
	Score       int    `json:"score"`
	Subtask     int    `json:"subtask"`
	TimeLimitMs int    `json:"timeLimitMs,omitempty"`
	MemoryMiB   int    `json:"memoryMib,omitempty"`
}

// TaskType 题目类型（对标 LemonLime TaskType 子集）。
type TaskType string

const (
	TaskTraditional TaskType = "traditional"
	TaskAnswersOnly TaskType = "answers_only"
	TaskInteraction TaskType = "interaction"
)

// IOMode 程序 IO 方式。
type IOMode string

const (
	IOStdio IOMode = "stdio"
	IOFile  IOMode = "file"
)

type Problem struct {
	ID             string                   `json:"id"`
	ContestID      string                   `json:"contestId"`
	Code           string                   `json:"code"`
	Title          string                   `json:"title"`
	Statement      string                   `json:"statement"`
	Constraints    string                   `json:"constraints"`
	Input          string                   `json:"input"`
	Output         string                   `json:"output"`
	Examples       string                   `json:"examples"`
	Locale         string                   `json:"locale"`
	TimeLimitMs    int                      `json:"timeLimitMs"`
	MemoryLimitMiB int                      `json:"memoryLimitMib"`
	CompareMode    ComparisonMode           `json:"compareMode,omitempty"`
	RealEps        float64                  `json:"realEps,omitempty"`
	TaskType       TaskType                 `json:"taskType,omitempty"`
	IOMode         IOMode                   `json:"ioMode,omitempty"`
	InFile         string                   `json:"inFile,omitempty"`
	OutFile        string                   `json:"outFile,omitempty"`
	CheckerLang    string                   `json:"checkerLang,omitempty"`
	CheckerCode    string                   `json:"checkerCode,omitempty"`
	InteractorLang string                   `json:"interactorLang,omitempty"`
	InteractorCode string                   `json:"interactorCode,omitempty"`
	Locales        map[string]ProblemLocale `json:"locales,omitempty"`
	TestCases      []TestCase               `json:"testCases,omitempty"`
	UpdatedAt      time.Time                `json:"updatedAt"`
}

type Submission struct {
	ID          string       `json:"id"`
	ContestID   string       `json:"contestId"`
	ProblemID   string       `json:"problemID"`
	UserID      string       `json:"userId"`
	UserName    string       `json:"userName,omitempty"`
	Language    string       `json:"language"`
	Code        string       `json:"code"`
	Verdict     string       `json:"verdict"`
	Score       int          `json:"score"`
	Cases       []CaseResult `json:"cases,omitempty"`
	SubmittedAt time.Time    `json:"submittedAt"`
	JudgedAt    time.Time    `json:"judgedAt,omitempty"`
}

type RankEntry struct {
	UserID        string         `json:"userId"`
	UserName      string         `json:"userName"`
	Score         int            `json:"score"`
	Accepted      int            `json:"accepted"`
	ProblemScores map[string]int `json:"problemScores"`
}

type Event struct {
	Type      string    `json:"type"`
	ContestID string    `json:"contestId,omitempty"`
	Data      any       `json:"data"`
	At        time.Time `json:"at"`
}

// Session 登录会话（token 明文只在签发时返回，落库只做精确匹配）。
type Session struct {
	Token     string    `json:"token"`
	UserID    string    `json:"userId"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// Expired 会话是否过期。
func (s Session) Expired(now time.Time) bool {
	return !s.ExpiresAt.IsZero() && !now.Before(s.ExpiresAt)
}
