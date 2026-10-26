package judge

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"sync"
	"time"
)

// helperWallMiB 特判/交互器等受控辅助进程的内存墙（教师下发源码仍按不可信
// 进程对待；解释型运行时经 effectiveLim 剥掉地址空间墙，避免误杀）。
const helperWallMiB = 1024

// runConfined 以最小环境 + 资源墙运行受控辅助进程（特判/交互器/diff）。
// 对标 LemonLime 对 SPJ 的独立时限管理：超时由调用方 ctx 控制，这里负责
// scrubEnv、内存墙与组杀兜底（此前 checker/interactor 是裸 exec，连环境
// 变量都没剥离——评测面上最裸露的洞）。
func runConfined(ctx context.Context, argv []string, dir string, interpreted bool, stdout, stderr io.Writer) error {
	if len(argv) == 0 {
		return errEmptyArgv
	}
	argv = confineArgv(argv, Limits{MemoryMiB: helperWallMiB, Interpreted: interpreted}, "")
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = scrubEnv()
	cmd.Stdout, cmd.Stderr = stdout, stderr
	cmd.SysProcAttr = sysProcAttr()
	// 与 runner 同债:checker 退出后若留握管道的孙进程,Wait 会阻塞到
	// 它自然退出;WaitDelay 5s 放行,孤儿交组杀/Job 清算。
	cmd.WaitDelay = 5 * time.Second
	limiter, err := newJobLimiter(helperWallMiB, 1)
	if err != nil {
		return err
	}
	defer limiter.Close()
	if err := cmd.Start(); err != nil {
		return err
	}
	stop := watchGroupKill(ctx, cmd.Process.Pid)
	defer stop()
	if err := limiter.Attach(cmd.Process.Pid); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return err
	}
	return normalizeWaitErr(cmd.Wait())
}

// normalizeWaitErr WaitDelay 触发时进程已正常收割,仅孤儿仍握管道——
// 结果有效,按正常结束处理。
func normalizeWaitErr(err error) error {
	if err != nil && errors.Is(err, exec.ErrWaitDelay) {
		return nil
	}
	return err
}

// watchGroupKill ctx 取消时对进程组补刀：Go 默认只杀直接子进程，prlimit /
// bwrap 的 exec 链与 fork 出的孙进程需要组杀兜底（Windows 由 Job Object
// 覆盖，killTree 为 no-op）。返回停表函数，Wait 返回后必须调用。
func watchGroupKill(ctx context.Context, pid int) func() {
	if pid <= 0 {
		return func() {}
	}
	done := make(chan struct{})
	var once sync.Once
	go func() {
		select {
		case <-ctx.Done():
			killTree(pid)
		case <-done:
		}
	}()
	return func() { once.Do(func() { close(done) }) }
}

// newCapped 构造限额缓冲（checker/interactor stderr 截获用）。
func newCapped(n int) *cappedWriter { return &cappedWriter{cap: n} }
