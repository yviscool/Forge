//go:build darwin

package judge

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yviscool/forge/internal/domain"
)

// TestDarwinClangCorpus macOS 真实编译判题抽样:clang + ulimit 降级墙 +
// rusage 峰值全链激活(bits/stdc++.h 在 libc++ 缺失且 clang 编译过慢,
// 重 corpus 留给 linux/windows,此处用纯 <iostream> 语料)。
func TestDarwinClangCorpus(t *testing.T) {
	requireTool(t, "g++")
	o := &Orchestrator{Compiler: Toolchain{}, Runner: LocalRunner{}}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	oc, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: darwinFixture(t, "ac.cpp"),
		Cases:   []CaseInput{{Input: "3 4\n", Expected: "7"}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 2000, MemoryMiB: 256},
	})
	if err != nil {
		t.Fatal(err)
	}
	if oc.Cases[0].Verdict != domain.CaseAC {
		t.Fatalf("darwin ac: %+v compile=%q", oc.Cases[0], oc.CompileMessage)
	}
	oc, err = o.JudgeOne(ctx, Request{
		Language: "cpp", Code: darwinFixture(t, "wa.cpp"),
		Cases:   []CaseInput{{Input: "3 4\n", Expected: "7"}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 2000, MemoryMiB: 256},
	})
	if err != nil {
		t.Fatal(err)
	}
	if oc.Cases[0].Verdict != domain.CaseWA {
		t.Fatalf("darwin wa: %+v", oc.Cases[0])
	}
	start := time.Now()
	oc, err = o.JudgeOne(ctx, Request{
		Language: "cpp", Code: darwinFixture(t, "tle.cpp"),
		Cases:   []CaseInput{{Input: "", Expected: ""}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 500, MemoryMiB: 256},
	})
	if err != nil {
		t.Fatal(err)
	}
	if oc.Cases[0].Verdict != domain.CaseTLE {
		t.Fatalf("darwin tle: %+v", oc.Cases[0])
	}
	if elapsed := time.Since(start); elapsed > 20*time.Second {
		t.Fatalf("darwin tle kill took too long: %v", elapsed)
	}
}

func darwinFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "darwin", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
