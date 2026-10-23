package domain

import (
	"encoding/json"
	"time"
)

// Status 比赛生命周期。
type Status string

const (
	Draft    Status = "draft"
	Running  Status = "running"
	Finished Status = "finished"
)

// CanTransitionTo 状态机合法迁移判断。
func (s Status) CanTransitionTo(next Status) bool {
	switch s {
	case Draft:
		return next == Running
	case Running:
		return next == Finished
	default:
		return false
	}
}

// Roles 角色常量。
const (
	RoleAdmin   = "admin"
	RoleTeacher = "teacher"
	RoleStudent = "student"
)

// Verdicts 判题结果规范值。
const (
	VerdictQueued            = "queued"
	VerdictJudging           = "judging"
	VerdictAccepted          = "accepted"
	VerdictWrongAnswer       = "wrong_answer"
	VerdictPresentationError = "presentation_error"
	VerdictTimeLimit         = "time_limit"
	VerdictMemoryLimit       = "memory_limit"
	VerdictOutputLimit       = "output_limit"
	VerdictRuntimeError      = "runtime_error"
	VerdictCompileError      = "compile_error"
	VerdictCheckerError      = "checker_error"
	VerdictSystemError       = "system_error"
	VerdictSkipped           = "skipped"
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
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	RankingMode string `json:"rankingMode,omitempty"`
	// AllowedLanguages 允许提交的语言白名单（空=不限；复赛填 ["cpp"]）。
	AllowedLanguages   []string  `json:"allowedLanguages,omitempty"`
	ProblemIDs         []string  `json:"problemIds"`
	GroupIDs           []string  `json:"groupIds"`
	ParticipantUserIDs []string  `json:"participantUserIds"`
	CreatedAt          time.Time `json:"createdAt"`
	StartedAt          time.Time `json:"startedAt,omitempty"`
	FinishedAt         time.Time `json:"finishedAt,omitempty"`
	// FinalRanking freezes the last OI ranking when a contest finishes so a
	// later account change (rename, password reset, group edit) cannot rewrite
	// published results.
	FinalRanking []RankEntry    `json:"finalRanking,omitempty"`
	FinalACM     []ACMRankEntry `json:"finalAcm,omitempty"`

	// 考场物理设备/IP 强绑定
	BindIPEnabled bool              `json:"bindIpEnabled,omitempty"`
	BoundIPs      map[string]string `json:"boundIps,omitempty"` // userID -> ClientIP

	// 严格考试时间窗口与加时
	WindowEnabled   bool           `json:"windowEnabled,omitempty"`
	StartTime       *time.Time     `json:"startTime,omitempty"`
	EndTime         *time.Time     `json:"endTime,omitempty"`
	DurationMinutes int            `json:"durationMinutes,omitempty"`
	Extensions      map[string]int `json:"extensions,omitempty"` // userID -> extra minutes

	// 考场通告与即时广播
	Notice     string             `json:"notice,omitempty"`
	Broadcasts []BroadcastMessage `json:"broadcasts,omitempty"`

	// ZipPasswordHash is a salted one-way verifier. It is intentionally never
	// serialized by the API or realtime transports.
	ZipPasswordHash string `json:"-"`
	// ZipProtected tells students that an unlock password exists without
	// exposing the password itself.
	ZipProtected bool `json:"-"`
	// ZipUnlocked is a per-response view field. It is never persisted: the
	// transport layer derives it from the signed, contest-scoped unlock grant.
	ZipUnlocked bool `json:"-"`
}

func (c Contest) ZipPasswordConfigured() bool {
	return c.ZipPasswordHash != ""
}

// MarshalJSON derives presentation-only fields and guarantees that the stored
// ZIP verifier can never cross an HTTP or SSE boundary.
func (c Contest) MarshalJSON() ([]byte, error) {
	type contestWire Contest
	return json.Marshal(struct {
		contestWire
		ZipProtected bool `json:"zipProtected,omitempty"`
		ZipUnlocked  bool `json:"zipUnlocked,omitempty"`
	}{
		contestWire:  contestWire(c),
		ZipProtected: c.ZipProtected || c.ZipPasswordConfigured(),
		ZipUnlocked:  c.ZipUnlocked,
	})
}

type BroadcastMessage struct {
	ID           string    `json:"id"`
	ContestID    string    `json:"contestId"`
	SenderName   string    `json:"senderName"`
	TargetUserID string    `json:"targetUserId,omitempty"` // 空或 "all" 为全员广播，指定用户为定向提醒
	Title        string    `json:"title"`
	Content      string    `json:"content"`
	Level        string    `json:"level"` // "info", "warning", "urgent"
	CreatedAt    time.Time `json:"createdAt"`
}

func (c Contest) CanTransitionTo(next Status) bool {
	return Status(c.Status).CanTransitionTo(next)
}

