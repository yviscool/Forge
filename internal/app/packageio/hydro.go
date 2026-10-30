package packageio

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/yviscool/forge/internal/domain"
)

// ParseHydro 解析 Hydro 题包 (problem.yaml / problem.md / testdata/)。
// 为保证单二进制便携无第三方 CGO/复杂依赖，这里用纯 Go 扁平解析 YAML 关键字段。
func ParseHydro(files map[string][]byte) (*ParsedPackage, error) {
	var metaData []byte
	for _, name := range []string{"problem.yaml", "problem.yml"} {
		if d, ok := files[name]; ok {
			metaData = d
			break
		}
	}
	if len(metaData) == 0 {
		return nil, fmt.Errorf("problem.yaml not found")
	}

	meta := parseSimpleYAML(string(metaData))
	title := meta["title"]
	if title == "" {
		title = "Hydro Problem"
	}

	// 题面 Markdown
	statement := "请在此输入题面描述"
	for _, name := range []string{"problem_zh.md", "problem.md", "README.md", "readme.md"} {
		if d, ok := files[name]; ok {
			statement = string(d)
			break
		}
	}

	timeMs := 1000
	memoryMiB := 256
	// 读取 testdata/config.yaml 或根配置
	for _, confName := range []string{"testdata/config.yaml", "testdata/config.yml", "config.yaml"} {
		if d, ok := files[confName]; ok {
			conf := parseSimpleYAML(string(d))
			if tStr, ok := conf["time"]; ok {
				timeMs = parseDurationMs(tStr, timeMs)
			}
			if mStr, ok := conf["memory"]; ok {
				memoryMiB = parseMemoryMiB(mStr, memoryMiB)
			}
			break
		}
	}

	// 提取测试点输入输出
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

	return &ParsedPackage{
		Format: FormatHydro,
		Problem: domain.Problem{
			Title:          title,
			Statement:      statement,
			Input:          "标准输入",
			Output:         "标准输出",
			Constraints:    fmt.Sprintf("时间限制 %d ms，空间限制 %d MiB", timeMs, memoryMiB),
			TimeLimitMs:    timeMs,
			MemoryLimitMiB: memoryMiB,
			CompareMode:    domain.CompareIgnoreSpace,
			TestCases:      testCases,
		},
		TestCases: testCases,
		InFiles:   inFiles,
		OutFiles:  outFiles,
	}, nil
}

func parseSimpleYAML(content string) map[string]string {
	res := make(map[string]string)
	lines := strings.Split(content, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "#") || !strings.Contains(l, ":") {
			continue
		}
		parts := strings.SplitN(l, ":", 2)
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		v = strings.Trim(v, `"'`)
		res[k] = v
	}
	return res
}

func parseDurationMs(val string, dflt int) int {
	val = strings.ToLower(strings.TrimSpace(val))
	if strings.HasSuffix(val, "ms") {
		num, err := strconv.Atoi(strings.TrimSuffix(val, "ms"))
		if err == nil && num > 0 {
			return num
		}
	}
	if strings.HasSuffix(val, "s") {
		num, err := strconv.ParseFloat(strings.TrimSuffix(val, "s"), 64)
		if err == nil && num > 0 {
			return int(num * 1000)
		}
	}
	num, err := strconv.Atoi(val)
	if err == nil && num > 0 {
		return num
	}
	return dflt
}

func parseMemoryMiB(val string, dflt int) int {
	val = strings.ToLower(strings.TrimSpace(val))
	val = strings.TrimSuffix(val, "b")
	val = strings.TrimSuffix(val, "i")
	val = strings.TrimSuffix(val, "m")
	num, err := strconv.Atoi(strings.TrimSpace(val))
	if err == nil && num > 0 {
		return num
	}
	return dflt
}
