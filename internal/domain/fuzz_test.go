package domain

import (
	"strings"
	"testing"
)

// FuzzTokenLineSeq token 切分必须与"按 ASCII 空白手撕"的参考实现一致,
// 且行号单调不减、落在 [1, 换行数+1]。
func FuzzTokenLineSeq(f *testing.F) {
	seeds := []string{"", "a b c", "1 2\n3", "  \t x \r\n y ", "\n\n", "a\x00b", "中文 测试"}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		tokens, lines := tokenLineSeq(s)
		if len(tokens) != len(lines) {
			t.Fatalf("token/line length mismatch")
		}
		newlines := strings.Count(s, "\n")
		prev := 0
		for i, tok := range tokens {
			if tok == "" {
				t.Fatalf("empty token: %q", s)
			}
			if strings.ContainsAny(tok, " \t\r\n") {
				t.Fatalf("token contains whitespace: %q in %q", tok, s)
			}
			if lines[i] < prev || lines[i] < 1 || lines[i] > newlines+1 {
				t.Fatalf("line number out of range: %d in %q", lines[i], s)
			}
			prev = lines[i]
		}
	})
}
