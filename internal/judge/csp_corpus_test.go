package judge

// CSP 集训第一天四题 corpus：40 个测试点 × 学生变体矩阵。
// 变体命名：ac=参考解，wa_*=典型错误算法，slow=正确但超时，re/ce=崩溃/编译错。

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yviscool/forge/internal/domain"
)

const cspRoot = "testdata/csp-day1"

func loadCode(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(cspRoot, "solutions", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// loadCases 读取 prefix01..prefixNN 的 .in/.out。
func loadCases(t *testing.T, dir, prefix string, idx []int) []CaseInput {
	t.Helper()
	var out []CaseInput
	for _, i := range idx {
		base := filepath.Join(cspRoot, dir, fmt.Sprintf("%s%02d", prefix, i))
		in, err := os.ReadFile(base + ".in")
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(base + ".out")
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, CaseInput{Input: string(in), Expected: string(want)})
	}
	return out
}

func allIdx(n int) []int {
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i + 1
	}
	return idx
}

func runVariant(t *testing.T, lang, code string, cases []CaseInput, lim Limits) []domain.CaseVerdict {
	t.Helper()
	o := realOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	oc, err := o.JudgeOne(ctx, Request{
		Language: lang, Code: code, Cases: cases,
		Compare: domain.CompareIgnoreSpace, Limits: lim,
	})
	got := oc.Cases
	if err != nil {
		t.Fatal(err)
	}
	v := make([]domain.CaseVerdict, len(got))
	for i, c := range got {
		v[i] = c.Verdict
	}
	return v
}

func wantAll(v domain.CaseVerdict, n int) []domain.CaseVerdict {
	w := make([]domain.CaseVerdict, n)
	for i := range w {
		w[i] = v
	}
	return w
}

func assertVerdicts(t *testing.T, variant string, got, want []domain.CaseVerdict) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: len %d != %d", variant, len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s: case %d got %s want %s (full=%v)", variant, i+1, got[i], want[i], got)
		}
	}
}

func stdLimits() Limits { return Limits{TimeMs: 2000, MemoryMiB: 256} }
