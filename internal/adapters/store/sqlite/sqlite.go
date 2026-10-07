// Package sqlite 提供 ports.Store 的 SQLite 实现：JSON 行 + 迁移水位。
// 单文件、WAL、纯 Go 驱动（modernc），教师机双击即跑，重启不丢。
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/ports"
	_ "modernc.org/sqlite"
)

const schemaVersion = 2

type dbExecutor interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

type Store struct {
	db *sql.DB
	ex dbExecutor
}

// Open 打开（不存在则创建）SQLite 文件并执行迁移。
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db, ex: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// WithTx 开启事务执行闭包，返回错误时自动回滚。若当前已在事务中则复用。
func (s *Store) WithTx(ctx context.Context, fn func(tx ports.Store) error) error {
	if _, isTx := s.ex.(*sql.Tx); isTx {
		return fn(s)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	txs := &Store{db: s.db, ex: tx}
	if err := fn(txs); err != nil {
		return err
	}
	return tx.Commit()
}

func migrate(db *sql.DB) error {
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		return err
	}
	if v < 1 {
		ddl := []string{
			`CREATE TABLE IF NOT EXISTS users (id TEXT PRIMARY KEY, data TEXT NOT NULL)`,
			`CREATE TABLE IF NOT EXISTS groups (id TEXT PRIMARY KEY, data TEXT NOT NULL)`,
			`CREATE TABLE IF NOT EXISTS contests (id TEXT PRIMARY KEY, data TEXT NOT NULL)`,
			`CREATE TABLE IF NOT EXISTS problems (id TEXT PRIMARY KEY, contest_id TEXT NOT NULL, data TEXT NOT NULL)`,
			`CREATE INDEX IF NOT EXISTS idx_problems_contest ON problems(contest_id)`,
			`CREATE TABLE IF NOT EXISTS submissions (id TEXT PRIMARY KEY, contest_id TEXT NOT NULL, data TEXT NOT NULL)`,
			`CREATE INDEX IF NOT EXISTS idx_submissions_contest ON submissions(contest_id)`,
			`CREATE TABLE IF NOT EXISTS seq (name TEXT PRIMARY KEY, val INTEGER NOT NULL)`,
		}
		for _, q := range ddl {
			if _, err := db.Exec(q); err != nil {
				return err
			}
		}
		v = 1
	}
	if v < 2 {
		ddl := []string{
			`CREATE TABLE IF NOT EXISTS credentials (user_id TEXT PRIMARY KEY, hash TEXT NOT NULL)`,
			`CREATE TABLE IF NOT EXISTS sessions (token TEXT PRIMARY KEY, user_id TEXT NOT NULL, data TEXT NOT NULL)`,
			`CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id)`,
		}
		for _, q := range ddl {
			if _, err := db.Exec(q); err != nil {
				return err
			}
		}
		v = 2
	}
	if v < 3 {
		ddl := []string{
			`CREATE INDEX IF NOT EXISTS idx_problems_contest_code ON problems(contest_id, json_extract(data, '$.code'))`,
		}
		for _, q := range ddl {
			if _, err := db.Exec(q); err != nil {
				return err
			}
		}
		v = 3
	}
	_, err := db.Exec(fmt.Sprintf(`PRAGMA user_version=%d`, v))
	return err
}

func marshal(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func notFound(what string) error { return errors.New(what + " not found") }

func (s *Store) put(table, id string, v any) error {
	data, err := marshal(v)
	if err != nil {
		return err
	}
	_, err = s.ex.Exec(fmt.Sprintf(`INSERT INTO %s(id, data) VALUES(?, ?) ON CONFLICT(id) DO UPDATE SET data=excluded.data`, table), id, data)
	return err
}

func (s *Store) putScoped(table, id, contestID string, v any) error {
	data, err := marshal(v)
	if err != nil {
		return err
	}
	_, err = s.ex.Exec(fmt.Sprintf(`INSERT INTO %s(id, contest_id, data) VALUES(?, ?, ?) ON CONFLICT(id) DO UPDATE SET data=excluded.data, contest_id=excluded.contest_id`, table), id, contestID, data)
	return err
}

func (s *Store) get(table, id string, v any, what string) error {
	var data string
	err := s.ex.QueryRow(fmt.Sprintf(`SELECT data FROM %s WHERE id=?`, table), id).Scan(&data)
	if err == sql.ErrNoRows {
		return notFound(what)
	}
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(data), v)
}

