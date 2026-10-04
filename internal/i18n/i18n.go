// Package i18n 全栈五维 i18n 的后端基石：UI 字典 + API 错误本地化 +
// 判题 verdict 中英映射 + Accept-Language 协商。
package i18n

import (
	"strings"
)

// VerdictZH 标准 verdict → 中文。
var VerdictZH = map[string]string{
	"queued":        "排队中",
	"judging":       "评测中",
	"accepted":      "答案正确",
	"wrong_answer":  "答案错误",
	"time_limit":    "运行超时",
	"runtime_error": "运行时错误",
	"compile_error": "编译错误",
}

// VerdictEN 中文 → 标准 verdict（兼容教师手填中文）。
var VerdictEN = map[string]string{
	"答案正确": "accepted",
	"答案错误": "wrong_answer",
	"运行超时": "time_limit",
	"运行时错误": "runtime_error",
	"编译错误": "compile_error",
}

// Negotiate 解析 Accept-Language，缺省 zh-CN。
func Negotiate(header string) string {
	h := strings.ToLower(header)
	if strings.Contains(h, "en") && !strings.Contains(h, "zh") {
		return "en-US"
	}
	return "zh-CN"
}

// VerdictText 按 locale 返回展示文本。
func VerdictText(verdict, locale string) string {
	if locale == "en-US" {
		return verdict
	}
	if zh, ok := VerdictZH[verdict]; ok {
		return zh
	}
	return verdict
}

// ErrorText 错误码 → 本地化消息（地基先覆盖核心码，后续增量扩展）。
func ErrorText(code, locale string) string {
	zh := map[string]string{
		"contest_not_running": "比赛尚未开始或已结束",
		"contest_not_found":   "比赛不存在",
		"user_not_found":      "用户不存在",
		"not_registered":      "该用户未报名本场比赛",
		"name_required":       "名称不能为空",
	}
	en := map[string]string{
		"contest_not_running": "contest is not running",
		"contest_not_found":   "contest not found",
		"user_not_found":      "user not found",
		"not_registered":      "user is not registered for this contest",
		"name_required":       "name is required",
	}
	if locale == "en-US" {
		if m, ok := en[code]; ok {
			return m
		}
		return code
	}
	if m, ok := zh[code]; ok {
		return m
	}
	return code
}
