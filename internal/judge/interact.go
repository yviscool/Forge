package judge

import (
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
	Verdict   domain.CaseVerdict
	Message   string
	TimeMs    int
	MemoryKiB int
	CpuMs     int
	// Score 部分分：interactor stderr 打分协议给出 0..full 时非零。
	Score int
}

// RunInteraction 运行交互式测试点：
// solution.stdin <- interactor.stdout，interactor.stdin <- solution.stdout。
// 协议（testlib interactor 习惯 + Forge 扩展；上游 LemonLime 的交互是
// 编译期耦合进可执行文件，无可对标）：
//   - 选手崩溃 → RE；interactor 编译失败/异常退出 → CheckerError
//     （评测器侧故障，不误报学生 RE）；
//   - interactor exit 1/2 → WA；
//   - exit 0 且 stderr 含 testlib 打分标记（ok / partially correct (N) /
//     points X）→ 按标记给满分/部分分；无标记 → AC。
//
// 输入 input 以 argv[1] 传给 interactor（文件路径形式）。
func RunInteraction(ctx context.Context, comp Compiler, inter InteractorSpec, solArgv []string, input string, lim Limits, workdir string, fullScore int) (InteractResult, error) {
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
	ic, err := comp.Compile(ctx, inter.Language, []byte(inter.Code), dir, CompileOptions{})
	if err != nil {
		return res, err
	}
	if !ic.OK {
		res.Verdict = domain.CaseCheckerError
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
	solCmd.SysProcAttr = sysProcAttr()
	solCmd.WaitDelay = 5 * time.Second
	solCmd.Stdin = interR
	solCmd.Stdout = solW
	solErrCap := newCapped(1 << 20)
	solCmd.Stderr = solErrCap

	// 交互器按受控辅助进程对待：内存墙 + 最小环境（教师源码仍不裸跑）；
	// 解释型交互器剥掉地址空间墙避免误杀。
	interArgv := confineArgv(ic.Argv(), Limits{MemoryMiB: helperWallMiB, Interpreted: ic.Interpreted}, "")
	interArgv = append(interArgv[:len(interArgv):len(interArgv)], inF)
	interCmd := exec.CommandContext(ctx, interArgv[0], interArgv[1:]...)
	interCmd.Dir = workdir
	interCmd.Env = scrubEnv()
	interCmd.SysProcAttr = sysProcAttr()
	interCmd.WaitDelay = 5 * time.Second
	interCmd.Stdin = solR
	interCmd.Stdout = interW
	interErrCap := newCapped(1 << 20)
	interCmd.Stderr = interErrCap

	// Windows：双进程共享一个 Job（合并内存墙 + ActiveProcessLimit=2 防 fork）。
	limiter, err := newJobLimiter(lim.MemoryMiB, 2)
	if err != nil {
		return res, err
	}
	defer limiter.Close()

	if err := solCmd.Start(); err != nil {
		return res, err
	}
	if err := interCmd.Start(); err != nil {
		_ = solCmd.Process.Kill()
		_ = solCmd.Wait()
		return res, err
	}
	// 父进程必须立刻关掉自己持有的两个写端：否则任一子进程退出后，
	// 对端永远等不到 EOF，选手程序会挂到超时（曾使交互器崩溃被误判 TLE）。
	_ = solW.Close()
	_ = interW.Close()
	stopSol := watchGroupKill(ctx, solCmd.Process.Pid)
	stopInter := watchGroupKill(ctx, interCmd.Process.Pid)
	defer stopSol()
	defer stopInter()
	if err := limiter.Attach(solCmd.Process.Pid); err != nil {
		killTree(solCmd.Process.Pid)
		killTree(interCmd.Process.Pid)
		_ = solCmd.Wait()
		_ = interCmd.Wait()
		return res, err
	}
	if err := limiter.Attach(interCmd.Process.Pid); err != nil {
		killTree(solCmd.Process.Pid)
		killTree(interCmd.Process.Pid)
		_ = solCmd.Wait()
		_ = interCmd.Wait()
		return res, err
	}

	// WaitDelay 触发时进程已退出、仅孤儿子进程仍握管道——按正常收割处理。
	solWaitErr := normalizeWaitErr(solCmd.Wait())
	interErr := normalizeWaitErr(interCmd.Wait())
	res.TimeMs = int(time.Since(start) / time.Millisecond)
	if ps := solCmd.ProcessState; ps != nil {
		res.CpuMs = int((ps.UserTime() + ps.SystemTime()) / time.Millisecond)
	}
	res.MemoryKiB = interactPeakKiB(limiter, solCmd.ProcessState, interCmd.ProcessState)

	if ctx.Err() == context.DeadlineExceeded {
		limiter.Terminate()
		res.Verdict = domain.CaseTLE
		return res, nil
	}
	// 交互器违反自身协议（退出码非 0/1/2 或无法收割）：评测器侧故障优先
	// 归 CheckerError——选手是在一次被污染的交互里陪跑的，不应误报 RE。
	if interErr != nil {
		ee, ok := interErr.(*exec.ExitError)
		if !ok {
			res.Verdict = domain.CaseCheckerError
			res.Message = "interactor error: " + interErr.Error()
			return res, nil
		}
		switch ee.ExitCode() {
		case 0:
			// 罕见：exit 0 但 Wait 报错（平台信号），继续按 stderr 解析。
		case 1, 2:
			// 交互器判 WA，但选手若先崩仍以 RE 为准（更有诊断价值）。
			if solWaitErr == nil {
				res.Verdict = domain.CaseWA
				res.Message = stderrTail(interErrCap.String())
				return res, nil
			}
		default:
			res.Verdict = domain.CaseCheckerError
			res.Message = stderrTail(interErrCap.String())
			return res, nil
		}
	}
	// 选手程序先崩：RE（不再信任交互器结论）。
	if solWaitErr != nil {
		res.Verdict = domain.CaseRE
		res.Message = stderrTail(solErrCap.String())
		return res, nil
	}
	// exit 0：解析 stderr 打分标记（ok / partially correct (N) / points X）。
	if fullScore <= 0 {
		fullScore = 100
	}
	score, marked, fail := parseTestlibResult(interErrCap.String())
	if fail {
		res.Verdict = domain.CaseCheckerError
		res.Message = stderrTail(interErrCap.String())
		return res, nil
	}
	if marked {
		res.Score = int(score*float64(fullScore) + 0.5)
		if res.Score < 0 {
			res.Score = 0
		}
		if res.Score > fullScore {
			res.Score = fullScore
		}
		res.Verdict = verdictForScore(res.Score, fullScore)
		res.Message = stderrTail(interErrCap.String())
		return res, nil
	}
	res.Verdict = domain.CaseAC
	return res, nil
}

// interactPeakKiB 交互双方峰值内存取 max（Windows 用 Job 汇总，Unix 用 rusage）。
func interactPeakKiB(limiter unixLimiter, states ...*os.ProcessState) int {
	peak := limiter.PeakBytes()
	for _, ps := range states {
		if b := processPeakBytes(ps); b > peak {
			peak = b
		}
	}
	return int(peak >> 10)
}
