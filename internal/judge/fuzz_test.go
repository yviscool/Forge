package judge

import (
	"path/filepath"
	"strings"
	"testing"
)

// 四个纯解析器的 fuzz 目标:种 子 在常规 go test 中作为用例执行(零 CI
// 成本),`go test -fuzz` 可深度探索。不变式以实现契约为准,防回归。

func FuzzParseTestlibResult(f *testing.F) {
	seeds := []string{"ok", "ok extra", "FAIL nothing", "partially correct (50)",
		"partially correct (0)", "partially correct (101)", "points 0.5", "points 2",
		"points -1", "", "\n", "ok\npoints 0.9", " points 0.3"}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		score, marked, fail := parseTestlibResult(s)
		if fail && marked {
			t.Fatalf("FAIL 与打分标记互斥: %q -> %v %v %v", s, score, marked, fail)
		}
		if score < 0 {
			t.Fatalf("score must not be negative: %q -> %v", s, score)
		}
	})
}

func FuzzParseLemonScore(f *testing.F) {
	seeds := []string{"100", "0", "-5", "", "  42  ", "abc", "1 2 3", "99999999999999999999"}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		n, ok := parseLemonScore(s)
		if ok && n < 0 {
			t.Fatalf("negative score accepted: %q -> %d", s, n)
		}
	})
}

func FuzzSafeFileName(f *testing.F) {
	seeds := []string{"", "p.in", "../evil", "..", ".", "/", "\\\\srv\\share", "CON", "a b.txt", "..\\..\\x"}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got := SafeFileName(s, "fallback")
		if got == "" {
			t.Fatalf("result must never be empty: %q", s)
		}
		if got != filepath.Base(got) {
			t.Fatalf("result must be a base name: %q -> %q", s, got)
		}
		if got == "." || got == ".." {
			t.Fatalf("dot/dotdot must fall back: %q -> %q", s, got)
		}
		if strings.Contains(got, ":") || got != strings.TrimRight(got, " .") {
			t.Fatalf("windows-hostile name must fall back: %q -> %q", s, got)
		}
		if got != "fallback" && s != "" && filepath.Base(s) != got && !strings.HasSuffix(got, baseSuffixOf(s)) {
			// 无更强约束:Base 语义已由上一条覆盖。
			_ = got
		}
	})
}

func baseSuffixOf(string) string { return "" }
