package packageio

import (
	"encoding/xml"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/yviscool/forge/internal/domain"
)

type polygonProblemXML struct {
	Names []struct {
		Language string `xml:"language,attr"`
		Value    string `xml:"value,attr"`
	} `xml:"names>name"`
	Statements []struct {
		Language string `xml:"language,attr"`
		Path     string `xml:"path,attr"`
		Type     string `xml:"type,attr"`
	} `xml:"statements>statement"`
	Judging struct {
		Testsets []struct {
			Name        string `xml:"name,attr"`
			TimeLimit   int    `xml:"time-limit"`
			MemoryLimit int64  `xml:"memory-limit"`
			Tests       []struct {
				Cmd    string `xml:"cmd,attr"`
				Method string `xml:"method,attr"`
			} `xml:"tests>test"`
		} `xml:"testset"`
	} `xml:"judging"`
	Files struct {
		Resources []struct {
			Path string `xml:"path,attr"`
			Type string `xml:"type,attr"`
		} `xml:"resources>file"`
		Executables []struct {
			Source struct {
				Path string `xml:"path,attr"`
				Type string `xml:"type,attr"`
			} `xml:"source"`
		} `xml:"executables>executable"`
	} `xml:"files"`
}

// ParsePolygon 解析 Codeforces Polygon 导出的试题包。
func ParsePolygon(files map[string][]byte) (*ParsedPackage, error) {
	xmlData, ok := files["problem.xml"]
	if !ok {
		return nil, fmt.Errorf("problem.xml not found")
	}

	var meta polygonProblemXML
	if err := xml.Unmarshal(xmlData, &meta); err != nil {
		return nil, fmt.Errorf("unmarshal problem.xml: %w", err)
	}

	title := "Polygon Problem"
	for _, n := range meta.Names {
		if n.Language == "chinese" || n.Language == "zh" {
			title = n.Value
			break
		}
		if n.Value != "" && title == "Polygon Problem" {
			title = n.Value
		}
	}

	timeMs := 1000
	memoryMiB := 256
	for _, ts := range meta.Judging.Testsets {
		if ts.TimeLimit > 0 {
			timeMs = ts.TimeLimit
		}
		if ts.MemoryLimit > 0 {
			memoryMiB = int(ts.MemoryLimit / (1024 * 1024))
			if memoryMiB <= 0 {
				memoryMiB = 256
			}
		}
		break
	}

	statement := "请在此输入题面描述"
	for _, st := range meta.Statements {
		p := path.Clean(st.Path)
		if data, exists := files[p]; exists {
			statement = string(data)
			break
		}
	}

	// 提取测试点数据（tests/01, tests/01.a）
	inFiles := make(map[string][]byte)
	outFiles := make(map[string][]byte)
	var testCases []domain.TestCase

	// 收集 tests/ 下的所有输入输出
	inputs := make(map[string]string)
	outputs := make(map[string]string)
	for fname := range files {
		if strings.HasPrefix(fname, "tests/") {
			base := strings.TrimPrefix(fname, "tests/")
			if strings.HasSuffix(base, ".a") {
				stem := strings.TrimSuffix(base, ".a")
				outputs[stem] = fname
			} else if !strings.Contains(base, ".") {
				inputs[base] = fname
			}
		}
	}

	// 排序
	var stems []string
	for s := range inputs {
		if _, ok := outputs[s]; ok {
			stems = append(stems, s)
		}
	}
	sort.Slice(stems, func(i, j int) bool {
		ai, errA := strconv.Atoi(stems[i])
		bi, errB := strconv.Atoi(stems[j])
		if errA == nil && errB == nil {
			return ai < bi
		}
		return stems[i] < stems[j]
	})

	scorePerCase := 100
	if len(stems) > 0 {
		scorePerCase = 100 / len(stems)
	}

	for idx, s := range stems {
		inName := fmt.Sprintf("case%02d.in", idx+1)
		outName := fmt.Sprintf("case%02d.out", idx+1)
		inFiles[inName] = files[inputs[s]]
		outFiles[outName] = files[outputs[s]]

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

	var checkerCode string
	var checkerLang string
	for fname, data := range files {
		if (strings.HasPrefix(fname, "files/check") || strings.HasPrefix(fname, "files/checker")) && strings.HasSuffix(fname, ".cpp") {
			checkerCode = string(data)
			checkerLang = "cpp"
			break
		}
	}

	compareMode := domain.CompareIgnoreSpace
	if checkerCode != "" {
		compareMode = domain.CompareTestlibSpecialJudge
	}

	return &ParsedPackage{
		Format: FormatPolygon,
		Problem: domain.Problem{
			Title:          title,
			Statement:      statement,
			Input:          "标准输入",
			Output:         "标准输出",
			Constraints:    fmt.Sprintf("时间限制 %d ms，空间限制 %d MiB", timeMs, memoryMiB),
			TimeLimitMs:    timeMs,
			MemoryLimitMiB: memoryMiB,
			CompareMode:    compareMode,
			CheckerCode:    checkerCode,
			CheckerLang:    checkerLang,
			TestCases:      testCases,
		},
		TestCases:   testCases,
		InFiles:     inFiles,
		OutFiles:    outFiles,
		CheckerCode: checkerCode,
		CheckerLang: checkerLang,
	}, nil
}
