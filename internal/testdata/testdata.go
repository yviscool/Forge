// Package testdata 测试数据文件仓：每题独立目录，.in/.out 成对管理。
// 对标 LemonLime exttestcase 的文件式数据管理；resolveIO 按路径读取。
package testdata

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MaxFileSize 单文件上限 10MB。
const MaxFileSize = 10 << 20

type Store struct {
	root string
}

func New(root string) *Store { return &Store{root: root} }

func (s *Store) dir(cid, pid string) string {
	return filepath.Join(s.root, cid, pid)
}

func clean(name string) (string, error) {
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return "", errors.New("invalid file name")
	}
	return name, nil
}

// Put 写入测试文件（覆盖同名）。
func (s *Store) Put(cid, pid, name string, content []byte) error {
	name, err := clean(name)
	if err != nil {
		return err
	}
	if len(content) > MaxFileSize {
		return errors.New("file too large (max 10MB)")
	}
	if err := os.MkdirAll(s.dir(cid, pid), 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir(cid, pid), name), content, 0644)
}

// Get 读取测试文件。
func (s *Store) Get(cid, pid, name string) ([]byte, error) {
	name, err := clean(name)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(s.dir(cid, pid), name))
}

// Delete 删除测试文件。
func (s *Store) Delete(cid, pid, name string) error {
	name, err := clean(name)
	if err != nil {
		return err
	}
	return os.Remove(filepath.Join(s.dir(cid, pid), name))
}

// List 列出测试文件名。
func (s *Store) List(cid, pid string) ([]string, error) {
	entries, err := os.ReadDir(s.dir(cid, pid))
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
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
	name, err := clean(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.dir(cid, pid), name), nil
}
