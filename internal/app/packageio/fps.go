package packageio

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/yviscool/forge/internal/domain"
)

type fpsXML struct {
	Items []fpsItem `xml:"item"`
}

type fpsItem struct {
	Title         string   `xml:"title"`
	TimeLimit     string   `xml:"time_limit"`
	TimeLimitUnit string   `xml:"time_limit,attr"`
	MemoryLimit   string   `xml:"memory_limit"`
	Description   string   `xml:"description"`
	Input         string   `xml:"input"`
	Output        string   `xml:"output"`
	SampleInput   string   `xml:"sample_input"`
	SampleOutput  string   `xml:"sample_output"`
	Hint          string   `xml:"hint"`
	TestInputs    []string `xml:"test_input"`
	TestOutputs   []string `xml:"test_output"`
	Spj           string   `xml:"spj"`
}

// ParseFPS 解析 HUSTOJ / Free Problem Set (FPS) XML 格式题包。
func ParseFPS(files map[string][]byte) (*ParsedPackage, error) {
	var xmlData []byte
	for name, d := range files {
		if name == "fps.xml" || strings.HasSuffix(name, ".xml") {
			xmlData = d
			break
		}
	}
	if len(xmlData) == 0 {
		return nil, fmt.Errorf("fps XML not found")
	}

	var fps fpsXML
	if err := xml.Unmarshal(xmlData, &fps); err != nil {
		return nil, fmt.Errorf("unmarshal fps.xml: %w", err)
	}
	if len(fps.Items) == 0 {
		return nil, fmt.Errorf("fps.xml contains no problem items")
	}

	// 默认取第一题
	item := fps.Items[0]

	timeMs := parseDurationMs(item.TimeLimit, 1000)
	if strings.ToLower(item.TimeLimitUnit) == "s" || (timeMs > 0 && timeMs <= 10) {
		timeMs *= 1000
	}
	memoryMiB := parseMemoryMiB(item.MemoryLimit, 256)

	statement := item.Description
	if statement == "" {
		statement = "请在此输入题面描述"
	}
	inputSpec := item.Input
	if inputSpec == "" {
		inputSpec = "标准输入"
	}
	outputSpec := item.Output
	if outputSpec == "" {
		outputSpec = "标准输出"
	}
	constraints := item.Hint
	if constraints == "" {
		constraints = fmt.Sprintf("时间限制 %d ms，空间限制 %d MiB", timeMs, memoryMiB)
	}

	var examples string
	if item.SampleInput != "" || item.SampleOutput != "" {
		examples = fmt.Sprintf("### 样例 1\n**输入**:\n```\n%s\n```\n**输出**:\n```\n%s\n```\n",
			item.SampleInput, item.SampleOutput)
	}

	inFiles := make(map[string][]byte)
	outFiles := make(map[string][]byte)
	var testCases []domain.TestCase

	count := len(item.TestInputs)
	if len(item.TestOutputs) < count {
		count = len(item.TestOutputs)
	}

	scorePerCase := 100
	if count > 0 {
		scorePerCase = 100 / count
	}

	for i := 0; i < count; i++ {
		inName := fmt.Sprintf("case%02d.in", i+1)
		outName := fmt.Sprintf("case%02d.out", i+1)
		inFiles[inName] = []byte(item.TestInputs[i])
		outFiles[outName] = []byte(item.TestOutputs[i])

		testCases = append(testCases, domain.TestCase{
			ID:          fmt.Sprintf("case%02d", i+1),
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
	var checkerCode string
	if item.Spj != "" {
		compareMode = domain.CompareSpecialJudge
		checkerCode = item.Spj
	}

	return &ParsedPackage{
		Format: FormatFPS,
		Problem: domain.Problem{
			Title:          item.Title,
			Statement:      statement,
			Input:          inputSpec,
			Output:         outputSpec,
			Constraints:    constraints,
			Examples:       examples,
			TimeLimitMs:    timeMs,
			MemoryLimitMiB: memoryMiB,
			CompareMode:    compareMode,
			CheckerCode:    checkerCode,
			CheckerLang:    "cpp",
			TestCases:      testCases,
		},
		TestCases:   testCases,
		InFiles:     inFiles,
		OutFiles:    outFiles,
		CheckerCode: checkerCode,
		CheckerLang: "cpp",
	}, nil
}
