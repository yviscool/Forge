package judge

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// CompileResult 编译结果。Interpreted=true 时 Executable 为待解释执行的源文件。
type CompileResult struct {
	OK          bool
	Message     string
	Executable  string
	Interpreted bool
	Interpreter string
	SourceFile  string
	// TimeLimitRatio / MemoryLimitRatio 工具链声明的运行限额倍率
	// （对标 LemonLime Compiler ratio；0 = 未设置，按 1 处理）。
	TimeLimitRatio   float64
	MemoryLimitRatio float64
	// DisableMemoryLimitCheck 该语言的运行时不适用硬内存墙（如带 GC 的运行时）。
	DisableMemoryLimitCheck bool
}

// CompileOptions 题目级编译选项（对标 LemonLime 每题每编译器配置）。
type CompileOptions struct {
	// Std 覆盖 C/C++ -std（如 "c++17"）；空保持工具链配置。
	Std string
	// Opt 覆盖优化等级（如 "O0", "O2", "O3"）；空保持缺省 "O2"。
	Opt string
	// Warn 告警级别（"default", "wall", "extra", "strict"）。
	Warn string
	// Flags 附加安全编译参数。
	Flags []string
}

// Compiler 语言工具链抽象（可 fake 单测 verdict 映射）。
type Compiler interface {
	Compile(ctx context.Context, lang string, src []byte, workdir string, opts CompileOptions) (CompileResult, error)
}

// compileFail 编译失败（产物缺失）：走 CompileResult.Message，非基础设施错误。
type compileFail struct{ msg string }

func (e *compileFail) Error() string { return e.msg }

// Toolchain 真实编译器：cpp/c（g++/gcc）、go（go build）、python（写入即跑）。
type Toolchain struct {
	// CompileTimeout 单次编译上限。
	CompileTimeout time.Duration
	// Config 工具链配置；零值等价于 DefaultToolchainConfig。
	Config ToolchainConfig
	// Cache 编译缓存（nil 则关闭）。
	Cache *CompileCache
}

func (t Toolchain) spec(lang string) ToolSpec {
	def := DefaultToolchainConfig()
	base, over := def.CPP, t.Config.CPP
	switch lang {
	case "c":
		base, over = def.C, t.Config.C
	case "go", "golang":
		base, over = def.Go, t.Config.Go
	case "python", "python3", "py":
		base, over = def.Python, t.Config.Python
	}
	if over.Command != "" {
		base.Command = over.Command
	}
	if over.Args != nil {
		base.Args = over.Args
	}
	if over.Std != "" {
		base.Std = over.Std
	}
	if over.CompileTimeoutSec > 0 {
		base.CompileTimeoutSec = over.CompileTimeoutSec
	}
	if over.CompileMemoryMiB > 0 {
		base.CompileMemoryMiB = over.CompileMemoryMiB
	}
	if over.TimeLimitRatio > 0 {
		base.TimeLimitRatio = over.TimeLimitRatio
	}
	if over.MemoryLimitRatio > 0 {
		base.MemoryLimitRatio = over.MemoryLimitRatio
	}
	if over.DisableMemoryLimitCheck != nil {
		base.DisableMemoryLimitCheck = over.DisableMemoryLimitCheck
	}
	return base
}

func (t Toolchain) timeout() time.Duration {
	if t.CompileTimeout > 0 {
		return t.CompileTimeout
	}
	return 30 * time.Second
}

// timeoutFor 分语言编译超时：语言配置 > 全局配置 > 30s 缺省。
func (t Toolchain) timeoutFor(sp ToolSpec) time.Duration {
	if sp.CompileTimeoutSec > 0 {
		return time.Duration(sp.CompileTimeoutSec) * time.Second
	}
	return t.timeout()
}

// Argv 运行参数：解释型走 [解释器, 源文件]，编译型走 [可执行文件]。
func (c CompileResult) Argv() []string {
	if !c.OK {
		return nil
	}
	if c.Interpreted {
		return []string{c.Interpreter, c.SourceFile}
	}
	return []string{c.Executable}
}

