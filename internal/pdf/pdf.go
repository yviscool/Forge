// Package pdf CCF A4 导出基石：模板文件化 + 浏览器探测 + 两级降级。
// Tier1 直接返回 HTML（Ctrl+P），Tier2 走 Edge/Chrome headless 直出 PDF。
package pdf

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/yviscool/forge/internal/domain"
)

//go:embed template.html
var templateHTML string

//go:embed contest_template.html
var contestTemplateHTML string

const (
	maxConcurrentPDFRenders = 2
	pdfRenderTimeout        = 90 * time.Second
	maxBrowserOutputBytes   = 64 << 10
)

var pdfRenderSlots = make(chan struct{}, maxConcurrentPDFRenders)

// RenderHTML 渲染单题 CCF 规范打印 HTML。
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

// RenderContestHTML 渲染整场比赛全卷合订本（4 题或多题，带封面与考生守则）HTML。
func RenderContestHTML(c domain.Contest, probs []domain.Problem) ([]byte, error) {
	t, err := template.New("ccf_contest").Parse(contestTemplateHTML)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, map[string]any{"Contest": c, "Problems": probs}); err != nil {
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
	return GeneratePDFContext(context.Background(), c, p)
}

// GeneratePDFContext bounds browser concurrency and respects request
// cancellation. Chromium is expensive and must never be launched without a
// finite deadline or a process-wide concurrency ceiling.
func GeneratePDFContext(ctx context.Context, c domain.Contest, p domain.Problem) ([]byte, error) {
	bin := FindBrowser()
	if bin == "" {
		return nil, fmt.Errorf("no Chromium/Chrome/Edge browser executable found on system")
	}
	html, err := RenderHTML(c, p)
	if err != nil {
		return nil, err
	}
	return runBrowserPDF(ctx, bin, "forge_pdf_", "problem.html", "output.pdf", html)
}

// GenerateContestPDF 整场比赛合订本 HTML → A4 PDF。
func GenerateContestPDF(c domain.Contest, probs []domain.Problem) ([]byte, error) {
	return GenerateContestPDFContext(context.Background(), c, probs)
}

// GenerateContestPDFContext is the request-aware full-paper variant.
func GenerateContestPDFContext(ctx context.Context, c domain.Contest, probs []domain.Problem) ([]byte, error) {
	bin := FindBrowser()
	if bin == "" {
		return nil, fmt.Errorf("no Chromium/Chrome/Edge browser executable found on system")
	}
	html, err := RenderContestHTML(c, probs)
	if err != nil {
		return nil, err
	}
	return runBrowserPDF(ctx, bin, "forge_contest_pdf_", "contest.html", "output.pdf", html)
}

func runBrowserPDF(
	ctx context.Context,
	bin, tempPattern, htmlName, pdfName string,
	html []byte,
) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, pdfRenderTimeout)
	defer cancel()
	select {
	case pdfRenderSlots <- struct{}{}:
		defer func() { <-pdfRenderSlots }()
	case <-ctx.Done():
		return nil, fmt.Errorf("pdf renderer unavailable: %w", ctx.Err())
	}

	dir, err := os.MkdirTemp("", tempPattern)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	htmlFile, pdfFile := filepath.Join(dir, htmlName), filepath.Join(dir, pdfName)
	if err := os.WriteFile(htmlFile, html, 0644); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, bin,
		"--headless",
		"--disable-gpu",
		"--disable-background-networking",
		"--disable-default-apps",
		"--disable-extensions",
		"--disable-sync",
		"--no-first-run",
		"--no-pdf-header-footer",
		fmt.Sprintf("--print-to-pdf=%s", pdfFile), htmlFile)
	out := &cappedBuffer{limit: maxBrowserOutputBytes}
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("browser pdf generation timed out: %w", ctxErr)
		}
		return nil, fmt.Errorf("browser pdf generation failed: %w (output: %s)",
			err, out.String())
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("browser pdf generation timed out: %w", err)
	}
	return os.ReadFile(pdfFile)
}

type cappedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	room := b.limit - b.buf.Len()
	if room <= 0 {
		b.truncated = true
		return len(p), nil
	}
	if len(p) > room {
		_, _ = b.buf.Write(p[:room])
		b.truncated = true
		return len(p), nil
	}
	return b.buf.Write(p)
}

func (b *cappedBuffer) String() string {
	if !b.truncated {
		return b.buf.String()
	}
	return b.buf.String() + "\n[output truncated]"
}
