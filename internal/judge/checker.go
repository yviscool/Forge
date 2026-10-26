package judge

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yviscool/forge/internal/domain"
)

//go:embed testlib_min.h
var testlibMinH string

// CheckerSpec 特判程序源码（随题目下发，评测机编译执行）。
type CheckerSpec struct {
	Language string
	Code     string
	// Flavor 特判协议：
	//   "" / "exit"   退出码协议：checker <input> <output> <answer>，
	//                  exit 0 = AC；1 = WA；2 = PE；其他 = CheckerError。
	//   "lemon"       Lemon 协议（对标 LemonLime LemonSpecialJudge）：
	//                  checker <input> <output> <answer> <fullScore>
	//                          <scoreFile> <messageFile>，
	//                  scoreFile 写 0..fullScore 支持部分分，messageFile 带附注。
	//   "testlib"     testlib stderr 协议（对标 LemonLime TestlibSpecialJudge）：
	//                  checker <input> <output> <answer>，
	//                  stderr 首行 ok / partially correct (N) / points X。
	Flavor string
}

const (
	checkerFlavorExit    = ""
	checkerFlavorLemon   = "lemon"
	checkerFlavorTestlib = "testlib"
)

type compiledChecker struct {
	ok     bool
	argv   []string
	msg    string
	flavor string
	// interpreted 解释型特判：跳过地址空间墙（ulimit -v/RLIMIT_AS 会误杀
	// 自带内存管理的运行时），保留 CPU/进程数墙与超时。
	interpreted bool
}

func compileChecker(ctx context.Context, comp Compiler, spec CheckerSpec, workdir string) (*compiledChecker, error) {
	dir, err := os.MkdirTemp(workdir, "checker_*")
	if err != nil {
		return nil, err
	}
	// testlib 兼容头随手可得：checker 直接 #include "testlib_min.h"。
	_ = os.WriteFile(filepath.Join(dir, "testlib_min.h"), []byte(testlibMinH), 0644)
	res, err := comp.Compile(ctx, spec.Language, []byte(spec.Code), dir, CompileOptions{})
	if err != nil {
		return nil, err
	}
	if !res.OK {
		return &compiledChecker{msg: res.Message}, nil
	}
	flavor := spec.Flavor
	switch flavor {
	case checkerFlavorLemon, checkerFlavorTestlib, checkerFlavorExit:
	default:
		flavor = checkerFlavorExit
	}
	return &compiledChecker{ok: true, argv: res.Argv(), flavor: flavor, interpreted: res.Interpreted}, nil
}

// check 运行一次特判：返回判定、该点得分（0..full）与附注消息。
func (c *compiledChecker) check(ctx context.Context, input, output, answer string, fullScore int, workdir string) (domain.CaseVerdict, int, string) {
	if fullScore <= 0 {
		fullScore = 100
	}
	switch c.flavor {
	case checkerFlavorLemon:
		return c.checkLemon(ctx, input, output, answer, fullScore, workdir)
	case checkerFlavorTestlib:
		return c.checkTestlib(ctx, input, output, answer, fullScore, workdir)
	default:
		return c.checkExit(ctx, input, output, answer, fullScore, workdir)
	}
}

// checkExit 退出码协议：全对或全错，无部分分。
func (c *compiledChecker) checkExit(ctx context.Context, input, output, answer string, fullScore int, _ string) (domain.CaseVerdict, int, string) {
	dir, err := os.MkdirTemp("", "chk_*")
	if err != nil {
		return domain.CaseCheckerError, 0, err.Error()
	}
	defer os.RemoveAll(dir)
	inF, outF, ansF := filepath.Join(dir, "input.txt"), filepath.Join(dir, "output.txt"), filepath.Join(dir, "answer.txt")
	for f, s := range map[string]string{inF: input, outF: output, ansF: answer} {
		if err := os.WriteFile(f, []byte(s), 0644); err != nil {
			return domain.CaseCheckerError, 0, err.Error()
		}
	}
	verdict, msg, err := c.runExitProtocol(ctx, dir, []string{inF, outF, ansF}, nil)
	if err != nil {
		return domain.CaseCheckerError, 0, err.Error() + ": " + msg
	}
	switch verdict {
	case domain.CaseAC:
		return domain.CaseAC, fullScore, msg
	case domain.CasePE:
		return domain.CasePE, 0, msg
	case domain.CaseWA:
		return domain.CaseWA, 0, msg
	default:
		return domain.CaseCheckerError, 0, msg
	}
}

