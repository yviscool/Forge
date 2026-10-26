//go:build linux

package judge

import (
	"context"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestKillTreeReapsGrandchild 硬墙超时后，fork 出的孙进程必须随进程组
// 一起被收割（此前 Unix 只杀直接子进程，孙进程可残留到 ctx 之外）。
func TestKillTreeReapsGrandchild(t *testing.T) {
	requireTool(t, "g++")
	work := t.TempDir()
	tc := Toolchain{}
	comp, err := tc.Compile(context.Background(), "cpp", fixture(t, "killtree.cpp"), work, CompileOptions{})
	if err != nil || !comp.OK {
		t.Fatalf("compile: %v %+v", err, comp)
	}
	run, err := LocalRunner{}.Run(context.Background(), comp.Argv(), "",
		Limits{TimeMs: 300, MemoryMiB: 256}, work)
	if err != nil {
		t.Fatal(err)
	}
	if !run.TimedOut {
		t.Fatalf("infinite loop must hit the hard wall: %+v", run)
	}
	pid, perr := strconv.Atoi(strings.TrimSpace(run.Stdout))
	if perr != nil || pid <= 0 {
		t.Fatalf("grandchild pid not reported: %q %v", run.Stdout, perr)
	}
	// 子进程必须已死：kill(pid,0) 返回 ESRCH。
	deadline := time.Now().Add(2 * time.Second)
	for {
		if err := syscall.Kill(pid, 0); err != nil {
			if err == syscall.ESRCH {
				return
			}
			t.Fatalf("signal probe failed: %v", err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("grandchild %d outlived the judge run", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
