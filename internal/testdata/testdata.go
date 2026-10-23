// Package testdata 测试数据文件仓：每题独立目录，.in/.out 成对管理。
// 对标 LemonLime exttestcase 的文件式数据管理；resolveIO 按路径读取。
package testdata

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// MaxFileSize 单文件上限 10MB。
const MaxFileSize = 10 << 20

type Store struct {
	root string
}

func New(root string) *Store {
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	return &Store{root: filepath.Clean(root)}
}

// Root returns the configured storage root (used by transport tests and
// operational diagnostics; file access still goes through the validated API).
func (s *Store) Root() string { return s.root }

func safeComponent(name, label string) error {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." ||
		strings.ContainsAny(name, "/\\\x00") ||
		filepath.Base(name) != name {
		return fmt.Errorf("invalid %s", label)
	}
	return nil
}

func (s *Store) relDir(cid, pid string) (string, error) {
	if err := safeComponent(cid, "contest id"); err != nil {
		return "", err
	}
	if err := safeComponent(pid, "problem id"); err != nil {
		return "", err
	}
	return filepath.Join(cid, pid), nil
}

func (s *Store) relFile(cid, pid, name string) (string, error) {
	dir, err := s.relDir(cid, pid)
	if err != nil {
		return "", err
	}
	name, err = clean(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

func (s *Store) openRoot() (*os.Root, error) {
	if strings.TrimSpace(s.root) == "" {
		return nil, errors.New("test data root is empty")
	}
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return nil, err
	}
	return os.OpenRoot(s.root)
}

// Health verifies that the configured test-data volume can create, sync, read,
// and remove a private file. Readiness must not report success for a read-only
// or missing data directory.
func (s *Store) Health(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	root, err := s.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	name := fmt.Sprintf(".forge-ready-%d", time.Now().UnixNano())
	f, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write([]byte("ok"))
	syncErr := f.Sync()
	closeErr := f.Close()
	removeErr := root.Remove(name)
	if writeErr != nil {
		return writeErr
	}
	if syncErr != nil {
		return syncErr
	}
	if closeErr != nil {
		return closeErr
	}
	if removeErr != nil {
		return removeErr
	}
	return nil
}

func clean(name string) (string, error) {
	if name == "" || name == "." || strings.Contains(name, "/") ||
		strings.Contains(name, "\\") || strings.Contains(name, "..") ||
		strings.ContainsRune(name, '\x00') || filepath.Base(name) != name {
		return "", errors.New("invalid file name")
	}
	return name, nil
}

// Put 写入测试文件（覆盖同名）。
func (s *Store) Put(cid, pid, name string, content []byte) error {
	rel, err := s.relFile(cid, pid, name)
	if err != nil {
		return err
	}
	if len(content) > MaxFileSize {
		return errors.New("file too large (max 10MB)")
	}
	root, err := s.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	dir, _ := s.relDir(cid, pid)
	if err := root.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if info, statErr := root.Lstat(rel); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		return errors.New("refusing to overwrite a symbolic link")
	}
	return root.WriteFile(rel, content, 0o600)
}

// Get 读取测试文件。
func (s *Store) Get(cid, pid, name string) ([]byte, error) {
	rel, err := s.relFile(cid, pid, name)
	if err != nil {
		return nil, err
	}
	root, err := s.openRoot()
	if err != nil {
		return nil, err
	}
	defer root.Close()
	f, err := root.Open(rel)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("test data entry is not a regular file")
	}
	if info.Size() > MaxFileSize {
		return nil, errors.New("file too large (max 10MB)")
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxFileSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxFileSize {
		return nil, errors.New("file too large (max 10MB)")
	}
	return data, nil
}

// Delete 删除测试文件。
func (s *Store) Delete(cid, pid, name string) error {
	rel, err := s.relFile(cid, pid, name)
	if err != nil {
		return err
	}
	root, err := s.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	return root.Remove(rel)
}

