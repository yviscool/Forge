package packageio

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/yviscool/forge/internal/domain"
)

func makeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		f, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestParsePolygon(t *testing.T) {
	xml := `<problem>
		<names><name language="chinese" value="A加B问题"/></names>
		<statements><statement path="statements/problem.md"/></statements>
		<judging>
			<testset name="tests">
				<time-limit>2000</time-limit>
				<memory-limit>536870912</memory-limit>
			</testset>
		</judging>
		<files><resources><file path="files/check.cpp"/></resources></files>
	</problem>`

	data := makeZip(t, map[string]string{
		"problem.xml":           xml,
		"statements/problem.md": "# A+B 问题描述",
		"tests/01":              "1 2",
		"tests/01.a":            "3",
		"files/check.cpp":       "// testlib checker",
	})

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if format := DetectFormat(zr); format != FormatPolygon {
		t.Fatalf("detect format = %v, want polygon", format)
	}

	files, err := ReadZipEntries(zr)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := ParsePolygon(files)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Problem.Title != "A加B问题" {
		t.Errorf("title = %s, want A加B问题", pkg.Problem.Title)
	}
	if pkg.Problem.TimeLimitMs != 2000 || pkg.Problem.MemoryLimitMiB != 512 {
		t.Errorf("limits = %d ms / %d MiB", pkg.Problem.TimeLimitMs, pkg.Problem.MemoryLimitMiB)
	}
	if len(pkg.TestCases) != 1 || string(pkg.InFiles["case01.in"]) != "1 2" {
		t.Errorf("cases = %+v", pkg.TestCases)
	}
}

func TestParseHydro(t *testing.T) {
	yaml := `title: 简单数论
pid: P1001`
	conf := `time: 1500ms
memory: 128m`

	data := makeZip(t, map[string]string{
		"problem.yaml":         yaml,
		"README.md":            "# 题面内容",
		"testdata/config.yaml": conf,
		"testdata/1.in":        "10",
		"testdata/1.out":       "20",
	})

	zr, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if format := DetectFormat(zr); format != FormatHydro {
		t.Fatalf("detect format = %v, want hydro", format)
	}
	files, err := ReadZipEntries(zr)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := ParseHydro(files)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Problem.Title != "简单数论" {
		t.Errorf("title = %s", pkg.Problem.Title)
	}
	if pkg.Problem.TimeLimitMs != 1500 || pkg.Problem.MemoryLimitMiB != 128 {
		t.Errorf("limits = %d ms / %d MiB", pkg.Problem.TimeLimitMs, pkg.Problem.MemoryLimitMiB)
	}
	if len(pkg.TestCases) != 1 {
		t.Errorf("test cases = %+v", pkg.TestCases)
	}
}

func TestParseQDUOJ(t *testing.T) {
	probJSON := `{
		"title": "QDUOJ题",
		"description": "求和",
		"input_description": "一行两数",
		"output_description": "一个数",
		"time_limit": 800,
		"memory_limit": 64
	}`
	data := makeZip(t, map[string]string{
		"problem.json": probJSON,
		"1.in":         "4 5",
		"1.out":        "9",
	})

	zr, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if format := DetectFormat(zr); format != FormatQDUOJ {
		t.Fatalf("detect format = %v, want qduoj", format)
	}
	files, _ := ReadZipEntries(zr)
	pkg, err := ParseQDUOJ(files)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Problem.Title != "QDUOJ题" || pkg.Problem.TimeLimitMs != 800 {
		t.Errorf("pkg = %+v", pkg.Problem)
	}
}

func TestParseFPS(t *testing.T) {
	fps := `<fps>
		<item>
			<title>FPS测试题</title>
			<time_limit unit="s">2</time_limit>
			<memory_limit unit="mb">128</memory_limit>
			<description>试题描述</description>
			<input>输入</input>
			<output>输出</output>
			<test_input><![CDATA[1]]></test_input>
			<test_output><![CDATA[2]]></test_output>
		</item>
	</fps>`

	data := makeZip(t, map[string]string{
		"fps.xml": fps,
	})
	zr, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if format := DetectFormat(zr); format != FormatFPS {
		t.Fatalf("detect format = %v, want fps", format)
	}
	files, _ := ReadZipEntries(zr)
	pkg, err := ParseFPS(files)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Problem.Title != "FPS测试题" || pkg.Problem.TimeLimitMs != 2000 {
		t.Errorf("pkg = %+v", pkg.Problem)
	}
}

func TestZipSlipProtection(t *testing.T) {
	data := makeZip(t, map[string]string{
		"../../evil.sh": "echo evil",
	})
	zr, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	_, err := ReadZipEntries(zr)
	if err == nil {
		t.Fatal("zip slip traversal must fail")
	}
}

func TestExportProblemNOIPackage(t *testing.T) {
	prob := domain.Problem{
		Code:           "A",
		Slug:           "add",
		Title:          "两数相加",
		Statement:      "计算 A + B",
		TimeLimitMs:    1000,
		MemoryLimitMiB: 256,
		TestCases: []domain.TestCase{
			{ID: "c1", InputFile: "add1.in", OutputFile: "add1.out", Score: 100},
		},
	}
	inFiles := map[string][]byte{"add1.in": []byte("1 2")}
	outFiles := map[string][]byte{"add1.out": []byte("3")}

	pkgBytes, err := ExportProblemNOIPackage(prob, inFiles, outFiles)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(pkgBytes), int64(len(pkgBytes)))
	if err != nil {
		t.Fatal(err)
	}

	foundConf, foundData, foundDown := false, false, false
	for _, f := range zr.File {
		if f.Name == "add/problem.conf" {
			foundConf = true
		}
		if f.Name == "add/data/add1.in" {
			foundData = true
		}
		if f.Name == "add/down/add.in" {
			foundDown = true
		}
	}
	if !foundConf || !foundData || !foundDown {
		t.Errorf("missing expected files in NOI package: conf=%v, data=%v, down=%v", foundConf, foundData, foundDown)
	}
}
