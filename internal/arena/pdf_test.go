package arena

import (
	"strings"
	"testing"
)

func TestRenderProblemHTML(t *testing.T) {
	c := Contest{Name: "2026 CSP-S 冲刺模拟赛"}
	p := Problem{
		Code:           "reverse",
		Title:          "二进制反转",
		Statement:      "给定一个正整数 x，求其二进制表示反转后的数值。",
		Input:          "第一行输入一个正整数 T，表示数据组数。",
		Output:         "对于每组数据输出一行，表示反转后的结果。",
		Constraints:    "1 <= T <= 100, 1 <= x <= 2^31 - 1",
		Examples:       "2\n13\n1\n-----\n11\n1",
		TimeLimitMs:    1000,
		MemoryLimitMiB: 512,
	}

	htmlBytes, err := RenderProblemHTML(c, p)
	if err != nil {
		t.Fatalf("unexpected error rendering HTML: %v", err)
	}

	html := string(htmlBytes)
	if !strings.Contains(html, "【题目描述】") {
		t.Errorf("HTML missing description section")
	}
	if !strings.Contains(html, "reverse.cpp") {
		t.Errorf("HTML missing source file metadata")
	}
	if !strings.Contains(html, "512 MiB") {
		t.Errorf("HTML missing memory limit metadata")
	}
}

func TestBrowserScannerAndPDFGeneration(t *testing.T) {
	browserPath := FindBrowserExecutable()
	if browserPath == "" {
		t.Skip("no browser executable detected on system, skipping headless PDF generation test")
	}

	c := Contest{Name: "2026 CSP-S 冲刺模拟赛"}
	p := Problem{
		Code:           "A",
		Title:          "Sum",
		Statement:      "a + b",
		Input:          "a, b",
		Output:         "sum",
		Constraints:    "1 <= a, b <= 100",
		TimeLimitMs:    1000,
		MemoryLimitMiB: 512,
	}

	pdfBytes, err := GenerateProblemPDF(c, p)
	if err != nil {
		t.Fatalf("PDF generation failed: %v", err)
	}

	if len(pdfBytes) < 100 {
		t.Fatalf("generated PDF too small: %d bytes", len(pdfBytes))
	}

	// Verify standard PDF header (%PDF-1.)
	if !strings.HasPrefix(string(pdfBytes[:8]), "%PDF-") {
		t.Fatalf("invalid PDF magic header: %s", string(pdfBytes[:8]))
	}
}
