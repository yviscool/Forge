package judge

import (
	"bytes"
	"context"
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
}

// Compiler 语言工具链抽象（可 fake 单测 verdict 映射）。
type Compiler interface {
	Compile(ctx context.Context, lang string, src []byte, workdir string) (CompileResult, error)
}

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
func (t Toolchain) Compile(ctx context.Context, lang string, src []byte, workdir string) (CompileResult, error) {
	sp := t.spec(lang)
	ctx, cancel := context.WithTimeout(ctx, t.timeoutFor(sp))
	defer cancel()

	switch lang {
	case "cpp", "c", "c++":
		cc, file := "g++", "main.cpp"
		if lang == "c" {
			cc, file = "gcc", "main.c"
		}
		cc = sp.Command
		srcPath := filepath.Join(workdir, file)
		exePath := filepath.Join(workdir, "main.exe")
		if err := os.WriteFile(srcPath, src, 0644); err != nil {
			return CompileResult{}, err
		}
		if hit, ok := t.Cache.Get(lang, src, t.Config, workdir); ok {
			return CompileResult{OK: true, Executable: hit}, nil
		}
		var out bytes.Buffer
		// 缺省即 CCF 复赛口径：-O2 -std=c++14（可在工具链配置覆盖）。
		args := append([]string{"-O2"}, sp.Args...)
		if lang != "c" && sp.Std != "" {
			args = append(args, "-std="+sp.Std)
		}
		args = append(args, "-o", exePath, srcPath)
		if err := t.runCompiler(ctx, append([]string{cc}, args...), "", sp.CompileMemoryMiB, &out); err != nil {
			return CompileResult{Message: out.String()}, nil
		}
		t.Cache.Put(lang, src, t.Config, exePath)
		return CompileResult{OK: true, Executable: exePath}, nil
	case "go", "golang":
		srcPath := filepath.Join(workdir, "main.go")
		exePath := filepath.Join(workdir, "main.exe")
		if err := os.WriteFile(srcPath, src, 0644); err != nil {
			return CompileResult{}, err
		}
		var out bytes.Buffer
		args := append([]string{"build"}, sp.Args...)
		args = append(args, "-o", exePath, srcPath)
		if err := t.runCompiler(ctx, append([]string{sp.Command}, args...), workdir, sp.CompileMemoryMiB, &out); err != nil {
			return CompileResult{Message: out.String()}, nil
		}
		return CompileResult{OK: true, Executable: exePath}, nil
	case "python", "python3", "py":
		srcPath := filepath.Join(workdir, "main.py")
		if err := os.WriteFile(srcPath, src, 0644); err != nil {
			return CompileResult{}, err
		}
		return CompileResult{OK: true, Executable: srcPath, Interpreted: true, Interpreter: sp.Command, SourceFile: srcPath}, nil
	default:
		return CompileResult{Message: "unsupported language: " + lang}, nil
	}
}

// runCompiler 运行编译器命令：内存墙按需加（Windows Job / Linux prlimit）。
// maxProcs 恒为 0：编译器起 cc1plus 等子进程，绝不加单进程墙。
// Job 的 KILL_ON_CLOSE 保证超时连坐杀掉整棵编译进程树。
func (t Toolchain) runCompiler(ctx context.Context, argv []string, workdir string, memMiB int, out *bytes.Buffer) error {
	if memMiB > 0 {
		argv = confineArgv(argv, Limits{MemoryMiB: memMiB})
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	if workdir != "" {
		cmd.Dir = workdir
	}
	cmd.Stdout, cmd.Stderr = out, out
	limiter, err := newJobLimiter(memMiB, 0)
	if err != nil {
		return err
	}
	defer limiter.Close()
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := limiter.Attach(cmd.Process.Pid); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return err
	}
	return cmd.Wait()
}
