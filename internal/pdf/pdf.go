// Package pdf CCF A4 导出基石：模板文件化 + 浏览器探测 + 两级降级。
// Tier1 直接返回 HTML（Ctrl+P），Tier2 走 Edge/Chrome headless 直出 PDF。
package pdf

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/yviscool/forge/internal/domain"
)

//go:embed template.html
var templateHTML string

// RenderHTML 渲染 CCF 规范打印 HTML。
func RenderHTML(c domain.Contest, p domain.Problem) ([]byte, error) {
	t, err := template.New("ccf").Parse(templateHTML)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, map[string]any{"Contest": c, "Problem": p}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// FindBrowser 探测系统 Edge/Chrome/Chromium。
func FindBrowser() string {
	var candidates []string
	if runtime.GOOS == "windows" {
		candidates = append(candidates,
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		)
		if lad := os.Getenv("LOCALAPPDATA"); lad != "" {
			candidates = append(candidates,
				filepath.Join(lad, "Google", "Chrome", "Application", "chrome.exe"),
				filepath.Join(lad, "Microsoft", "Edge", "Application", "msedge.exe"),
			)
			if m, _ := filepath.Glob(filepath.Join(lad, "ms-playwright", "chromium-*", "chrome-win", "chrome.exe")); len(m) > 0 {
				candidates = append(candidates, m...)
			}
		}
	} else {
		candidates = append(candidates,
			"/usr/bin/google-chrome", "/usr/bin/chromium",
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		)
	}
	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	for _, n := range []string{"chrome", "msedge", "google-chrome", "chromium"} {
		if p, err := exec.LookPath(n); err == nil {
			return p
		}
	}
	return ""
}

// GeneratePDF HTML → A4 PDF，无浏览器时返回明确错误并由调用方降级到 HTML。
func GeneratePDF(c domain.Contest, p domain.Problem) ([]byte, error) {
	bin := FindBrowser()
	if bin == "" {
		return nil, fmt.Errorf("no Chromium/Chrome/Edge browser executable found on system")
	}
	html, err := RenderHTML(c, p)
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "forge_pdf_*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	htmlFile, pdfFile := filepath.Join(dir, "problem.html"), filepath.Join(dir, "output.pdf")
	if err := os.WriteFile(htmlFile, html, 0644); err != nil {
		return nil, err
	}
	cmd := exec.Command(bin, "--headless", "--disable-gpu", "--no-pdf-header-footer",
		fmt.Sprintf("--print-to-pdf=%s", pdfFile), htmlFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("browser pdf generation failed: %w (output: %s)", err, string(out))
	}
	return os.ReadFile(pdfFile)
}
