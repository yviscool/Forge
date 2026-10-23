// Package ports 定义六边形架构的端口：domain 不依赖任何适配器，
// app 只依赖接口，store/broadcaster 可替换（memory → sqlite，SSE → NATS）。
package ports

import (
	"context"
	"time"

	"github.com/yviscool/forge/internal/domain"
)

// Clock 可注入时钟，便于测试固定时间。
type Clock interface {
	Now() time.Time
}

// SystemClock 生产实现。
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }

// IDGenerator ID 生成策略（序号/UUID/ULID），存储层可替换。
type IDGenerator interface {
	New(prefix string) string
}

// Broadcaster 实时事件端口，支持按比赛过滤订阅。
// Subscribe 的 userID 是不透明的在线标识（可空）：SSE 断连时用于注销监考在线状态。
type Broadcaster interface {
	Publish(e domain.Event)
	Subscribe(contestID, userID string) (<-chan domain.Event, func())
}

// Store 持久化端口。Phase0 由 memory 实现，Phase1 由 sqlite 实现同一接口。
type Store interface {
	CreateUser(u domain.User) (domain.User, error)
	GetUser(id string) (domain.User, error)
	ListUsers() ([]domain.User, error)
	UpdateUser(u domain.User) error

	// ProvinceStates 返回省份启用状态覆盖表：编号 → 是否启用；
	// 未出现的省份默认启用（内置行政区划本身编译在 domain 里）。
	ProvinceStates() (map[string]bool, error)
	// SetProvinceState 记录单个省份的启用/停用；code 必须是内置省份。
	SetProvinceState(code string, enabled bool) error

	CreateGroup(g domain.Group) (domain.Group, error)
	GetGroup(id string) (domain.Group, error)
	ListGroups() ([]domain.Group, error)
	UpdateGroup(g domain.Group) error

	CreateContest(c domain.Contest) (domain.Contest, error)
	GetContest(id string) (domain.Contest, error)
	ListContests() ([]domain.Contest, error)
	UpdateContest(c domain.Contest) error

	CreateProblem(p domain.Problem) (domain.Problem, error)
	GetProblem(id string) (domain.Problem, error)
	ListProblems(contestID string) ([]domain.Problem, error)
	UpdateProblem(p domain.Problem) (domain.Problem, error)
	DeleteProblem(id string) error

	CreateSubmission(s domain.Submission) (domain.Submission, error)
	GetSubmission(id string) (domain.Submission, error)
	FindSubmissionByRequestID(requestID string) (*domain.Submission, error)
	ListSubmissions(contestID string) ([]domain.Submission, error)
	ListSubmissionFeed(contestID, userID string, limit int) ([]domain.Submission, error)
	ListQueuedSubmissions() ([]domain.Submission, error)
	UpdateSubmission(s domain.Submission) error

	// CodeVersions 代码版本仓：Seq 在（比赛, 题目, 用户）维度从 1 连续递增，
	// 每维度只保留最近 MaxCodeVersionsPerProblem 个（写入时自动裁剪）。
	// ListCodeVersions 按 Seq 降序返回（最新在前）。
	CreateCodeVersion(v domain.CodeVersion) (domain.CodeVersion, error)
	ListCodeVersions(contestID, problemID, userID string, limit int) ([]domain.CodeVersion, error)

	// Credentials & sessions（auth 端口：hash 永不进 domain.User JSON）。
	SetPasswordHash(userID, hash string) error
	GetPasswordHash(userID string) (string, error)
	SaveSession(s domain.Session) error
	GetSession(token string) (domain.Session, error)
	DeleteSession(token string) error
	DeleteSessionsForUser(userID string) error
	DeleteExpiredSessions(now time.Time) (int, error)

	NextID(prefix string) string
}

// HealthChecker is implemented by stores that can verify their backing
// datastore is still reachable. Readiness endpoints use this optional port so
// an open file descriptor alone never reports a broken database as healthy.
type HealthChecker interface {
	Ping(ctx context.Context) error
}

// Transactor 事务边界接口：支持在事务内执行原子操作并在报错时回滚。
type Transactor interface {
	WithTx(ctx context.Context, fn func(tx Store) error) error
}
