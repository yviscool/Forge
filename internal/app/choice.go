package app

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/yviscool/forge/internal/domain"
)

// normalizeChoiceAnswer 将各种格式的客观题答案规整为标准化比对形式。
func normalizeChoiceAnswer(qType string, v any) string {
	if v == nil {
		return ""
	}

	// 处理数组/切片（多选题）
	switch val := v.(type) {
	case []any:
		var items []string
		for _, item := range val {
			s := strings.ToUpper(strings.TrimSpace(fmt.Sprint(item)))
			if s != "" {
				items = append(items, s)
			}
		}
		sort.Strings(items)
		return strings.Join(items, "")
	case []string:
		var items []string
		for _, s := range val {
			ts := strings.ToUpper(strings.TrimSpace(s))
			if ts != "" {
				items = append(items, ts)
			}
		}
		sort.Strings(items)
		return strings.Join(items, "")
	}

	raw := strings.TrimSpace(fmt.Sprint(v))
	upper := strings.ToUpper(raw)

	if qType == "judge" {
		switch upper {
		case "T", "TRUE", "YES", "1", "对", "正确", "V", "√":
			return "T"
		case "F", "FALSE", "NO", "0", "错", "错误", "X", "×":
			return "F"
		default:
			return upper
		}
	}

	if qType == "multiple" {
		// 清除多选中的逗号、空格等分隔符并排序
		cleaned := strings.Map(func(r rune) rune {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				return r
			}
			return -1
		}, upper)
		runes := []rune(cleaned)
		sort.Slice(runes, func(i, j int) bool { return runes[i] < runes[j] })
		return string(runes)
	}

	return upper
}

// JudgeChoice 对 CCF GESP 客观选择/判断题进行纯内存微秒级自动判分。
func JudgeChoice(p domain.Problem, submissionCode string) (string, int, []domain.CaseResult, error) {
	var userAnswers map[string]any
	if err := json.Unmarshal([]byte(submissionCode), &userAnswers); err != nil {
		return domain.VerdictCompileError, 0, nil, fmt.Errorf("答题卡 JSON 格式解析失败: %w", err)
	}

	questions := p.ChoiceQuestions
	if len(questions) == 0 && p.ChoiceKeyJSON != "" {
		// 尝试从 ChoiceKeyJSON 解析 key.json
		var keyMap map[string]any
		if err := json.Unmarshal([]byte(p.ChoiceKeyJSON), &keyMap); err == nil {
			var qIDs []int
			for k := range keyMap {
				if id, err := strconv.Atoi(k); err == nil {
					qIDs = append(qIDs, id)
				}
			}
			sort.Ints(qIDs)
			for _, id := range qIDs {
				questions = append(questions, domain.ChoiceQuestion{
					ID:     id,
					Type:   "single",
					Answer: keyMap[strconv.Itoa(id)],
					Score:  2,
				})
			}
		}
	}

	if len(questions) == 0 {
		return domain.VerdictSystemError, 0, nil, fmt.Errorf("题目未配置客观题题库或标准答案")
	}

	totalScore := 0
	maxScore := 0
	cases := make([]domain.CaseResult, 0, len(questions))

	for i, q := range questions {
		qScore := q.Score
		if qScore <= 0 {
			qScore = 2 // 默认 2 分
		}
		maxScore += qScore

		// 支持通过 "1" 或 1 查找用户答案
		uVal, exists := userAnswers[strconv.Itoa(q.ID)]
		if !exists {
			uVal, exists = userAnswers[strconv.Itoa(i+1)]
		}

		userNorm := normalizeChoiceAnswer(q.Type, uVal)
		keyNorm := normalizeChoiceAnswer(q.Type, q.Answer)

		isCorrect := exists && userNorm != "" && userNorm == keyNorm

		cRes := domain.CaseResult{
			CaseIndex: q.ID,
			TimeMs:    1,
			MemoryKiB: 512,
		}

		if isCorrect {
			cRes.Verdict = domain.CaseAC
			cRes.Score = qScore
			totalScore += qScore
		} else {
			cRes.Verdict = domain.CaseWA
			cRes.Score = 0
		}

		cases = append(cases, cRes)
	}

	verdict := domain.VerdictWrongAnswer
	if totalScore == maxScore && maxScore > 0 {
		verdict = domain.VerdictAccepted
	}

	return verdict, totalScore, cases, nil
}
