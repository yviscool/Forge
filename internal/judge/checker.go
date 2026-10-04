package judge

import (
	"bytes"
	"context"
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

//go:embed testlib_min.h
var testlibMinH string

// Checker 协议：checker <input >output >answer。
// exit 0 = AC；exit 1/2 = WA（含 testlib 的 WA/PE）；其他非零 = RE。
// stdout 首行可附带信息（忽略）。
type compiledChecker struct {
	ok   bool
	argv []string
	msg  string
}

func compileChecker(ctx context.Context, comp Compiler, spec CheckerSpec, workdir string) (*compiledChecker, error) {
	dir, err := os.MkdirTemp(workdir, "checker_*")
	if err != nil {
		return nil, err
	}
	// testlib 兼容头随手可得：checker 直接 #include "testlib_min.h"。
	_ = os.WriteFile(filepath.Join(dir, "testlib_min.h"), []byte(testlibMinH), 0644)
	res, err := comp.Compile(ctx, spec.Language, []byte(spec.Code), dir)
	if err != nil {
		return nil, err
	}
	if !res.OK {
		return &compiledChecker{msg: res.Message}, nil
	}
	return &compiledChecker{ok: true, argv: res.Argv()}, nil
}

func (c *compiledChecker) check(ctx context.Context, input, output, answer, workdir string) (bool, string) {
	dir, err := os.MkdirTemp(workdir, "chk_*")
	if err != nil {
		return false, err.Error()
	}
	defer os.RemoveAll(dir)
	inF := filepath.Join(dir, "input.txt")
	outF := filepath.Join(dir, "output.txt")
	ansF := filepath.Join(dir, "answer.txt")
	for f, s := range map[string]string{inF: input, outF: output, ansF: answer} {
		if err := os.WriteFile(f, []byte(s), 0644); err != nil {
			return false, err.Error()
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, c.argv[0], append(append([]string{}, c.argv[1:]...), inF, outF, ansF)...)
	cmd.Stdout, cmd.Stderr = &stdout, &stdout
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok && (ee.ExitCode() == 1 || ee.ExitCode() == 2) {
			return false, stdout.String()
		}
		return false, "checker error: " + stdout.String()
	}
	return true, stdout.String()
}
