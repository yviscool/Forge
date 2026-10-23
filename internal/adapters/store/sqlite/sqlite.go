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

	"github.com/yviscool/forge/internal/auth"
	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/ports"
	_ "modernc.org/sqlite"
)

const schemaVersion = 7

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
		if err := os.MkdirAll(dir, 0o700); err != nil {
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
	if err := migrateLegacyContestZipPasswords(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db, ex: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// Ping verifies the connection and schema watermark. Readiness must fail if a
// database was replaced by an unsupported schema after process startup.
func (s *Store) Ping(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return err
	}
	var v int
	if err := s.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&v); err != nil {
		return err
	}
	if v != schemaVersion {
		return fmt.Errorf("unsupported sqlite schema version %d (want %d)", v, schemaVersion)
	}
	// Verify the main database accepts writes, not merely network/file reads.
	// The probe is rolled back so readiness remains side-effect free.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("readiness write transaction: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO seq(name, val) VALUES('__forge_readiness_probe', 0)
		 ON CONFLICT(name) DO UPDATE SET val=seq.val`,
	); err != nil {
		return fmt.Errorf("readiness write probe: %w", err)
	}
	if err := tx.Rollback(); err != nil {
		return fmt.Errorf("readiness rollback: %w", err)
	}
	return nil
}

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
	if v < 4 {
		ddl := []string{
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username_nocase
			 ON users(lower(trim(json_extract(data, '$.username'))))
			 WHERE trim(json_extract(data, '$.username')) <> ''`,
		}
		for _, q := range ddl {
			if _, err := db.Exec(q); err != nil {
				return fmt.Errorf("migrate schema v4: %w", err)
			}
		}
		v = 4
	}
	if v < 5 {
		ddl := []string{
			`ALTER TABLE submissions ADD COLUMN request_id TEXT NOT NULL DEFAULT ''`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_submissions_request_id
			 ON submissions(request_id) WHERE request_id <> ''`,
		}
		for _, q := range ddl {
			if _, err := db.Exec(q); err != nil {
				return fmt.Errorf("migrate schema v5: %w", err)
			}
		}
		v = 5
	}
	if v < 6 {
		ddl := []string{
			`ALTER TABLE submissions ADD COLUMN verdict TEXT NOT NULL DEFAULT ''`,
			`UPDATE submissions
			 SET verdict=COALESCE(json_extract(data, '$.verdict'), '')`,
			`CREATE INDEX IF NOT EXISTS idx_submissions_verdict
			 ON submissions(verdict)`,
		}
		for _, q := range ddl {
			if _, err := db.Exec(q); err != nil {
				return fmt.Errorf("migrate schema v6: %w", err)
			}
		}
		v = 6
	}
	if v < 7 {
		ddl := []string{
			// 省份启用状态覆盖：未出现的行默认启用；行政区划本身编译在 domain 里。
			`CREATE TABLE IF NOT EXISTS province_states (
				code TEXT PRIMARY KEY,
				enabled INTEGER NOT NULL CHECK (enabled IN (0, 1))
			)`,
			// 准考证号一经分配不再变化且全省同组唯一；空值（未分配）不参与唯一约束。
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_exam_ticket
			 ON users(upper(json_extract(data, '$.examTicket')))
			 WHERE COALESCE(json_extract(data, '$.examTicket'), '') <> ''`,
		}
		for _, q := range ddl {
			if _, err := db.Exec(q); err != nil {
				return fmt.Errorf("migrate schema v7: %w", err)
			}
		}
		v = 7
	}
	if v != schemaVersion {
		return fmt.Errorf("unsupported sqlite schema version %d (want %d)", v, schemaVersion)
	}
	_, err := db.Exec(fmt.Sprintf(`PRAGMA user_version=%d`, v))
	return err
}

