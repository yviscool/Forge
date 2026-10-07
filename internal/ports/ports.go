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
type Broadcaster interface {
	Publish(e domain.Event)
	Subscribe(contestID string) (<-chan domain.Event, func())
}

// Store 持久化端口。Phase0 由 memory 实现，Phase1 由 sqlite 实现同一接口。
type Store interface {
	CreateUser(u domain.User) (domain.User, error)
	GetUser(id string) (domain.User, error)
	ListUsers() ([]domain.User, error)
	UpdateUser(u domain.User) error

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

	CreateSubmission(s domain.Submission) (domain.Submission, error)
	GetSubmission(id string) (domain.Submission, error)
	ListSubmissions(contestID string) ([]domain.Submission, error)
	UpdateSubmission(s domain.Submission) error

	// Credentials & sessions（auth 端口：hash 永不进 domain.User JSON）。
	SetPasswordHash(userID, hash string) error
	GetPasswordHash(userID string) (string, error)
	SaveSession(s domain.Session) error
	GetSession(token string) (domain.Session, error)
	DeleteSession(token string) error
	DeleteSessionsForUser(userID string) error

	NextID(prefix string) string
}

// Transactor 事务边界接口：支持在事务内执行原子操作并在报错时回滚。
type Transactor interface {
	WithTx(ctx context.Context, fn func(tx Store) error) error
}