// Compile 源码落盘 workdir 并编译/检查，返回可执行路径。
// 编译型产物经缓存（键含编译器版本指纹与平台）；解释型直接落盘即跑。
func (t Toolchain) Compile(ctx context.Context, lang string, src []byte, workdir string, opts CompileOptions) (CompileResult, error) {
	sp := t.spec(lang)
	if opts.Std != "" && (lang == "cpp" || lang == "c" || lang == "c++") {
		sp.Std = opts.Std
	}
	ctx, cancel := context.WithTimeout(ctx, t.timeoutFor(sp))
	defer cancel()

	switch lang {
	case "cpp", "c", "c++":
		cc, file := "g++", "main.cpp"
		if lang == "c" {
			cc, file = "gcc", "main.c"
		}
		cc = sp.Command
		// Windows 沙箱:编译器目录(含运行 DLL)需对包 SID 只读可执行。
		prepareRuntimeDir(lang, cc)
		srcPath := filepath.Join(workdir, file)
		exePath := filepath.Join(workdir, "main.exe")
		if err := os.WriteFile(srcPath, src, 0644); err != nil {
			return CompileResult{}, err
		}
		// 缺省即 CCF 复赛口径：-O2 -std=c++14（可在工具链配置或题目级覆盖）。
		opt := "-O2"
		if opts.Opt != "" {
			opt = "-" + opts.Opt
		}
		var extraArgs []string
		switch opts.Warn {
		case "wall":
			extraArgs = append(extraArgs, "-Wall")
		case "extra":
			extraArgs = append(extraArgs, "-Wall", "-Wextra")
		case "strict":
			extraArgs = append(extraArgs, "-Wall", "-Wextra", "-Werror")
		}
		if len(opts.Flags) > 0 {
			extraArgs = append(extraArgs, opts.Flags...)
		}
		args := append([]string{opt}, sp.Args...)
		args = append(args, extraArgs...)
		if lang != "c" && sp.Std != "" {
			args = append(args, "-std="+sp.Std)
		}
		args = append(args, "-o", exePath, srcPath)
		// 缓存 key 记录生效的完整编译参数组合
		sp.Args = append(append([]string{opt}, sp.Args...), extraArgs...)
		return t.cachedCompile(ctx, lang, src, sp, workdir, exePath,
			func(out *bytes.Buffer) error {
				return t.runCompiler(ctx, append([]string{cc}, args...), "", sp.CompileMemoryMiB, out)
			})
	case "go", "golang":
		srcPath := filepath.Join(workdir, "main.go")
		exePath := filepath.Join(workdir, "main.exe")
		if err := os.WriteFile(srcPath, src, 0644); err != nil {
			return CompileResult{}, err
		}
		args := append([]string{"build"}, sp.Args...)
		args = append(args, "-o", exePath, srcPath)
		return t.cachedCompile(ctx, lang, src, sp, workdir, exePath,
			func(out *bytes.Buffer) error {
				return t.runCompiler(ctx, append([]string{sp.Command}, args...), workdir, sp.CompileMemoryMiB, out)
			})
	case "python", "python3", "py":
		// Windows 沙箱:解释器前缀(Lib/ 标准库)需对包 SID 只读。
		prepareRuntimeDir(lang, sp.Command)
		srcPath := filepath.Join(workdir, "main.py")
		if err := os.WriteFile(srcPath, src, 0644); err != nil {
			return CompileResult{}, err
		}
		return CompileResult{OK: true, Executable: srcPath, Interpreted: true,
			Interpreter: sp.Command, SourceFile: srcPath,
			TimeLimitRatio: sp.TimeLimitRatio, MemoryLimitRatio: sp.MemoryLimitRatio,
			DisableMemoryLimitCheck: sp.DisableMemoryLimitCheck != nil && *sp.DisableMemoryLimitCheck}, nil
	default:
		return CompileResult{Message: "unsupported language: " + lang}, nil
	}
}

// cachedCompile 缓存感知编译：命中直接复制产物，未命中执行 build 后落缓存。
// 编译失败（产物缺失）以 CompileResult.Message 表达，不作为基础设施错误。
func (t Toolchain) cachedCompile(ctx context.Context, lang string, src []byte, sp ToolSpec, workdir, exePath string, build func(*bytes.Buffer) error) (CompileResult, error) {
	ratios := CompileResult{
		TimeLimitRatio:          sp.TimeLimitRatio,
		MemoryLimitRatio:        sp.MemoryLimitRatio,
		DisableMemoryLimitCheck: sp.DisableMemoryLimitCheck != nil && *sp.DisableMemoryLimitCheck,
	}
	if t.Cache == nil {
		var out bytes.Buffer
		if err := build(&out); err != nil {
			return CompileResult{Message: out.String()}, nil
		}
		ratios.OK = true
		ratios.Executable = exePath
		return ratios, nil
	}
	exe, _, err := t.Cache.execute(t.Cache.key(lang, src, sp), workdir, func() (string, error) {
		var out bytes.Buffer
		if err := build(&out); err != nil {
			return "", &compileFail{msg: out.String()}
		}
		return exePath, nil
	})
	if err != nil {
		var cf *compileFail
		if errors.As(err, &cf) {
			return CompileResult{Message: cf.msg}, nil
		}
		return CompileResult{}, err
	}
	ratios.OK = true
	ratios.Executable = exe
	return ratios, nil
}

// runCompiler 运行编译器命令：内存墙按需加（Windows Job / Linux prlimit）。
// maxProcs 恒为 0：编译器起 cc1plus 等子进程，绝不套单进程墙；
// Job 的 KILL_ON_CLOSE + Unix 进程组杀保证超时连坐杀掉整棵编译进程树。
func (t Toolchain) runCompiler(ctx context.Context, argv []string, workdir string, memMiB int, out *bytes.Buffer) error {
	if memMiB > 0 {
		// 编译进程树不进 bwrap 沙箱（只需 prlimit 内存墙）。
		argv = confineArgv(argv, Limits{MemoryMiB: memMiB}, "")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	if workdir != "" {
		cmd.Dir = workdir
	}
	cmd.Stdout, cmd.Stderr = out, out
	cmd.SysProcAttr = sysProcAttr()
	limiter, err := newJobLimiter(memMiB, 0)
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
	return cmd.Wait()
}
