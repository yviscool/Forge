package judge

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/yviscool/forge/internal/domain"
)

// InteractorSpec 交互器源码（随题目下发）。
type InteractorSpec struct {
	Language string
	Code     string
}

// InteractResult 一次交互评测结果。
type InteractResult struct {
	Verdict domain.CaseVerdict
	Message string
	TimeMs  int
}

// RunInteraction 运行交互式测试点：
// solution.stdin <- interactor.stdout，interactor.stdin <- solution.stdout。
// interactor exit 0 = AC；exit 1/2 = WA；其他 = RE；超时 = TLE。
// 输入 input 作为 argv[1] 传给 interactor（文件路径形式，testlib 习惯）。
func RunInteraction(ctx context.Context, comp Compiler, inter InteractorSpec, solArgv []string, input string, lim Limits, workdir string) (InteractResult, error) {
	var res InteractResult
	if lim.TimeMs <= 0 {
		lim.TimeMs = 1000
	}
	if lim.MemoryMiB <= 0 {
		lim.MemoryMiB = 512
	}

	dir, err := os.MkdirTemp(workdir, "inter_*")
	if err != nil {
		return res, err
	}
	inF := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(inF, []byte(input), 0644); err != nil {
		return res, err
	}
	// 注意：编译必须用长 ctx（Toolchain 内部编译超时），绝不能套用例运行
	// 时限——g++ 编译 bits 头约 3s，比常见运行限还长（曾因此静默 exit 1）。
	ic, err := comp.Compile(ctx, inter.Language, []byte(inter.Code), dir)
	if err != nil {
		return res, err
	}
	if !ic.OK {
		res.Verdict = domain.CaseRE
		res.Message = "interactor compile failed: " + ic.Message
		return res, nil
	}

	// 运行阶段再套用例时限。
	ctx, cancel := context.WithTimeout(ctx, time.Duration(lim.TimeMs)*time.Millisecond)
	defer cancel()

	// 双向管道：sol.stdout -> inter.stdin，inter.stdout -> sol.stdin。
	solR, solW, err := os.Pipe()
	if err != nil {
		return res, err
	}
	defer solR.Close()
	defer solW.Close()
	interR, interW, err := os.Pipe()
	if err != nil {
		return res, err
	}
	defer interR.Close()
	defer interW.Close()

	start := time.Now()
	solCmd := exec.CommandContext(ctx, solArgv[0], solArgv[1:]...)
	solCmd.Dir = workdir
	solCmd.Env = scrubEnv()
	solCmd.Stdin = interR
	solCmd.Stdout = solW
	var solStderr bytes.Buffer
	solCmd.Stderr = &solStderr

	interCmd := exec.CommandContext(ctx, ic.Argv()[0], append(append([]string{}, ic.Argv()[1:]...), inF)...)
	interCmd.Dir = workdir
	interCmd.Env = scrubEnv()
	interCmd.Stdin = solR
	// 交互器 stdout 直通选手 stdin（管道独占，不再截获；结论只看退出码）。
	interCmd.Stdout = interW
	var interStderr bytes.Buffer
	interCmd.Stderr = &interStderr

	if err := solCmd.Start(); err != nil {
		return res, err
	}
	if err := interCmd.Start(); err != nil {
		_ = solCmd.Process.Kill()
		_ = solCmd.Wait()
		return res, err
	}
	solWaitErr := solCmd.Wait()
	interErr := interCmd.Wait()
	res.TimeMs = int(time.Since(start) / time.Millisecond)

	if ctx.Err() == context.DeadlineExceeded {
		res.Verdict = domain.CaseTLE
		return res, nil
	}
	// 选手程序先崩：直接 RE（不再信任交互器结论）。
	if solWaitErr != nil {
		res.Verdict = domain.CaseRE
		res.Message = solStderr.String()
		return res, nil
	}
	if interErr != nil {
		if ee, ok := interErr.(*exec.ExitError); ok && (ee.ExitCode() == 1 || ee.ExitCode() == 2) {
			res.Verdict = domain.CaseWA
			res.Message = interStderr.String()
			return res, nil
		}
		res.Verdict = domain.CaseRE
		res.Message = interStderr.String()
		return res, nil
	}
	res.Verdict = domain.CaseAC
	return res, nil
}