func (c Contest) CanModifySettings() bool {
	// Once a paper is published to candidates, statements, tests, ordering,
	// scores, languages, and ranking mode are frozen. Operational controls
	// (notice, extensions, IP recovery, broadcasts) use separate use-cases.
	return c.Status == string(Draft)
}

func (c Contest) CanSubmit() bool {
	return c.Status == string(Running)
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
	DependsOn   []int  `json:"dependsOn,omitempty"`
}

// TaskType 题目类型（对标 LemonLime TaskType 子集）。
type TaskType string

const (
	TaskTraditional TaskType = "traditional"
	TaskAnswersOnly TaskType = "answers_only"
	TaskInteraction TaskType = "interaction"
	TaskChoice      TaskType = "choice" // CCF GESP 客观选择/判断题
)

// MaxSubmissionBytes is the contract shared by the web client and the server.
// Keep the transport limit above this value so valid submissions are not
// rejected by generic request-size middleware first.
const MaxSubmissionBytes = 64 << 10

// ChoiceQuestion CCF GESP 客观选择/判断题结构。
type ChoiceQuestion struct {
	ID          int      `json:"id"`                    // 题号（从 1 起）
	Type        string   `json:"type"`                  // "single", "multiple", "judge"
	Stem        string   `json:"stem"`                  // 题干 Markdown
	Options     []string `json:"options,omitempty"`     // 选项列表（如 ["A. 1", "B. 2", "C. 3", "D. 4"]）
	Answer      any      `json:"answer"`                // 标准答案："A" 或 ["A", "C"] 或 "T"/"F"
	Score       int      `json:"score"`                 // 该题分值
	Explanation string   `json:"explanation,omitempty"` // 题解/说明
}

// IOMode 程序 IO 方式。
type IOMode string

const (
	IOStdio IOMode = "stdio"
	IOFile  IOMode = "file"
)

type Problem struct {
	ID             string         `json:"id"`
	ContestID      string         `json:"contestId"`
	Code           string         `json:"code"`
	Title          string         `json:"title"`
	Statement      string         `json:"statement"`
	Constraints    string         `json:"constraints"`
	Input          string         `json:"input"`
	Output         string         `json:"output"`
	Examples       string         `json:"examples"`
	Locale         string         `json:"locale"`
	TimeLimitMs    int            `json:"timeLimitMs"`
	MemoryLimitMiB int            `json:"memoryLimitMib"`
	CompareMode    ComparisonMode `json:"compareMode,omitempty"`
	RealEps        float64        `json:"realEps,omitempty"`
	// OutputLimitKiB 单点 stdout 上限（KB），0 则默认；超限判 OLE。
	OutputLimitKiB int `json:"outputLimitKib,omitempty"`
	// CompileTimeoutSec 单次编译上限（秒），0 则默认。
	CompileTimeoutSec int                      `json:"compileTimeoutSec,omitempty"`
	TaskType          TaskType                 `json:"taskType,omitempty"`
	IOMode            IOMode                   `json:"ioMode,omitempty"`
	InFile            string                   `json:"inFile,omitempty"`
	OutFile           string                   `json:"outFile,omitempty"`
	CheckerLang       string                   `json:"checkerLang,omitempty"`
	CheckerCode       string                   `json:"checkerCode,omitempty"`
	InteractorLang    string                   `json:"interactorLang,omitempty"`
	InteractorCode    string                   `json:"interactorCode,omitempty"`
	Locales           map[string]ProblemLocale `json:"locales,omitempty"`
	TestCases         []TestCase               `json:"testCases,omitempty"`
	// GESP 客观选择/判断题
	ChoiceQuestions []ChoiceQuestion `json:"choiceQuestions,omitempty"`
	ChoiceKeyJSON   string           `json:"choiceKeyJson,omitempty"`
	UpdatedAt       time.Time        `json:"updatedAt"`
}

type Submission struct {
	ID        string `json:"id"`
	ContestID string `json:"contestId"`
	ProblemID string `json:"problemID"`
	UserID    string `json:"userId"`
	UserName  string `json:"userName,omitempty"`
	Language  string `json:"language"`
	Code      string `json:"code"`
	// RequestID is an internal idempotency scope. It is persisted separately
	// from the client-visible submission payload.
	RequestID string `json:"-"`
	Verdict   string `json:"verdict"`
	Score     int    `json:"score"`
	// CompileMessage 编译错误原文（CE 申诉/教学用， peer 脱敏）。
	CompileMessage string       `json:"compileMessage,omitempty"`
	Cases          []CaseResult `json:"cases,omitempty"`
	SubmittedAt    time.Time    `json:"submittedAt"`
	JudgedAt       time.Time    `json:"judgedAt,omitempty"`
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
