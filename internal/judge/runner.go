package judge

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/yviscool/forge/internal/domain"
)

// OutputCap 缺省单点 stdout 上限（防爆输出拖死 worker），可被 Limits 覆盖。
const OutputCap = 16 << 20

// outputCapOf 取有效输出上限（字节）。
func outputCapOf(lim Limits) int {
	if lim.OutputLimitKiB > 0 {
		return lim.OutputLimitKiB << 10
	}
	return OutputCap
}

// RunResult 单点运行结果。
type RunResult struct {
	ExitCode    int
	TimedOut    bool
	OutOfMemory bool
	// NoOutput 文件 IO 题未产出输出文件（计 WA，非 runner 事故）。
	NoOutput  bool
	Stdout    string
	Stderr    string
	Truncated bool
	TimeMs    int
	PeakKiB   int
	// CpuMs 用户+系统 CPU 时间（并行程序 wall 低估时用它判 TLE）。
	CpuMs int
}

// unixLimiter 平台资源墙接口(Linux cgroup/prlimit、Windows Job 都实现)。
// Runner 在 Start 后 Attach,负责 PeakBytes/OutOfMemory 读取与 Terminate/Close。
type unixLimiter interface {
	Attach(pid int) error
	Terminate()
	PeakBytes() uint64
	OutOfMemory() bool
	Close()
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
	// 硬杀墙 = 时限 + 额外时间（对标 LemonLime extraTime = max(2000ms,
	// 2×时限)×比例）。软判定（记 TLE）在 judge.go 按 TimeMs/CpuMs 计算；
	// 此处只兜底杀进程，让边界抖动的程序留给上层一次重测机会。
	hardMs := lim.TimeMs
	if hardMs <= 0 {
		hardMs = 1000
	}
	timeout := time.Duration(hardMs+extraTimeMs(hardMs)) * time.Millisecond
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	argv = confineArgv(argv, lim, workdir)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = workdir
	cmd.Env = scrubEnv()
	cmd.SysProcAttr = sysProcAttr()
	// 孤儿管道死锁防线:选手退出但其子进程仍握住 stdout 时,Wait 会阻塞在
	// 管道排空上(Unix 组杀/Windows Job 虽能收割,但都排在 Wait 之后)。
	// WaitDelay 只需盖过孤儿收割的调度延迟:放行时间必须远小于常见用例
	// 时限,否则空等会污染 wall 计时、把毫秒级退出的正解顶成 TLE(曾以 5s
	// 放行在 CI 上把 pipeholder 顶过 5s 软限)。
	cmd.WaitDelay = 500 * time.Millisecond
	// Windows 低权沙箱(LowBox 令牌 + 工作目录 ACL):可用则整体替换进程
	// 属性并临时覆盖 TMP/TEMP 到工作目录;不可用(nil)按 Job-only 继续。
	if attr, envOver, cleanup, err := sandboxSetup(workdir); err == nil && attr != nil {
		cmd.SysProcAttr = attr
		cmd.Env = append(cmd.Env, envOver...)
		defer cleanup()
	}

	// 文件 IO 模式（OI 文件读写题）：输入落盘，输出从文件回读。
	// 文件名强制 Base 净化：配置来自题目数据，../ 不得逃逸工作目录。
	stdinData := input
	outFromFile := ""
	if lim.IOMode == domain.IOFile {
		inName, outName := SafeFileName(lim.InFile, "input.txt"), SafeFileName(lim.OutFile, "output.txt")
		_ = os.Remove(filepath.Join(workdir, outName))
		if err := os.WriteFile(filepath.Join(workdir, inName), []byte(input), 0644); err != nil {
			return res, err
		}
		stdinData = ""
		outFromFile = filepath.Join(workdir, outName)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return res, err
	}
	outCap := newCapped(outputCapOf(lim))
	errCap := newCapped(1 << 20)
	cmd.Stdout, cmd.Stderr = outCap, errCap

	limiter, err := newJobLimiter(lim.MemoryMiB, 1)
	if err != nil {
		return res, err
	}
	defer limiter.Close()

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return res, err
	}
	stop := watchGroupKill(ctx, cmd.Process.Pid)
	defer stop()
	// 启动后立刻纳入 Job：单进程程序无 Bourne-shell 间接层，窗口足够小。
	if err := limiter.Attach(cmd.Process.Pid); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return res, err
	}
	// stdin 异步写：输入超过管道缓冲且程序不读时，同步写会阻塞至硬墙
	// 杀进程解锁，污染软限计时；goroutine 写完即关，进程退出管道自解。
	if stdinData != "" {
		go func() {
			_, _ = io.Copy(stdin, bytes.NewReader([]byte(stdinData)))
			_ = stdin.Close()
		}()
	} else {
		_ = stdin.Close()
	}
	waitErr := cmd.Wait()
	elapsed := time.Since(start)

	res.TimeMs = int(elapsed / time.Millisecond)
	if ps := cmd.ProcessState; ps != nil {
		res.CpuMs = int((ps.UserTime() + ps.SystemTime()) / time.Millisecond)
	}
	res.Stdout, res.Stderr = outCap.String(), errCap.String()
	fileTruncated := false
	if outFromFile != "" {
		b, readErr := readCappedFile(outFromFile, outputCapOf(lim))
		switch {
		case errors.Is(readErr, errOutputFileMissing):
			res.NoOutput = true
		case errors.Is(readErr, errOutputCapExceeded):
			fileTruncated = true
			res.Stdout = string(b)
		case readErr != nil:
			res.NoOutput = true
			res.Stdout = ""
		default:
			res.Stdout = string(b)
		}
	}
	res.Truncated = outCap.truncated || errCap.truncated || fileTruncated
	peak := limiter.PeakBytes()
	if peak == 0 && cmd.ProcessState != nil {
		// Unix 侧 jobLimiter 无 Job Object：用 wait4 rusage 的 ru_maxrss 回填。
		peak = processPeakBytes(cmd.ProcessState)
	}
	res.PeakKiB = int(peak >> 10)

	if ctx.Err() == context.DeadlineExceeded {
		// 命中硬杀墙（= 软时限 + 额外时间），稳定 TLE，无需重测。
		res.TimedOut = true
		limiter.Terminate()
		return res, nil
	}
	if waitErr != nil {
		if errors.Is(waitErr, exec.ErrWaitDelay) {
			// WaitDelay 触发:进程已退出,仅孤儿进程仍持管道。结果有效,
			// 退出码取已收割进程的(此处为 0/正常)。
		} else if exitErr, ok := waitErr.(*exec.ExitError); ok {
			res.ExitCode = exitErr.ExitCode()
		} else {
			return res, waitErr
		}
	}
	// Job 内存墙杀掉的进程退出码不可靠：用峰值贴墙判定 MLE。
	if limitBytes(lim.MemoryMiB) > 0 && limiter.PeakBytes()*100 >= uint64(limitBytes(lim.MemoryMiB))*95 {
		res.OutOfMemory = true
	}
	// Unix 补充：RLIMIT_AS 拒绝分配触发的致命信号（bad_alloc abort 等）
	// 判 MLE 而非 RE（对标 LemonLime watcher 的信号映射口径）。
	if !res.OutOfMemory && crashedOnMemoryWall(cmd.ProcessState, res.Stderr, limitBytes(lim.MemoryMiB)) {
		res.OutOfMemory = true
	}
	// cgroup v2:memory.events 的 oom_kill 是确定性 MLE(无启发式)。
	if !res.OutOfMemory && limiter.OutOfMemory() {
		res.OutOfMemory = true
	}
	return res, nil
}

