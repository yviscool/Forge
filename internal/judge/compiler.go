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
}

func (t Toolchain) timeout() time.Duration {
	if t.CompileTimeout > 0 {
		return t.CompileTimeout
	}
	return 30 * time.Second
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
	ctx, cancel := context.WithTimeout(ctx, t.timeout())
	defer cancel()

	switch lang {
	case "cpp", "c", "c++":
		cc, file := "g++", "main.cpp"
		if lang == "c" {
			cc, file = "gcc", "main.c"
		}
		srcPath := filepath.Join(workdir, file)
		exePath := filepath.Join(workdir, "main.exe")
		if err := os.WriteFile(srcPath, src, 0644); err != nil {
			return CompileResult{}, err
		}
		var out bytes.Buffer
		cmd := exec.CommandContext(ctx, cc, "-O2", "-std=c++17", "-o", exePath, srcPath)
		if lang == "c" {
			cmd = exec.CommandContext(ctx, cc, "-O2", "-o", exePath, srcPath)
		}
		cmd.Stdout, cmd.Stderr = &out, &out
		if err := cmd.Run(); err != nil {
			return CompileResult{Message: out.String()}, nil
		}
		return CompileResult{OK: true, Executable: exePath}, nil
	case "go", "golang":
		srcPath := filepath.Join(workdir, "main.go")
		exePath := filepath.Join(workdir, "main.exe")
		if err := os.WriteFile(srcPath, src, 0644); err != nil {
			return CompileResult{}, err
		}
		var out bytes.Buffer
		cmd := exec.CommandContext(ctx, "go", "build", "-o", exePath, srcPath)
		cmd.Stdout, cmd.Stderr = &out, &out
		cmd.Dir = workdir
		if err := cmd.Run(); err != nil {
			return CompileResult{Message: out.String()}, nil
		}
		return CompileResult{OK: true, Executable: exePath}, nil
	case "python", "python3", "py":
		srcPath := filepath.Join(workdir, "main.py")
		if err := os.WriteFile(srcPath, src, 0644); err != nil {
			return CompileResult{}, err
		}
		return CompileResult{OK: true, Executable: srcPath, Interpreted: true, Interpreter: "python", SourceFile: srcPath}, nil
	default:
		return CompileResult{Message: "unsupported language: " + lang}, nil
	}
}