// checkLemon Lemon 协议特判：scoreFile/messageFile 落盘传参，支持部分分。
func (c *compiledChecker) checkLemon(ctx context.Context, input, output, answer string, fullScore int, workdir string) (domain.CaseVerdict, int, string) {
	dir, err := os.MkdirTemp(workdir, "lemonchk_*")
	if err != nil {
		return domain.CaseCheckerError, 0, err.Error()
	}
	defer os.RemoveAll(dir)
	inF := filepath.Join(dir, "input.txt")
	outF := filepath.Join(dir, "output.txt")
	ansF := filepath.Join(dir, "answer.txt")
	scoreF := filepath.Join(dir, "_score")
	msgF := filepath.Join(dir, "_message")
	for f, s := range map[string]string{inF: input, outF: output, ansF: answer} {
		if err := os.WriteFile(f, []byte(s), 0644); err != nil {
			return domain.CaseCheckerError, 0, err.Error()
		}
	}
	argv := append(append([]string{}, c.argv...),
		inF, outF, ansF, strconv.Itoa(fullScore), scoreF, msgF)
	ctx, cancel := context.WithTimeout(ctx, checkerTimeout)
	defer cancel()
	out := newCapped(1 << 20)
	if err := runConfined(ctx, argv, dir, c.interpreted, out, out); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return domain.CaseCheckerError, 0, "lemon checker timeout"
		}
		if ee, ok := err.(*exec.ExitError); ok {
			return domain.CaseCheckerError, 0, fmt.Sprintf("lemon checker exit %d: %s", ee.ExitCode(), out.String())
		}
		return domain.CaseCheckerError, 0, fmt.Sprintf("lemon checker error: %v: %s", err, out.String())
	}
	scoreRaw, err := os.ReadFile(scoreF)
	if err != nil {
		return domain.CaseCheckerError, 0, "lemon checker 未写出 _score 文件"
	}
	score, ok := parseLemonScore(string(scoreRaw))
	if !ok {
		return domain.CaseCheckerError, 0, "lemon checker _score 内容无效: " + strings.TrimSpace(string(scoreRaw))
	}
	if score > fullScore {
		score = fullScore
	}
	message := ""
	if b, err := os.ReadFile(msgF); err == nil {
		message = truncateTail(string(b), 4096)
	}
	return verdictForScore(score, fullScore), score, message
}

// checkTestlib testlib stderr 协议：ok / partially correct (N) / points X。
func (c *compiledChecker) checkTestlib(ctx context.Context, input, output, answer string, fullScore int, _ string) (domain.CaseVerdict, int, string) {
	dir, err := os.MkdirTemp("", "tlchk_*")
	if err != nil {
		return domain.CaseCheckerError, 0, err.Error()
	}
	defer os.RemoveAll(dir)
	inF, outF, ansF := filepath.Join(dir, "input.txt"), filepath.Join(dir, "output.txt"), filepath.Join(dir, "answer.txt")
	for f, s := range map[string]string{inF: input, outF: output, ansF: answer} {
		if err := os.WriteFile(f, []byte(s), 0644); err != nil {
			return domain.CaseCheckerError, 0, err.Error()
		}
	}
	ctx, cancel := context.WithTimeout(ctx, checkerTimeout)
	defer cancel()
	argv := append(append([]string{}, c.argv...), inF, outF, ansF)
	stdout, stderr := newCapped(1<<20), newCapped(1<<20)
	err = runConfined(ctx, argv, dir, c.interpreted, stdout, stderr)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return domain.CaseCheckerError, 0, "testlib checker timeout"
		}
		ee, ok := err.(*exec.ExitError)
		if !ok {
			return domain.CaseCheckerError, 0, fmt.Sprintf("testlib checker error: %v", err)
		}
		switch ee.ExitCode() {
		case 1:
			return domain.CaseWA, 0, stderr.String()
		case 2:
			return domain.CasePE, 0, stderr.String()
		default:
			// testlib exit 3 = FAIL（校验器自身出错）。
			return domain.CaseCheckerError, 0, stderr.String()
		}
	}
	score, ok, fail := parseTestlibResult(stderr.String())
	if fail {
		return domain.CaseCheckerError, 0, stderr.String()
	}
	if !ok {
		// 无打分标记：按 LemonLime 语义落到 WA 0 分并附 stderr。
		return domain.CaseWA, 0, stderr.String()
	}
	total := int(score*float64(fullScore) + 0.5)
	if total < 0 {
		total = 0
	}
	if total > fullScore {
		total = fullScore
	}
	return verdictForScore(total, fullScore), total, stderr.String()
}

