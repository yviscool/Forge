//go:build linux

package judge

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/yviscool/forge/internal/domain"
)

// cgroup v2 资源墙:探测通过时确定性 OOM→MLE 与正常判题;
// 探测不可用(CI 未委任 user slice 等)全部跳过,prlimit-only 照常。

func requireCgroup(t *testing.T) {
	t.Helper()
	if !probeCgroup() {
		t.Skipf("cgroup v2 wall unavailable: %s", CgroupStatus())
	}
}

// TestCgroupWallNormal 挂上 cgroup 墙后普通判题不受影响。
func TestCgroupWallNormal(t *testing.T) {
	requireCgroup(t)
	requireTool(t, "g++")
	o := &Orchestrator{Compiler: Toolchain{}, Runner: LocalRunner{}}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	oc, err := o.JudgeOne(ctx, Request{
		Language: "cpp", Code: fixture(t, "aplusb.cpp"),
		Cases:   []CaseInput{{Input: "3 4\n", Expected: "7"}},
		Compare: domain.CompareIgnoreSpace,
		Limits:  Limits{TimeMs: 2000, MemoryMiB: 64},
	})
	if err != nil {
		t.Fatal(err)
	}
	if oc.Cases[0].Verdict != domain.CaseAC {
		t.Fatalf("cgroup wall must not disturb normal judging: %+v msg=%q", oc.Cases[0], oc.Cases[0].Message)
	}
}

// TestCgroupWallOOM 内存超 memory.max → 内核 OOM kill → 确定性 MLE。
// 直接对原始子进程挂墙(不经 runner),绕开 prlimit AS 墙的干扰,
// 单测 cgroup 判定本身。
func TestCgroupWallOOM(t *testing.T) {
	requireCgroup(t)
	requireTool(t, "g++")
	work := t.TempDir()
	tc := Toolchain{}
	comp, err := tc.Compile(context.Background(), "cpp",
		[]byte("#include <cstring>\nint main(){char* p = new char[200<<20]; memset(p, 1, 200<<20); return *(p+(100<<20)) & 1;}"),
		work, CompileOptions{})
	if err != nil || !comp.OK {
		t.Fatalf("compile: %v %+v", err, comp)
	}
	lim, err := newJobLimiter(64, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := lim.(*cgroupLimiter); !ok {
		t.Skip("cgroup wall not engaged (probe available but limiter fell back)")
	}
	cmd := exec.Command(comp.Argv()[0])
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if err := lim.Attach(cmd.Process.Pid); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal(err)
	}
	defer lim.Close()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if lim.OutOfMemory() {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return
		}
		if ps := cmd.ProcessState; ps != nil {
			t.Fatalf("hog exited before OOM: %+v", ps)
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = os.RemoveAll(filepath.Join(work, "main.exe"))
	t.Fatal("cgroup OOM did not fire within 15s")
}
