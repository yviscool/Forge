package packageio

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yviscool/forge/internal/domain"
)

type qduojProblemJSON struct {
	Title             string `json:"title"`
	Description       string `json:"description"`
	InputDescription  string `json:"input_description"`
	OutputDescription string `json:"output_description"`
	Hint              string `json:"hint"`
	TimeLimit         int    `json:"time_limit"`
	MemoryLimit       int    `json:"memory_limit"`
	SPJ               bool   `json:"spj"`
	SPJCode           string `json:"spj_code"`
	SPJLanguage       string `json:"spj_language"`
	Samples           []struct {
		Input  string `json:"input"`
		Output string `json:"output"`
	} `json:"samples"`
}

// ParseQDUOJ 解析 QDUOJ / 洛谷标准 JSON 试题包。
func ParseQDUOJ(files map[string][]byte) (*ParsedPackage, error) {
	data, ok := files["problem.json"]
	if !ok {
		return nil, fmt.Errorf("problem.json not found")
	}

	var meta qduojProblemJSON
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("unmarshal problem.json: %w", err)
	}

	timeMs := meta.TimeLimit
	if timeMs <= 0 {
		timeMs = 1000
	}
	memoryMiB := meta.MemoryLimit
	if memoryMiB <= 0 {
		memoryMiB = 256
	}

	statement := meta.Description
	if statement == "" {
		statement = "请在此输入题面描述"
	}
	inputSpec := meta.InputDescription
	if inputSpec == "" {
		inputSpec = "标准输入"
	}
	outputSpec := meta.OutputDescription
	if outputSpec == "" {
		outputSpec = "标准输出"
	}
	constraints := meta.Hint
	if constraints == "" {
		constraints = fmt.Sprintf("时间限制 %d ms，空间限制 %d MiB", timeMs, memoryMiB)
	}

	var examplesBuilder strings.Builder
	for i, smp := range meta.Samples {
		examplesBuilder.WriteString(fmt.Sprintf("### 样例 %d\n**输入**:\n```\n%s\n```\n**输出**:\n```\n%s\n```\n\n",
			i+1, smp.Input, smp.Output))
	}

	// 测试数据提取
	inFiles := make(map[string][]byte)
	outFiles := make(map[string][]byte)
	rawInputs := make(map[string]string)
	rawOutputs := make(map[string]string)

	for fname := range files {
		ext := strings.ToLower(filepath.Ext(fname))
		if ext == ".in" {
			base := strings.TrimSuffix(fname, ext)
			rawInputs[base] = fname
		} else if ext == ".out" || ext == ".ans" {
			base := strings.TrimSuffix(fname, ext)
			rawOutputs[base] = fname
		}
	}

	var stems []string
	for s := range rawInputs {
		if _, ok := rawOutputs[s]; ok {
			stems = append(stems, s)
		}
	}
	sort.Strings(stems)

	var testCases []domain.TestCase
	scorePerCase := 100
	if len(stems) > 0 {
		scorePerCase = 100 / len(stems)
	}

	for idx, s := range stems {
		inName := fmt.Sprintf("case%02d.in", idx+1)
		outName := fmt.Sprintf("case%02d.out", idx+1)
		inFiles[inName] = files[rawInputs[s]]
		outFiles[outName] = files[rawOutputs[s]]

		testCases = append(testCases, domain.TestCase{
			ID:          fmt.Sprintf("case%02d", idx+1),
			InputFile:   inName,
			OutputFile:  outName,
			Score:       scorePerCase,
			Subtask:     0,
			TimeLimitMs: timeMs,
			MemoryMiB:   memoryMiB,
		})
	}
	if len(testCases) > 0 {
		testCases[len(testCases)-1].Score += 100 - (scorePerCase * len(testCases))
	}

	compareMode := domain.CompareIgnoreSpace
	if meta.SPJ {
		compareMode = domain.CompareSpecialJudge
	}

	return &ParsedPackage{
		Format: FormatQDUOJ,
		Problem: domain.Problem{
			Title:          meta.Title,
			Statement:      statement,
			Input:          inputSpec,
			Output:         outputSpec,
			Constraints:    constraints,
			Examples:       examplesBuilder.String(),
			TimeLimitMs:    timeMs,
			MemoryLimitMiB: memoryMiB,
			CompareMode:    compareMode,
			CheckerCode:    meta.SPJCode,
			CheckerLang:    meta.SPJLanguage,
			TestCases:      testCases,
		},
		TestCases:   testCases,
		InFiles:     inFiles,
		OutFiles:    outFiles,
		CheckerCode: meta.SPJCode,
		CheckerLang: meta.SPJLanguage,
	}, nil
}
