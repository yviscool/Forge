package judge

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/yviscool/forge/internal/domain"
)

// compareExternal 外部 diff 工具比较（对标 LemonLime compareWithDiff +
// ExternalToolMode）：选手输出与标准答案落盘后交给教师配置的 diff 命令，
// exit 0 = AC、1 = WA、其他 = CheckerError（工具故障不误判学生）。
func compareExternal(ctx context.Context, diff *DiffSpec, got, want, workdir string, res domain.CaseResult) domain.CaseResult {
	if diff == nil || diff.Command == "" {
		res.Verdict = domain.CaseCheckerError
		res.Message = "external tool compare requires diffCommand"
		return res
	}
	dir, err := os.MkdirTemp(workdir, "diff_*")
	if err != nil {
		res.Verdict = domain.CaseCheckerError
		res.Message = err.Error()
		return res
	}
	defer os.RemoveAll(dir)
	gotF := filepath.Join(dir, "output.txt")
	wantF := filepath.Join(dir, "answer.txt")
	if err := os.WriteFile(gotF, []byte(got), 0644); err != nil {
		res.Verdict = domain.CaseCheckerError
		res.Message = err.Error()
		return res
	}
	if err := os.WriteFile(wantF, []byte(want), 0644); err != nil {
		res.Verdict = domain.CaseCheckerError
		res.Message = err.Error()
		return res
	}
	args := diff.Args
	if args == nil {
		args = DefaultDiffArgs()
	}
	argv := append(append([]string{diff.Command}, args...), gotF, wantF)
	ctx, cancel := context.WithTimeout(ctx, checkerTimeout)
	defer cancel()
	out := newCapped(1 << 20)
	if err := runConfined(ctx, argv, dir, false, out, out); err != nil {
		res.Verdict = domain.CaseCheckerError
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			res.Verdict = domain.CaseWA
		}
		res.Message = truncateTail(out.String(), 1024)
		return res
	}
	res.Verdict = domain.CaseAC
	return res
}