func (s *Store) listAll(table string, v any) error {
	rows, err := s.ex.Query(fmt.Sprintf(`SELECT data FROM %s`, table))
	if err != nil {
		return err
	}
	defer rows.Close()
	return scanAll(rows, v)
}

func (s *Store) listScoped(table, contestID string, v any) error {
	rows, err := s.ex.Query(fmt.Sprintf(`SELECT data FROM %s WHERE contest_id=?`, table), contestID)
	if err != nil {
		return err
	}
	defer rows.Close()
	return scanAll(rows, v)
}

func scanAll(rows *sql.Rows, v any) error {
	var raws []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return err
		}
		raws = append(raws, d)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	arr := "[" + strings.Join(raws, ",") + "]"
	if len(raws) == 0 {
		arr = "[]"
	}
	return json.Unmarshal([]byte(arr), v)
}

const (
	tUsers    = "users"
	tGroups   = "groups"
	tContests = "contests"
	tProblems = "problems"
	tSubs     = "submissions"
)

func (s *Store) NextID(prefix string) string {
	var n int64
	err := s.ex.QueryRow(`INSERT INTO seq(name,val) VALUES('seq',1) ON CONFLICT(name) DO UPDATE SET val=val+1 RETURNING val`).Scan(&n)
	if err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s-%04d", prefix, n)
}

func (s *Store) CreateUser(u domain.User) (domain.User, error) {
	if u.ID == "" {
		u.ID = s.NextID("usr")
	}
	return u, s.put(tUsers, u.ID, u)
}

func (s *Store) GetUser(id string) (domain.User, error) {
	var u domain.User
	if err := s.get(tUsers, id, &u, "user"); err != nil {
		return domain.User{}, err
	}
	return u, nil
}

func (s *Store) ListUsers() ([]domain.User, error) {
	var out []domain.User
	if err := s.listAll(tUsers, &out); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *Store) UpdateUser(u domain.User) error {
	var cur domain.User
	if err := s.get(tUsers, u.ID, &cur, "user"); err != nil {
		return err
	}
	return s.put(tUsers, u.ID, u)
}

func (s *Store) CreateGroup(g domain.Group) (domain.Group, error) {
	if g.ID == "" {
		g.ID = s.NextID("grp")
	}
	return g, s.put(tGroups, g.ID, g)
}

func (s *Store) GetGroup(id string) (domain.Group, error) {
	var g domain.Group
	if err := s.get(tGroups, id, &g, "group"); err != nil {
		return domain.Group{}, err
	}
	return g, nil
}

func (s *Store) ListGroups() ([]domain.Group, error) {
	var out []domain.Group
	if err := s.listAll(tGroups, &out); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *Store) UpdateGroup(g domain.Group) error {
	var cur domain.Group
	if err := s.get(tGroups, g.ID, &cur, "group"); err != nil {
		return err
	}
	return s.put(tGroups, g.ID, g)
}

func (s *Store) CreateContest(c domain.Contest) (domain.Contest, error) {
	if c.ID == "" {
		c.ID = s.NextID("cnt")
	}
	return c, s.put(tContests, c.ID, c)
}

func (s *Store) GetContest(id string) (domain.Contest, error) {
	var c domain.Contest
	if err := s.get(tContests, id, &c, "contest"); err != nil {
		return domain.Contest{}, err
	}
	return c, nil
}

func (s *Store) ListContests() ([]domain.Contest, error) {
	var out []domain.Contest
	if err := s.listAll(tContests, &out); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (s *Store) UpdateContest(c domain.Contest) error {
	var cur domain.Contest
	if err := s.get(tContests, c.ID, &cur, "contest"); err != nil {
		return err
	}
	return s.put(tContests, c.ID, c)
}

func (s *Store) CreateProblem(p domain.Problem) (domain.Problem, error) {
	if p.ID == "" {
		p.ID = s.NextID("prb")
	}
	if err := s.putScoped(tProblems, p.ID, p.ContestID, p); err != nil {
		return domain.Problem{}, err
	}
	// 同步比赛 ProblemIDs
	if c, err := s.GetContest(p.ContestID); err == nil {
		c.ProblemIDs = append(c.ProblemIDs, p.ID)
		if err := s.put(tContests, c.ID, c); err != nil {
			return domain.Problem{}, err
		}
	}
	return p, nil
}

func (s *Store) GetProblem(id string) (domain.Problem, error) {
	var raw string
	if e := s.ex.QueryRow(`SELECT data FROM problems WHERE id=?`, id).Scan(&raw); e == sql.ErrNoRows {
		return domain.Problem{}, notFound("problem")
	} else if e != nil {
		return domain.Problem{}, e
	}
	var p domain.Problem
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return domain.Problem{}, err
	}
	return p, nil
}

