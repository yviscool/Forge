package judge

import (
	"bytes"
	"context"
	"os/exec"
	"time"
)

// OutputCap 单点 stdout 上限（防爆输出拖死 worker）。
const OutputCap = 16 << 20

// RunResult 单点运行结果。
type RunResult struct {
	ExitCode    int
	TimedOut    bool
	OutOfMemory bool
	Stdout      string
	Stderr      string
	Truncated   bool
	TimeMs      int
	PeakKiB     int
}

// Runner 运行器抽象（可 fake 单测）。
type Runner interface {
	Run(ctx context.Context, argv []string, input string, lim Limits, workdir string) (RunResult, error)
}

// LocalRunner 本机运行器：超时 kill + 输出上限 + 平台内存墙（见 job_*.go）。
type LocalRunner struct{}

func (LocalRunner) Run(ctx context.Context, argv []string, input string, lim Limits, workdir string) (RunResult, error) {
	var res RunResult
	if len(argv) == 0 {
		return res, errEmptyArgv
	}
	timeout := time.Duration(lim.TimeMs) * time.Millisecond
	if timeout <= 0 {
		timeout = time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = workdir
	cmd.Env = scrubEnv()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return res, err
	}
	outCap := &cappedWriter{cap: OutputCap}
	errCap := &cappedWriter{cap: 1 << 20}
	cmd.Stdout, cmd.Stderr = outCap, errCap

	limiter, err := newJobLimiter(lim.MemoryMiB)
	if err != nil {
		return res, err
	}
	defer limiter.Close()

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return res, err
	}
	// 启动后立刻纳入 Job：单进程程序无 Bourne-shell 间接层，窗口足够小。
	if err := limiter.Attach(cmd.Process.Pid); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return res, err
	}
	if _, err := stdin.Write([]byte(input)); err != nil {
		// 程序提前退出导致管道关闭：非致命，继续等退出码。
		_ = err
	}
	_ = stdin.Close()
	waitErr := cmd.Wait()
	elapsed := time.Since(start)

	res.TimeMs = int(elapsed / time.Millisecond)
	res.Stdout, res.Stderr = outCap.String(), errCap.String()
	res.Truncated = outCap.truncated || errCap.truncated
	res.PeakKiB = int(limiter.PeakBytes() >> 10)

	if ctx.Err() == context.DeadlineExceeded {
		res.TimedOut = true
		limiter.Terminate()
		return res, nil
	}
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			res.ExitCode = exitErr.ExitCode()
		} else {
			return res, waitErr
		}
	}
	// Job 内存墙杀掉的进程退出码不可靠：用峰值贴墙判定 MLE。
	if limitBytes(lim.MemoryMiB) > 0 && limiter.PeakBytes()*100 >= uint64(limitBytes(lim.MemoryMiB))*95 {
		res.OutOfMemory = true
	}
	return res, nil
}

func limitBytes(mib int) int64 {
	if mib <= 0 {
		return 0
	}
	return int64(mib) << 20
}

type runnerErr string

func (e runnerErr) Error() string { return string(e) }

const errEmptyArgv runnerErr = "empty argv"

// cappedWriter 超上限丢弃并标记（防 zip-bomb 式输出）。
type cappedWriter struct {
	cap       int
	buf       bytes.Buffer
	truncated bool
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	room := w.cap - w.buf.Len()
	if room <= 0 {
		w.truncated = true
		return len(p), nil
	}
	if len(p) > room {
		w.buf.Write(p[:room])
		w.truncated = true
		return len(p), nil
	}
	return w.buf.Write(p)
}

func (w *cappedWriter) String() string { return w.buf.String() }
