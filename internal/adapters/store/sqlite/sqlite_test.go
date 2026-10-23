package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yviscool/forge/internal/adapters/store/storetest"
	"github.com/yviscool/forge/internal/auth"
	"github.com/yviscool/forge/internal/domain"
)

func schemaV1() []string {
	return []string{
		`CREATE TABLE users (id TEXT PRIMARY KEY, data TEXT NOT NULL)`,
		`CREATE TABLE groups (id TEXT PRIMARY KEY, data TEXT NOT NULL)`,
		`CREATE TABLE contests (id TEXT PRIMARY KEY, data TEXT NOT NULL)`,
		`CREATE TABLE problems (id TEXT PRIMARY KEY, contest_id TEXT NOT NULL, data TEXT NOT NULL)`,
		`CREATE INDEX idx_problems_contest ON problems(contest_id)`,
		`CREATE TABLE submissions (id TEXT PRIMARY KEY, contest_id TEXT NOT NULL, data TEXT NOT NULL)`,
		`CREATE INDEX idx_submissions_contest ON submissions(contest_id)`,
		`CREATE TABLE seq (name TEXT PRIMARY KEY, val INTEGER NOT NULL)`,
	}
}

func schemaV2() []string {
	return []string{
		`CREATE TABLE credentials (user_id TEXT PRIMARY KEY, hash TEXT NOT NULL)`,
		`CREATE TABLE sessions (token TEXT PRIMARY KEY, user_id TEXT NOT NULL, data TEXT NOT NULL)`,
		`CREATE INDEX idx_sessions_user ON sessions(user_id)`,
	}
}

func schemaV3() []string {
	return []string{
		`CREATE INDEX idx_problems_contest_code ON problems(contest_id, json_extract(data, '$.code'))`,
	}
}

func schemaV4() []string {
	return []string{
		`CREATE UNIQUE INDEX idx_users_username_nocase
		 ON users(lower(trim(json_extract(data, '$.username'))))
		 WHERE trim(json_extract(data, '$.username')) <> ''`,
	}
}

func schemaV5() []string {
	return []string{
		`ALTER TABLE submissions ADD COLUMN request_id TEXT NOT NULL DEFAULT ''`,
		`CREATE UNIQUE INDEX idx_submissions_request_id
		 ON submissions(request_id) WHERE request_id <> ''`,
	}
}

func TestStoreConformance(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "forge.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	storetest.Exercise(t, s)
	storetest.PasswordsAndSessions(t, s)
	storetest.TransactionSemantics(t, s)
	storetest.ProvinceStates(t, s)
	storetest.CodeVersions(t, s)
}

func TestPersistenceAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "forge.db")

	s1, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := s1.CreateUser(domain.User{Name: "Ada", Role: "student"})
	c, _ := s1.CreateContest(domain.Contest{
		Name:            "CSP",
		Status:          "draft",
		ZipPasswordHash: "$argon2id$test-verifier",
	})
	p, _ := s1.CreateProblem(domain.Problem{ContestID: c.ID, Code: "A", Title: "和"})
	x, _ := s1.CreateSubmission(domain.Submission{
		ContestID: c.ID, ProblemID: p.ID, UserID: u.ID, Score: 42, RequestID: "persisted-request",
	})
	if err := s1.Close(); err != nil {
		t.Fatal(err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if _, err := s2.GetUser(u.ID); err != nil {
		t.Fatalf("user lost: %v", err)
	}
	if cc, err := s2.GetContest(c.ID); err != nil ||
		len(cc.ProblemIDs) != 1 ||
		cc.ZipPasswordHash != "$argon2id$test-verifier" {
		t.Fatalf("contest lost: %v %+v", err, cc)
	}
	if xx, err := s2.GetSubmission(x.ID); err != nil || xx.Score != 42 {
		t.Fatalf("submission lost: %v %+v", err, xx)
	}
	if xx, err := s2.FindSubmissionByRequestID("persisted-request"); err != nil ||
		xx == nil || xx.ID != x.ID {
		t.Fatalf("submission request id lost: %v %+v", err, xx)
	}
	// seq 计数器不断流：新 ID 不得与已有关联。
	if id := s2.NextID("usr"); id == u.ID {
		t.Fatalf("seq reset after reopen: %s", id)
	}
}

func TestOpenMigratesLegacyPlaintextZipPassword(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-zip.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.CreateContest(domain.Contest{Name: "Legacy", Status: "draft"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(
		`UPDATE contests SET data=json_set(data, '$.zipPassword', ?) WHERE id=?`,
		"legacy-exam-password",
		c.ID,
	); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := reopened.GetContest(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ZipPasswordHash == "" || !strings.HasPrefix(got.ZipPasswordHash, "$argon2id$") {
		t.Fatalf("legacy password was not converted to a verifier: %+v", got)
	}
	ok, err := auth.VerifyZipPassword(got.ZipPasswordHash, "legacy-exam-password")
	if err != nil || !ok {
		t.Fatalf("migrated verifier does not match: ok=%v err=%v", ok, err)
	}
	var raw string
	if err := reopened.db.QueryRow(`SELECT data FROM contests WHERE id=?`, c.ID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "legacy-exam-password") ||
		strings.Contains(raw, `"zipPassword"`) {
		t.Fatalf("legacy plaintext password remains at rest: %s", raw)
	}
}

func TestMigrateFromPriorSchemas(t *testing.T) {
	tests := []struct {
		name    string
		version int
		ddl     []string
	}{
		{name: "v1", version: 1, ddl: schemaV1()},
		{name: "v2", version: 2, ddl: append(schemaV1(), schemaV2()...)},
		{
			name:    "v3",
			version: 3,
			ddl:     append(append(schemaV1(), schemaV2()...), schemaV3()...),
		},
		{
			name:    "v4",
			version: 4,
			ddl:     append(append(append(schemaV1(), schemaV2()...), schemaV3()...), schemaV4()...),
		},
		{
			name:    "v5",
			version: 5,
			ddl:     append(append(append(append(schemaV1(), schemaV2()...), schemaV3()...), schemaV4()...), schemaV5()...),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "legacy.db")
			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			for _, q := range tt.ddl {
				if _, err := db.Exec(q); err != nil {
					_ = db.Close()
					t.Fatalf("schema: %v", err)
				}
			}
			userJSON := `{"id":"legacy","username":"legacy","name":"Legacy","role":"student","groups":[],"createdAt":"2026-01-01T00:00:00Z"}`
			if _, err := db.Exec(`INSERT INTO users(id,data) VALUES('legacy',?)`, userJSON); err != nil {
				_ = db.Close()
				t.Fatal(err)
			}
			submissionJSON := `{"id":"legacy-sub","contestId":"legacy-contest","problemID":"legacy-problem","userId":"legacy","language":"cpp","code":"x","verdict":"queued","score":0,"submittedAt":"2026-01-01T00:00:01Z"}`
			if tt.version >= 5 {
				_, err = db.Exec(
					`INSERT INTO submissions(id,contest_id,request_id,data) VALUES('legacy-sub','legacy-contest','',?)`,
					submissionJSON,
				)
			} else {
				_, err = db.Exec(
					`INSERT INTO submissions(id,contest_id,data) VALUES('legacy-sub','legacy-contest',?)`,
					submissionJSON,
				)
			}
			if err != nil {
				_ = db.Close()
				t.Fatal(err)
			}
			if _, err := db.Exec(fmt.Sprintf(`PRAGMA user_version=%d`, tt.version)); err != nil {
				_ = db.Close()
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}

			store, err := Open(path)
			if err != nil {
				t.Fatalf("migrate: %v", err)
			}
			defer store.Close()
			if err := store.Ping(context.Background()); err != nil {
				t.Fatalf("migrated store not healthy: %v", err)
			}
			u, err := store.GetUser("legacy")
			if err != nil || u.Username != "legacy" {
				t.Fatalf("legacy row lost: %+v %v", u, err)
			}
			queued, err := store.ListQueuedSubmissions()
			if err != nil || len(queued) != 1 || queued[0].ID != "legacy-sub" {
				t.Fatalf("queued verdict was not backfilled: %+v %v", queued, err)
			}
		})
	}
}

func TestOpenRejectsUnknownSchemaVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "future.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA user_version=99`); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); err == nil || !strings.Contains(err.Error(), "unsupported sqlite schema version") {
		t.Fatalf("future schema must fail closed, got %v", err)
	}
}

func TestPingDetectsSchemaDriftAndClosedDatabase(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "forge.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Ping(context.Background()); err != nil {
		t.Fatalf("fresh store should be healthy: %v", err)
	}
	if _, err := s.db.Exec(`PRAGMA user_version=99`); err != nil {
		t.Fatal(err)
	}
	if err := s.Ping(context.Background()); err == nil {
		t.Fatal("schema drift must make readiness fail")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Ping(context.Background()); err == nil {
		t.Fatal("closed database must make readiness fail")
	}
}