// migrateLegacyContestZipPasswords removes plaintext values written by early
// pre-release builds. It runs at startup because a lazy read migration could
// leave the secret on disk indefinitely.
func migrateLegacyContestZipPasswords(db *sql.DB) error {
	rows, err := db.Query(
		`SELECT id, data FROM contests
		 WHERE json_type(data, '$.zipPassword') = 'text'`,
	)
	if err != nil {
		return fmt.Errorf("scan legacy zip passwords: %w", err)
	}
	type pending struct {
		id   string
		data string
	}
	var updates []pending
	for rows.Next() {
		var id, raw string
		if err := rows.Scan(&id, &raw); err != nil {
			rows.Close()
			return fmt.Errorf("scan legacy zip password row: %w", err)
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(raw), &record); err != nil {
			rows.Close()
			return fmt.Errorf("decode contest %s for zip migration: %w", id, err)
		}
		legacy, _ := record["zipPassword"].(string)
		delete(record, "zipPassword")
		if strings.TrimSpace(legacy) != "" {
			verifier, err := auth.HashZipPassword(strings.TrimSpace(legacy))
			if err != nil {
				rows.Close()
				return fmt.Errorf(
					"contest %s has an incompatible legacy zip password; rotate it before upgrading: %w",
					id,
					err,
				)
			}
			record["zipPasswordHash"] = verifier
		}
		encoded, err := json.Marshal(record)
		if err != nil {
			rows.Close()
			return fmt.Errorf("encode contest %s zip migration: %w", id, err)
		}
		updates = append(updates, pending{id: id, data: string(encoded)})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate legacy zip password rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(updates) == 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, update := range updates {
		if _, err := tx.Exec(`UPDATE contests SET data=? WHERE id=?`, update.data, update.id); err != nil {
			return fmt.Errorf("migrate contest %s zip password: %w", update.id, err)
		}
	}
	return tx.Commit()
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

type contestAlias domain.Contest

type contestDisk struct {
	contestAlias
	ZipPasswordHash string `json:"zipPasswordHash,omitempty"`
}

func marshalContest(c domain.Contest) (string, error) {
	wire := contestDisk{
		contestAlias:    contestAlias(c),
		ZipPasswordHash: c.ZipPasswordHash,
	}
	return marshal(wire)
}

func unmarshalContest(raw string) (domain.Contest, error) {
	var wire contestDisk
	if err := json.Unmarshal([]byte(raw), &wire); err != nil {
		return domain.Contest{}, err
	}
	c := domain.Contest(wire.contestAlias)
	c.ZipPasswordHash = wire.ZipPasswordHash
	return c, nil
}

func (s *Store) putContest(c domain.Contest) error {
	data, err := marshalContest(c)
	if err != nil {
		return err
	}
	_, err = s.ex.Exec(
		`INSERT INTO contests(id, data) VALUES(?, ?)
		 ON CONFLICT(id) DO UPDATE SET data=excluded.data`,
		c.ID,
		data,
	)
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

// ProvinceStates 读取省份启用状态覆盖表。
func (s *Store) ProvinceStates() (map[string]bool, error) {
	rows, err := s.ex.Query(`SELECT code, enabled FROM province_states`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var code string
		var enabled int
		if err := rows.Scan(&code, &enabled); err != nil {
			return nil, err
		}
		out[code] = enabled == 1
	}
	return out, rows.Err()
}

// SetProvinceState 写入单个省份的启用状态；非内置省份直接拒绝，
// 避免运营侧误建出准考证号无法使用的假省份。
func (s *Store) SetProvinceState(code string, enabled bool) error {
	canonical, ok := domain.ProvinceByCode(code)
	if !ok {
		return fmt.Errorf("unknown province %q", code)
	}
	flag := 0
	if enabled {
		flag = 1
	}
	_, err := s.ex.Exec(
		`INSERT INTO province_states(code, enabled) VALUES(?, ?)
		 ON CONFLICT(code) DO UPDATE SET enabled=excluded.enabled`,
		canonical.Code, flag,
	)
	return err
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
	return c, s.putContest(c)
}

func (s *Store) GetContest(id string) (domain.Contest, error) {
	var raw string
	if err := s.ex.QueryRow(`SELECT data FROM contests WHERE id=?`, id).Scan(&raw); err == sql.ErrNoRows {
		return domain.Contest{}, notFound("contest")
	} else if err != nil {
		return domain.Contest{}, err
	}
	c, err := unmarshalContest(raw)
	if err != nil {
		return domain.Contest{}, err
	}
	return c, nil
}

func (s *Store) ListContests() ([]domain.Contest, error) {
	rows, err := s.ex.Query(`SELECT data FROM contests`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.Contest, 0)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		c, err := unmarshalContest(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
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
	return s.putContest(c)
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
		if err := s.putContest(c); err != nil {
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

func (s *Store) DeleteProblem(id string) error {
	res, err := s.ex.Exec(`DELETE FROM problems WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return notFound("problem")
	}
	return nil
}

func (s *Store) CreateSubmission(x domain.Submission) (domain.Submission, error) {
	if x.ID == "" {
		x.ID = s.NextID("sub")
	}
	data, err := marshal(x)
	if err != nil {
		return domain.Submission{}, err
	}
	_, err = s.ex.Exec(
		`INSERT INTO submissions(id, contest_id, request_id, verdict, data) VALUES(?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
			contest_id=excluded.contest_id,
			request_id=excluded.request_id,
			verdict=excluded.verdict,
			data=excluded.data`,
		x.ID, x.ContestID, x.RequestID, x.Verdict, data,
	)
	return x, err
}

func (s *Store) GetSubmission(id string) (domain.Submission, error) {
	var raw string
	var requestID string
	if e := s.ex.QueryRow(
		`SELECT data, request_id FROM submissions WHERE id=?`, id,
	).Scan(&raw, &requestID); e == sql.ErrNoRows {
		return domain.Submission{}, notFound("submission")
	} else if e != nil {
		return domain.Submission{}, e
	}
	var x domain.Submission
	if err := json.Unmarshal([]byte(raw), &x); err != nil {
		return domain.Submission{}, err
	}
	x.RequestID = requestID
	return x, nil
}

func (s *Store) FindSubmissionByRequestID(requestID string) (*domain.Submission, error) {
	if requestID == "" {
		return nil, nil
	}
	var raw string
	err := s.ex.QueryRow(
		`SELECT data FROM submissions WHERE request_id=?`, requestID,
	).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var x domain.Submission
	if err := json.Unmarshal([]byte(raw), &x); err != nil {
		return nil, err
	}
	x.RequestID = requestID
	return &x, nil
}

func (s *Store) ListSubmissions(contestID string) ([]domain.Submission, error) {
	var out []domain.Submission
	if err := s.listScoped(tSubs, contestID, &out); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SubmittedAt.After(out[j].SubmittedAt) })
	return out, nil
}

func (s *Store) ListSubmissionFeed(contestID, userID string, limit int) ([]domain.Submission, error) {
	query := `SELECT data, request_id FROM submissions WHERE contest_id=?`
	args := []any{contestID}
	if userID != "" {
		query += ` AND json_extract(data, '$.userId')=?`
		args = append(args, userID)
	}
	query += ` ORDER BY json_extract(data, '$.submittedAt') DESC, id DESC`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.ex.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Submission{}
	for rows.Next() {
		var raw, requestID string
		if err := rows.Scan(&raw, &requestID); err != nil {
			return nil, err
		}
		var x domain.Submission
		if err := json.Unmarshal([]byte(raw), &x); err != nil {
			return nil, err
		}
		x.RequestID = requestID
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) ListQueuedSubmissions() ([]domain.Submission, error) {
	rows, err := s.ex.Query(
		`SELECT data, request_id FROM submissions
		 WHERE verdict=?
		 ORDER BY json_extract(data, '$.submittedAt') ASC, id ASC`,
		domain.VerdictQueued,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Submission{}
	for rows.Next() {
		var raw, requestID string
		if err := rows.Scan(&raw, &requestID); err != nil {
			return nil, err
		}
		var x domain.Submission
		if err := json.Unmarshal([]byte(raw), &x); err != nil {
			return nil, err
		}
		x.RequestID = requestID
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) UpdateSubmission(x domain.Submission) error {
	existing, err := s.GetSubmission(x.ID)
	if err != nil {
		return err
	}
	if x.RequestID == "" {
		x.RequestID = existing.RequestID
	}
	data, err := marshal(x)
	if err != nil {
		return err
	}
	_, err = s.ex.Exec(
		`UPDATE submissions SET contest_id=?, request_id=?, verdict=?, data=? WHERE id=?`,
		x.ContestID, x.RequestID, x.Verdict, data, x.ID,
	)
	return err
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

func (s *Store) DeleteExpiredSessions(now time.Time) (int, error) {
	rows, err := s.ex.Query(`SELECT token, data FROM sessions`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var expired []string
	for rows.Next() {
		var token, data string
		if err := rows.Scan(&token, &data); err != nil {
			return 0, err
		}
		var sess domain.Session
		if err := json.Unmarshal([]byte(data), &sess); err != nil {
			continue
		}
		if sess.Expired(now) {
			expired = append(expired, token)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	_ = rows.Close()
	for _, token := range expired {
		if _, err := s.ex.Exec(`DELETE FROM sessions WHERE token=?`, token); err != nil {
			return 0, err
		}
	}
	return len(expired), nil
}