func (s *Store) ListProblems(contestID string) ([]domain.Problem, error) {
	var out []domain.Problem
	if err := s.listScoped(tProblems, contestID, &out); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out, nil
}

func (s *Store) UpdateProblem(p domain.Problem) (domain.Problem, error) {
	if _, err := s.GetProblem(p.ID); err != nil {
		return domain.Problem{}, err
	}
	if err := s.putScoped(tProblems, p.ID, p.ContestID, p); err != nil {
		return domain.Problem{}, err
	}
	return p, nil
}

func (s *Store) CreateSubmission(x domain.Submission) (domain.Submission, error) {
	if x.ID == "" {
		x.ID = s.NextID("sub")
	}
	return x, s.putScoped(tSubs, x.ID, x.ContestID, x)
}

func (s *Store) GetSubmission(id string) (domain.Submission, error) {
	var raw string
	if e := s.ex.QueryRow(`SELECT data FROM submissions WHERE id=?`, id).Scan(&raw); e == sql.ErrNoRows {
		return domain.Submission{}, notFound("submission")
	} else if e != nil {
		return domain.Submission{}, e
	}
	var x domain.Submission
	if err := json.Unmarshal([]byte(raw), &x); err != nil {
		return domain.Submission{}, err
	}
	return x, nil
}

func (s *Store) ListSubmissions(contestID string) ([]domain.Submission, error) {
	var out []domain.Submission
	if err := s.listScoped(tSubs, contestID, &out); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SubmittedAt.After(out[j].SubmittedAt) })
	return out, nil
}

func (s *Store) UpdateSubmission(x domain.Submission) error {
	if _, err := s.GetSubmission(x.ID); err != nil {
		return err
	}
	return s.putScoped(tSubs, x.ID, x.ContestID, x)
}

func (s *Store) SetPasswordHash(userID, hash string) error {
	if _, err := s.GetUser(userID); err != nil {
		return err
	}
	_, err := s.ex.Exec(`INSERT INTO credentials(user_id, hash) VALUES(?, ?) ON CONFLICT(user_id) DO UPDATE SET hash=excluded.hash`, userID, hash)
	return err
}

func (s *Store) GetPasswordHash(userID string) (string, error) {
	if _, err := s.GetUser(userID); err != nil {
		return "", err
	}
	var hash string
	if err := s.ex.QueryRow(`SELECT hash FROM credentials WHERE user_id=?`, userID).Scan(&hash); err == sql.ErrNoRows {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return hash, nil
}

func (s *Store) SaveSession(sess domain.Session) error {
	data, err := marshal(sess)
	if err != nil {
		return err
	}
	_, err = s.ex.Exec(`INSERT INTO sessions(token, user_id, data) VALUES(?, ?, ?) ON CONFLICT(token) DO UPDATE SET user_id=excluded.user_id, data=excluded.data`, sess.Token, sess.UserID, data)
	return err
}

func (s *Store) GetSession(token string) (domain.Session, error) {
	var data string
	if err := s.ex.QueryRow(`SELECT data FROM sessions WHERE token=?`, token).Scan(&data); err == sql.ErrNoRows {
		return domain.Session{}, errors.New("session not found")
	} else if err != nil {
		return domain.Session{}, err
	}
	var sess domain.Session
	if err := json.Unmarshal([]byte(data), &sess); err != nil {
		return domain.Session{}, err
	}
	return sess, nil
}

func (s *Store) DeleteSession(token string) error {
	_, err := s.ex.Exec(`DELETE FROM sessions WHERE token=?`, token)
	return err
}

func (s *Store) DeleteSessionsForUser(userID string) error {
	_, err := s.ex.Exec(`DELETE FROM sessions WHERE user_id=?`, userID)
	return err
}
