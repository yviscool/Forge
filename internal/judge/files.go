package judge

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

const maxJudgeDataFileBytes = 10 << 20

var testDataRoot atomic.Pointer[string]

// SetTestDataRoot establishes the only directory from which a test-case file
// path may be read. Absolute paths outside this root remain opaque literal
// values and are never opened.
func SetTestDataRoot(root string) error {
	root = strings.TrimSpace(root)
	if root == "" {
		testDataRoot.Store(nil)
		return nil
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(abs, 0o700); err != nil {
		return err
	}
	testDataRoot.Store(&abs)
	return nil
}

func resolveIO(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || !filepath.IsAbs(trimmed) || strings.ContainsRune(trimmed, '\x00') {
		return value
	}
	rootPath := testDataRoot.Load()
	if rootPath == nil {
		return value
	}
	root, err := os.OpenRoot(*rootPath)
	if err != nil {
		return value
	}
	defer root.Close()
	rel, err := filepath.Rel(*rootPath, trimmed)
	if err != nil || rel == "." || rel == ".." ||
		strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return value
	}
	f, err := root.Open(rel)
	if err != nil {
		return value
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxJudgeDataFileBytes {
		return value
	}
	data, err := io.ReadAll(io.LimitReader(f, maxJudgeDataFileBytes+1))
	if err != nil || len(data) > maxJudgeDataFileBytes {
		return value
	}
	return string(data)
}