// List 列出测试文件名。
func (s *Store) List(cid, pid string) ([]string, error) {
	dir, err := s.relDir(cid, pid)
	if err != nil {
		return nil, err
	}
	root, err := s.openRoot()
	if err != nil {
		return nil, err
	}
	defer root.Close()
	handle, err := root.Open(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer handle.Close()
	entries, err := handle.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		info, err := e.Info()
		if err == nil && info.Mode().IsRegular() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

// Pairs 校验 .in 与 .out/.ans 成对情况，返回缺失配对的文件名。
// 对标 LemonLime 规范，.ans 与 .out 均作为标准测试答案扩展名。
func (s *Store) Pairs(cid, pid string) ([]string, error) {
	names, err := s.List(cid, pid)
	if err != nil {
		return nil, err
	}
	has := map[string]bool{}
	for _, n := range names {
		has[n] = true
	}
	var missing []string
	for _, n := range names {
		if strings.HasSuffix(n, ".in") {
			base := strings.TrimSuffix(n, ".in")
			if !has[base+".out"] && !has[base+".ans"] {
				missing = append(missing, n)
			}
		} else if strings.HasSuffix(n, ".out") {
			base := strings.TrimSuffix(n, ".out")
			if !has[base+".in"] {
				missing = append(missing, n)
			}
		} else if strings.HasSuffix(n, ".ans") {
			base := strings.TrimSuffix(n, ".ans")
			if !has[base+".in"] {
				missing = append(missing, n)
			}
		}
	}
	sort.Strings(missing)
	return missing, nil
}

// MatchedCase 成对测试点信息。
type MatchedCase struct {
	BaseName   string
	InputFile  string
	OutputFile string
}

// AnswerKeys lists answer-only task keys. Answer-only problems have no input
// stream: each .out/.ans file is one expected answer. .in files are ignored so
// a stale traditional-test file cannot make an answer-only paper look valid.
func (s *Store) AnswerKeys(cid, pid string) ([]MatchedCase, error) {
	names, err := s.List(cid, pid)
	if err != nil {
		return nil, err
	}
	var keys []MatchedCase
	for _, n := range names {
		var base string
		switch {
		case strings.HasSuffix(n, ".ans"):
			base = strings.TrimSuffix(n, ".ans")
		case strings.HasSuffix(n, ".out"):
			base = strings.TrimSuffix(n, ".out")
		default:
			continue
		}
		outPath, err := s.Path(cid, pid, n)
		if err != nil {
			return nil, err
		}
		keys = append(keys, MatchedCase{BaseName: base, OutputFile: outPath})
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].BaseName < keys[j].BaseName
	})
	return keys, nil
}

// MatchCases 自动识别已配对的测试点列表（兼容 .in + .out 或 .in + .ans）。
func (s *Store) MatchCases(cid, pid string) ([]MatchedCase, error) {
	names, err := s.List(cid, pid)
	if err != nil {
		return nil, err
	}
	has := map[string]bool{}
	for _, n := range names {
		has[n] = true
	}
	var matched []MatchedCase
	for _, n := range names {
		if strings.HasSuffix(n, ".in") {
			base := strings.TrimSuffix(n, ".in")
			outName := ""
			if has[base+".ans"] {
				outName = base + ".ans"
			} else if has[base+".out"] {
				outName = base + ".out"
			}
			if outName != "" {
				inPath, _ := s.Path(cid, pid, n)
				outPath, _ := s.Path(cid, pid, outName)
				matched = append(matched, MatchedCase{
					BaseName:   base,
					InputFile:  inPath,
					OutputFile: outPath,
				})
			}
		}
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].BaseName < matched[j].BaseName
	})
	return matched, nil
}

// Path 测试文件绝对路径（供 TestCase.InputFile 引用）。
func (s *Store) Path(cid, pid, name string) (string, error) {
	rel, err := s.relFile(cid, pid, name)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.root, rel), nil
}