// errOutputFileMissing / errOutputCapExceeded 文件 IO 回读的两种特判态。
var (
	errOutputFileMissing = errors.New("output file missing")
	errOutputCapExceeded = errors.New("output cap exceeded")
)

// readCappedFile 限额读取输出文件：超限截断（文件 IO 模式的 OLE 口径，
// 对标 LemonLime 输出上限；此前 os.ReadFile 无上限，OLE 配置不生效）。
func readCappedFile(path string, cap int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errOutputFileMissing
		}
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, int64(cap)+1))
	if err != nil {
		return b, err
	}
	if len(b) > cap {
		return b[:cap], errOutputCapExceeded
	}
	return b, nil
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

// SafeFileName 文件 IO 名净化：空用缺省，路径一律压平为 Base，杜绝逃逸。
// Windows 语义的硬ening(判题产物最终落在教师机):
//   - "."/".." 单独成段时 Join 会逃出工作目录;
//   - 冒号是盘符相对路径("c:" 逃逸)与 NTFS ADS 分隔;
//   - 尾点/尾空格会被 Win32 静默改写,改名比较与重读产生歧义;
//   - CON/PRN/AUX/NUL/COM¹⁻⁹/LPT¹⁻⁹ 是保留设备名。
//
// 命中任一即落回缺省名。
func SafeFileName(name, fallback string) string {
	if name == "" {
		return fallback
	}
	base := filepath.Base(name)
	if base == "" || base == "." || base == ".." || base == string(filepath.Separator) {
		return fallback
	}
	if strings.ContainsRune(base, ':') {
		return fallback
	}
	if base != strings.TrimRight(base, " .") {
		return fallback
	}
	if reservedWindowsDeviceName(base) {
		return fallback
	}
	return base
}

var windowsReservedDevices = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true,
	"COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true,
	"LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

func reservedWindowsDeviceName(base string) bool {
	stem := strings.ToUpper(base)
	if i := strings.IndexByte(stem, '.'); i >= 0 {
		stem = stem[:i]
	}
	return windowsReservedDevices[stem]
}
