//go:build windows

package judge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yviscool/forge/internal/domain"
)

// Windows 低权沙箱(LowBox)端到端:机制可用、越界读拒绝、网络拒绝、
// 工作目录正常读写、关闭模式回落 Job-only。

func requireWinSandbox(t *testing.T) {
	t.Helper()
	ConfigureWinSandbox("auto")
	ensureWinSandboxProbed(60 * time.Second)
	if !winSandboxUsable.Load() {
		t.Skipf("win sandbox unavailable: %s", WinSandboxStatus())
	}
}

func sandboxOrchestrator() *Orchestrator {
	return &Orchestrator{Compiler: Toolchain{}, Runner: LocalRunner{}}
}

// TestWinSandboxProbeMechanism 探测必须自证:容器内可运行、工作目录可写、
// 越界读被拒。
func TestWinSandboxProbeMechanism(t *testing.T) {
	requireWinSandbox(t)
	if got := WinSandboxStatus(); got != "appcontainer" {
		t.Fatalf("status = %q", got)
	}
}

// TestIntegrationSandbox_AC_CPP 沙箱内编译产物照常运行(MinGW 运行时
// DLL 经运行时目录只读 ACL 可达)。
func TestIntegrationSandbox_AC_CPP(t *testing.T) {
	requireWinSandbox(t)
	requireTool(t, "g++")
	o := sandboxOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	oc, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: fixture(t, "aplusb.cpp"),
		Cases:   []CaseInput{{Input: "3 4\n", Expected: "7"}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 2000, MemoryMiB: 256},
	})
	if err != nil {
		t.Fatal(err)
	}
	if oc.Cases[0].Verdict != domain.CaseAC {
		t.Fatalf("sandboxed AC: %+v msg=%q compile=%q", oc.Cases[0], oc.Cases[0].Message, oc.CompileMessage)
	}
}

// TestIntegrationSandbox_FSDenied 沙箱内读用户目录哨兵必须失败
// (文件系统隔离证据)。程序经 stdin 拿路径,fopen 判定可达性。
func TestIntegrationSandbox_FSDenied(t *testing.T) {
	requireWinSandbox(t)
	requireTool(t, "g++")
	home := os.Getenv("USERPROFILE")
	if home == "" {
		t.Skip("no USERPROFILE")
	}
	canary := filepath.Join(home, "forge_sandbox_canary.txt")
	if err := os.WriteFile(canary, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(canary)

	code := strings.Join([]string{
		"#include <cstdio>",
		"int main(){",
		"  char p[600];",
		"  if(!(std::scanf(\"%599s\", p))) return 2;",
		"  FILE* f = std::fopen(p, \"r\");",
		"  std::printf(f ? \"read-ok\" : \"read-denied\");",
		"  return 0;",
		"}",
	}, "\n")
	o := sandboxOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	oc, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: code,
		Cases:   []CaseInput{{Input: canary + "\n", Expected: "read-denied"}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 5000, MemoryMiB: 256},
	})
	if err != nil {
		t.Fatal(err)
	}
	if oc.Cases[0].Verdict != domain.CaseAC {
		t.Fatalf("fs isolation: expected program to observe denial (AC on read-denied), got %+v msg=%q",
			oc.Cases[0], oc.Cases[0].Message)
	}
}

// TestIntegrationSandbox_NetDenied 沙箱内无 capability,套接字必须被拒
// (网络隔离证据):程序内调 system("ping")——ping 走真实 socket,
// LowBox 下 connect 得 WSAEACCES,ping 失败 -> net-denied。
func TestIntegrationSandbox_NetDenied(t *testing.T) {
	requireWinSandbox(t)
	requireTool(t, "g++")
	code := strings.Join([]string{
		"#include <cstdio>",
		"#include <cstdlib>",
		"int main(){",
		"  int rc = std::system(\"ping -n 1 -w 500 127.0.0.1 > nul\");",
		"  std::printf(rc == 0 ? \"net-ok\" : \"net-denied\");",
		"  return 0;",
		"}",
	}, "\n")
	o := sandboxOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	oc, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: code,
		Cases:   []CaseInput{{Input: "", Expected: "net-denied"}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 10000, MemoryMiB: 256},
	})
	if err != nil {
		t.Fatal(err)
	}
	if oc.Cases[0].Verdict != domain.CaseAC {
		t.Fatalf("net isolation: expected net-denied AC, got %+v msg=%q compile=%q",
			oc.Cases[0], oc.Cases[0].Message, oc.CompileMessage)
	}
}

// TestIntegrationSandbox_Python 解释型经前缀 ACL 后照常判题。
func TestIntegrationSandbox_AC_Python(t *testing.T) {
	requireWinSandbox(t)
	requireTool(t, "python")
	o := sandboxOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	oc, err := o.JudgeOne(ctx, Request{
		Language: "python", Code: fixture(t, "aplusb.py"),
		Cases:   []CaseInput{{Input: "3 4", Expected: "7"}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 5000, MemoryMiB: 256},
	})
	if err != nil {
		t.Fatal(err)
	}
	if oc.Cases[0].Verdict != domain.CaseAC {
		t.Fatalf("sandboxed python AC: %+v compile=%q", oc.Cases[0], oc.CompileMessage)
	}
}

// TestSandboxOffFallsBackToJobOnly 关闭模式必须保持原 Job-only 路径可用。
func TestSandboxOffFallsBackToJobOnly(t *testing.T) {
	ConfigureWinSandbox("off")
	defer ConfigureWinSandbox("auto")
	requireTool(t, "g++")
	o := sandboxOrchestrator()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	oc, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: fixture(t, "aplusb.cpp"),
		Cases:   []CaseInput{{Input: "3 4\n", Expected: "7"}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 2000, MemoryMiB: 256},
	})
	if err != nil {
		t.Fatal(err)
	}
	if oc.Cases[0].Verdict != domain.CaseAC {
		t.Fatalf("job-only fallback: %+v", oc.Cases[0])
	}
}
