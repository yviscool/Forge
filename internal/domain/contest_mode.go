package domain

import "strings"

// 比赛组织模式（Contest.Mode），对齐 LemonNext 的三种开展方式：
//   online — 在线提交（默认）：边赛边评，学生实时看到判定与榜单；
//   noi    — NOI 全真模拟：赛中学生只看到「已提交」，判定与榜单在比赛结束后公布，
//            每题以最后一次提交为准（提交可覆盖，覆盖规则与 NOI 现场一致）；
//   gesp   — GESP 机考：客观题试卷（选择题/判断题），交卷即判。
const (
	ModeOnline = "online"
	ModeNOI    = "noi"
	ModeGESP   = "gesp"
)

// NormalizeMode 归一化比赛模式；空值视为 online（存量比赛零迁移保持原行为）。
func NormalizeMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case ModeNOI:
		return ModeNOI
	case ModeGESP:
		return ModeGESP
	default:
		return ModeOnline
	}
}

// ValidMode 报告取值是否为已知模式（空值合法，代表 online）。
// 注意不能走 NormalizeMode——它会把未知值兜底成 online。
func ValidMode(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", ModeOnline, ModeNOI, ModeGESP:
		return true
	default:
		return false
	}
}

// StudentResultsVisible NOI 全真语义：比赛进行中学生不可见任何判定结果
// （自己的提交也只显示「已提交」），结束后随成绩一并公布。
// online / gesp 模式始终实时可见。
func StudentResultsVisible(mode string, status Status) bool {
	if NormalizeMode(mode) == ModeNOI && status == Running {
		return false
	}
	return true
}

// NOI 形态下学生侧展示用的替代判定：不携带任何判分信息。
const VerdictSubmittedMasked = "submitted"

// MaskSubmissionForStudent NOI 进行中对学生隐藏判分细节：
// 判定替换为 submitted、分数清零、分点明细抹除；提交时间与语言保留（监考与列表需要）。
func MaskSubmissionForStudent(s Submission) Submission {
	masked := s
	masked.Verdict = VerdictSubmittedMasked
	masked.Score = 0
	masked.Cases = nil
	return masked
}
