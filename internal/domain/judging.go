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
	CompareLineByLine  ComparisonMode = "line_by_line"
	CompareIgnoreSpace ComparisonMode = "ignore_space"
	CompareRealNumber  ComparisonMode = "real_number"
	CompareSpecialJudge ComparisonMode = "special_judge"
)

// CaseVerdict 单测试点结论。
type CaseVerdict string

const (
	CaseAC CaseVerdict = "AC"
	CaseWA CaseVerdict = "WA"
	CaseTLE CaseVerdict = "TLE"
	CaseMLE CaseVerdict = "MLE"
	CaseRE  CaseVerdict = "RE"
	CaseCE  CaseVerdict = "CE"
)

// CaseResult 单点评测结果（worker 回写时使用）。
type CaseResult struct {
	CaseIndex int         `json:"caseIndex"`
	Verdict   CaseVerdict `json:"verdict"`
	Score     int         `json:"score"`
	TimeMs    int         `json:"timeMs"`
	MemoryKiB int         `json:"memoryKib"`
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

// AggregateScore 子任务依赖感知聚合：
// 某子任务内任一测试点非 AC 则该子任务得 0（对标 LemonLime dependenceSubtask 语义）。
func AggregateScore(cases []CaseResult, subtaskOf func(caseIndex int) int, fullOf func(subtask int) int) int {
	bySub := map[int][]CaseResult{}
	for _, c := range cases {
		st := subtaskOf(c.CaseIndex)
		bySub[st] = append(bySub[st], c)
	}
	total := 0
	for st, cs := range bySub {
		ok := true
		for _, c := range cs {
			if c.Verdict != CaseAC {
				ok = false
				break
			}
		}
		if ok {
			total += fullOf(st)
		}
	}
	return total
}
