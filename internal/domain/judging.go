// LemonLime 对标：Task 比较模式 + TestCase 分点模型 + 子任务依赖 + 分数聚合。
// 评测执行（编译/沙箱运行）仍在后续 judge worker 实现，本文件只定“可单测的纯规则”。
package domain

import (
	"math"
	"strconv"
	"strings"
)

// ComparisonMode 输出比较模式，对标 LemonLime Task::ComparisonMode。
type ComparisonMode string

const (
	CompareLineByLine   ComparisonMode = "line_by_line"
	CompareIgnoreSpace  ComparisonMode = "ignore_space"
	CompareRealNumber   ComparisonMode = "real_number"
	CompareSpecialJudge ComparisonMode = "special_judge"
)

// CaseVerdict 单测试点结论。
type CaseVerdict string

const (
	CaseAC           CaseVerdict = "AC"
	CaseWA           CaseVerdict = "WA"
	CasePE           CaseVerdict = "PE"
	CaseTLE          CaseVerdict = "TLE"
	CaseMLE          CaseVerdict = "MLE"
	CaseRE           CaseVerdict = "RE"
	CaseCE           CaseVerdict = "CE"
	// CaseOLE 输出超限（stdout 命中上限被截断）。
	CaseOLE          CaseVerdict = "OLE"
	CaseSkipped      CaseVerdict = "Skipped"
	CaseCheckerError CaseVerdict = "CheckerError"
	CaseSystemError  CaseVerdict = "SystemError"
)

// CaseResult 单点评测结果（worker 回写时使用）。
type CaseResult struct {
	CaseIndex int         `json:"caseIndex"`
	Verdict   CaseVerdict `json:"verdict"`
	Score     int         `json:"score"`
	TimeMs    int         `json:"timeMs"`
	MemoryKiB int         `json:"memoryKib"`
	// CpuMs 用户+系统 CPU 时间（CCF 口径；并行程序 wall 会低估，用它判 TLE）。
	CpuMs int `json:"cpuMs,omitempty"`
}

// JudgeOutcome 一次评测的完整产出：分点明细 + 编译信息（CE 原文落库用）。
type JudgeOutcome struct {
	Cases          []CaseResult `json:"cases"`
	CompileMessage string       `json:"compileMessage,omitempty"`
}

// normalize 按模式归一化输出文本。
func normalize(s string, mode ComparisonMode) string {
	if mode == CompareIgnoreSpace {
		fields := strings.Fields(s)
		return strings.Join(fields, " ")
	}
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	return strings.Join(lines, "\n")
}

// CompareOutput 比较选手输出与标准输出。
// realEps 仅在 CompareRealNumber 下使用（如 1e-3 对标 LemonLime realPrecision）。
func CompareOutput(got, want string, mode ComparisonMode, realEps float64) bool {
	switch mode {
	case CompareIgnoreSpace:
		return normalize(got, mode) == normalize(want, mode)
	case CompareRealNumber:
		g, w := strings.Fields(got), strings.Fields(want)
		if len(g) != len(w) {
			return false
		}
		eps := realEps
		if eps <= 0 {
			eps = 1e-3
		}
		for i := range g {
			gf, ge := strconv.ParseFloat(g[i], 64)
			wf, we := strconv.ParseFloat(w[i], 64)
			if ge == nil && we == nil {
				diff := math.Abs(gf - wf)
				allowed := eps * math.Max(1, math.Abs(wf))
				if diff > allowed {
					return false
				}
				continue
			}
			if g[i] != w[i] {
				return false
			}
		}
		return true
	case CompareSpecialJudge:
		// 特判逻辑由外部 checker 二进制执行，这里只占位：永不误判 AC。
		return false
	default:
		return normalize(got, CompareLineByLine) == normalize(want, CompareLineByLine)
	}
}

// AggregateScore 子任务依赖感知聚合（兼容无显式依赖旧接口）。
func AggregateScore(cases []CaseResult, subtaskOf func(caseIndex int) int, fullOf func(subtask int) int) int {
	return AggregateScoreWithDeps(cases, subtaskOf, fullOf, nil)
}

// AggregateScoreWithDeps 完整子任务 DAG 依赖感知聚合：
// 1. 某子任务内任一测试点非 AC，则该子任务失败（得 0 分）；
// 2. 某子任务若依赖其他子任务，则其所有依赖子任务必须全部 AC，否则该子任务得 0 分。
func AggregateScoreWithDeps(cases []CaseResult, subtaskOf func(caseIndex int) int, fullOf func(subtask int) int, depsOf func(subtask int) []int) int {
	bySub := map[int][]CaseResult{}
	for _, c := range cases {
		st := subtaskOf(c.CaseIndex)
		bySub[st] = append(bySub[st], c)
	}

	subtaskAC := map[int]bool{}
	for st, cs := range bySub {
		ok := true
		for _, c := range cs {
			if c.Verdict != CaseAC {
				ok = false
				break
			}
		}
		subtaskAC[st] = ok
	}

	if depsOf == nil {
		total := 0
		for st, ok := range subtaskAC {
			if ok {
				total += fullOf(st)
			}
		}
		return total
	}

	// 依赖校验（支持多级依赖拓扑）
	var checkPassed func(st int, visiting map[int]bool) bool
	checkPassed = func(st int, visiting map[int]bool) bool {
		if !subtaskAC[st] {
			return false
		}
		if visiting[st] {
			return false // 防止配置了环形依赖死递归
		}
		visiting[st] = true
		for _, dep := range depsOf(st) {
			if !checkPassed(dep, visiting) {
				return false
			}
		}
		delete(visiting, st)
		return true
	}

	total := 0
	for st := range bySub {
		if checkPassed(st, map[int]bool{}) {
			total += fullOf(st)
		}
	}
	return total
}

// ShouldSkipCase 判定当前测试点是否应短路跳过：
// 当所属子任务已有测试点失败，或其任何前置依赖子任务已失败时返回 true。
func ShouldSkipCase(subtask int, deps []int, failedSubtasks map[int]bool) bool {
	if failedSubtasks[subtask] {
		return true
	}
	for _, d := range deps {
		if failedSubtasks[d] {
			return true
		}
	}
	return false
}
