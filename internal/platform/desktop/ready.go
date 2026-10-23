package desktop

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// WaitForReady 轮询本机 /healthz，等服务就绪后再打开浏览器/托盘。
// 最多等待 timeout；返回是否就绪。
func WaitForReady(healthzURL string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(healthzURL)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return true
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

// PrintJoinCard 在控制台输出教师机加入卡片：局域网地址 + 学生扫码二维码。
func PrintJoinCard(out io.Writer, lanURLs []string, studentPath string) error {
	if len(lanURLs) == 0 {
		fmt.Fprintln(out, "（未检测到局域网地址——服务仅对本机可见；如需考生加入请绑定具体网卡或 0.0.0.0 并开放防火墙）")
		return nil
	}
	fmt.Fprintln(out, "──────────────── 学生加入方式 ────────────────")
	for _, u := range lanURLs {
		fmt.Fprintf(out, "  %s%s\n", strings.TrimSuffix(u, "/"), studentPath)
	}
	best := lanURLs[0]
	qr, err := TerminalQR(best + studentPath)
	if err != nil {
		fmt.Fprintf(out, "  （二维码生成失败: %v）\n", err)
		return nil
	}
	fmt.Fprintln(out, "  考生也可扫描下方二维码加入（首选网络）：")
	fmt.Fprintln(out, qr)
	fmt.Fprintln(out, "──────────────────────────────────────────────")
	return nil
}

// EnsureTrayIcon 供启动自检：托盘图标资源必须可解码（否则降级为无图标托盘）。
func EnsureTrayIcon() error {
	_, err := trayIcon()
	return err
}
