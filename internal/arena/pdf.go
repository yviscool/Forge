package arena

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// FindBrowserExecutable searches for Chromium/Edge/Chrome in standard system paths.
func FindBrowserExecutable() string {
	candidates := []string{}

	if runtime.GOOS == "windows" {
		candidates = append(candidates,
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		)
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData != "" {
			candidates = append(candidates,
				filepath.Join(localAppData, "Google", "Chrome", "Application", "chrome.exe"),
				filepath.Join(localAppData, "Microsoft", "Edge", "Application", "msedge.exe"),
			)
			// Check Playwright installation paths if present
			matches, _ := filepath.Glob(filepath.Join(localAppData, "ms-playwright", "chromium-*", "chrome-win64", "chrome.exe"))
			candidates = append(candidates, matches...)
		}
	} else if runtime.GOOS == "darwin" {
		candidates = append(candidates,
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		)
	} else {
		candidates = append(candidates,
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/snap/bin/chromium",
		)
	}

	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}

	// Try PATH lookups
	for _, name := range []string{"chrome", "msedge", "google-chrome", "chromium", "chromium-browser"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}

	return ""
}

// RenderProblemHTML renders the CCF CSP-compliant HTML template for a given problem and contest.
func RenderProblemHTML(c Contest, p Problem) ([]byte, error) {
	tmplStr := `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<title>{{.Problem.Title}} - CCF CSP 试题规范</title>
<style>
@page {
  size: A4;
  margin: 16mm 20mm 17mm;
}
body {
  font-family: "SimSun", "Songti SC", "STSong", serif;
  font-size: 10.5pt;
  line-height: 1.6;
  color: #111827;
  margin: 0;
}
.header {
  border-bottom: 0.8px solid #111;
  padding-bottom: 4px;
  margin-bottom: 16px;
  display: flex;
  justify-content: space-between;
  font-size: 9pt;
  color: #4b5563;
}
h1 {
  font-size: 16pt;
  font-weight: bold;
  text-align: center;
  margin-top: 0;
  margin-bottom: 8px;
}
.meta-table {
  width: 100%;
  border-collapse: collapse;
  margin: 12px 0 20px;
  font-size: 9.5pt;
}
.meta-table th, .meta-table td {
  border: 1px solid #374151;
  padding: 6px 10px;
  text-align: center;
}
.meta-table th {
  background: #f3f4f6;
  font-weight: 600;
}
.section-title {
  font-size: 11pt;
  font-weight: bold;
  margin-top: 14px;
  margin-bottom: 4px;
}
p {
  margin: 4px 0 8px;
  text-indent: 2em;
}
.sample-box {
  border: 0.8px solid #405dff;
  border-radius: 4px;
  background: #fafafa;
  margin: 8px 0 14px;
  overflow: hidden;
}
.sample-header {
  background: #eef2ff;
  color: #312e81;
  font-weight: 600;
  font-size: 9pt;
  padding: 4px 10px;
  border-bottom: 0.6px solid #405dff;
}
pre {
  margin: 0;
  padding: 8px 12px;
  font-family: "Consolas", monospace;
  font-size: 9.5pt;
  line-height: 1.45;
  white-space: pre-wrap;
  word-break: break-all;
}
.tip {
  font-family: "KaiTi", "STKaiti", serif;
  color: #374151;
}
@media print {
  .no-print { display: none; }
}
</style>
</head>
<body>
<div class="header">
  <span>{{.Contest.Name}}</span>
  <span>入门级 / 提高级 · {{.Problem.Title}}（{{.Problem.Code}}）</span>
</div>

<h1>{{.Problem.Title}} ({{.Problem.Code}})</h1>

<table class="meta-table">
  <tr>
    <th>题目名称</th>
    <th>题目目录</th>
    <th>源程序文件名</th>
    <th>输入文件名</th>
    <th>输出文件名</th>
    <th>时间限制</th>
    <th>内存限制</th>
  </tr>
  <tr>
    <td>{{.Problem.Title}}</td>
    <td>{{.Problem.Code}}</td>
    <td>{{.Problem.Code}}.cpp</td>
    <td>{{.Problem.Code}}.in</td>
    <td>{{.Problem.Code}}.out</td>
    <td>{{.Problem.TimeLimitMs}} ms</td>
    <td>{{.Problem.MemoryLimitMiB}} MiB</td>
  </tr>
</table>

<div class="section-title">【题目描述】</div>
<p>{{.Problem.Statement}}</p>

<div class="section-title">【输入格式】</div>
<p>{{.Problem.Input}}</p>

<div class="section-title">【输出格式】</div>
<p>{{.Problem.Output}}</p>

<div class="sample-box">
  <div class="sample-header">样例 1 输入 / 输出</div>
  <pre>{{.Problem.Examples}}</pre>
</div>

<div class="section-title">【数据范围与约束】</div>
<p class="tip">{{.Problem.Constraints}}</p>

</body>
</html>`
	t, err := template.New("ccf_pdf").Parse(tmplStr)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, map[string]any{"Contest": c, "Problem": p}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GenerateProblemPDF renders problem HTML and converts it to a standard A4 PDF using the local browser engine.
func GenerateProblemPDF(c Contest, p Problem) ([]byte, error) {
	browserPath := FindBrowserExecutable()
	if browserPath == "" {
		return nil, fmt.Errorf("no Chromium/Chrome/Edge browser executable found on system")
	}

	htmlContent, err := RenderProblemHTML(c, p)
	if err != nil {
		return nil, err
	}

	// Create temp directory for conversion
	tmpDir, err := os.MkdirTemp("", "forge_pdf_*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	htmlFile := filepath.Join(tmpDir, "problem.html")
	pdfFile := filepath.Join(tmpDir, "output.pdf")

	if err := os.WriteFile(htmlFile, htmlContent, 0644); err != nil {
		return nil, err
	}

	// Run headless browser print-to-pdf command
	cmd := exec.Command(browserPath,
		"--headless",
		"--disable-gpu",
		"--no-pdf-header-footer",
		fmt.Sprintf("--print-to-pdf=%s", pdfFile),
		htmlFile,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("browser pdf generation failed: %w (output: %s)", err, string(out))
	}

	return os.ReadFile(pdfFile)
}