// checkerTimeout 特判运行上限（对标 LemonLime specialJudgeTimeLimit 缺省）。
const checkerTimeout = 10 * time.Second

func verdictForScore(score, full int) domain.CaseVerdict {
	switch {
	case score >= full:
		return domain.CaseAC
	case score <= 0:
		return domain.CaseWA
	default:
		return domain.CasePartly
	}
}

// runExitProtocol 退出码协议执行（exit 0=AC/1=WA/2=PE/其他=CheckerError）。
// extraArgs 允许追加参数（协议扩展用），nil 即经典三参。
func (c *compiledChecker) runExitProtocol(ctx context.Context, dir string, fileArgs, extraArgs []string) (domain.CaseVerdict, string, error) {
	ctx, cancel := context.WithTimeout(ctx, checkerTimeout)
	defer cancel()
	out := newCapped(1 << 20)
	full := append(append(append([]string{}, c.argv...), fileArgs...), extraArgs...)
	if err := runConfined(ctx, full, dir, c.interpreted, out, out); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return domain.CaseCheckerError, "checker timeout", nil
		}
		if ee, ok := err.(*exec.ExitError); ok {
			switch ee.ExitCode() {
			case 1:
				return domain.CaseWA, out.String(), nil
			case 2:
				return domain.CasePE, out.String(), nil
			default:
				return domain.CaseCheckerError, fmt.Sprintf("checker crash (exit %d): %s", ee.ExitCode(), out.String()), nil
			}
		}
		return domain.CaseCheckerError, fmt.Sprintf("checker error: %v: %s", err, out.String()), nil
	}
	return domain.CaseAC, out.String(), nil
}

// parseLemonScore 解析 Lemon 协议 _score 文件：整数分值（允许首尾空白）。
func parseLemonScore(content string) (int, bool) {
	fields := strings.Fields(content)
	if len(fields) == 0 {
		return 0, false
	}
	n, err := strconv.Atoi(fields[0])
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

var (
	testlibPartialRe = regexp.MustCompile(`(?m)^partially correct \((\d+)\)`)
	testlibPointsRe  = regexp.MustCompile(`(?m)^points ([0-9]*\.[0-9]+|[0-9]+)`)
)

// parseTestlibResult 解析 testlib stderr 打分标记。
// 返回 (分值比例或绝对值, 是否命中打分标记, 是否 FAIL)。
//   - "ok" 前缀 → 1.0（满分）
//   - "partially correct (N)" → N/100
//   - "points X" → X（0..1 的比例由调用方乘满分；X>1 钳制到满分，
//     与 upstream points X×fullScore 等价且不会落超满分分——有意差异）
func parseTestlibResult(stderr string) (float64, bool, bool) {
	text := strings.TrimSpace(stderr)
	if text == "" {
		return 0, false, false
	}
	if strings.HasPrefix(text, "FAIL") {
		return 0, false, true
	}
	if m := testlibPartialRe.FindStringSubmatch(text); m != nil {
		n, err := strconv.Atoi(m[1])
		if err == nil {
			return float64(n) / 100, true, false
		}
	}
	if m := testlibPointsRe.FindStringSubmatch(text); m != nil {
		v, err := strconv.ParseFloat(m[1], 64)
		if err == nil {
			return v, true, false
		}
	}
	if strings.HasPrefix(text, "ok") {
		return 1, true, false
	}
	return 0, false, false
}

func truncateTail(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[len(s)-limit:]
}
