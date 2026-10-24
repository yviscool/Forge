package desktop

import (
	"bytes"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func TestLANURLsWildcard(t *testing.T) {
	urls := LANURLs("0.0.0.0:8080", "/app")
	if len(urls) == 0 {
		t.Skip("host has no global IPv4 unicast address; wildcard discovery untestable")
	}
	for _, u := range urls {
		if !strings.HasPrefix(u, "http://") || !strings.Contains(u, ":8080/app") {
			t.Fatalf("unexpected lan url %q", u)
		}
		if strings.Contains(u, "127.0.0.1") {
			t.Fatalf("loopback must not appear in LAN list: %q", u)
		}
	}
}

func TestLANURLsSpecificBinding(t *testing.T) {
	if got := LANURLs("127.0.0.1:8080", "/app"); len(got) != 0 {
		t.Fatalf("loopback binding must not expose LAN urls: %v", got)
	}
	// 绑定具体内网地址时只暴露该地址。
	urls := LANURLs("192.0.2.10:9000", "/teacher")
	if len(urls) != 1 || urls[0] != "http://192.0.2.10:9000/teacher" {
		t.Fatalf("specific binding urls = %v", urls)
	}
}

func TestLocalURL(t *testing.T) {
	cases := []struct{ addr, path, want string }{
		{":8080", "/teacher", "http://127.0.0.1:8080/teacher"},
		{"0.0.0.0:80", "/app", "http://127.0.0.1:80/app"},
		{"192.168.1.5:9000", "/app", "http://192.168.1.5:9000/app"},
	}
	for _, c := range cases {
		if got := LocalURL(c.addr, c.path); got != c.want {
			t.Errorf("LocalURL(%q,%q) = %q, want %q", c.addr, c.path, got, c.want)
		}
	}
}

func TestSystemBrowserCommandConstruction(t *testing.T) {
	lookPath := func(file string) (string, error) { return "C:/mock/" + file, nil }
	var gotArgs []string
	browser := SystemBrowser{
		LookPath: lookPath,
		Start: func(cmd *exec.Cmd) error {
			gotArgs = cmd.Args
			return nil
		},
	}
	if err := browser.Open("http://127.0.0.1:8080/teacher"); err != nil {
		t.Fatal(err)
	}
	switch runtime.GOOS {
	case "windows":
		if gotArgs[0] != `C:\Windows\System32\rundll32.exe` {
			t.Fatalf("windows opener = %v", gotArgs)
		}
		if gotArgs[1] != "url.dll,FileProtocolHandler" || gotArgs[2] != "http://127.0.0.1:8080/teacher" {
			t.Fatalf("windows args = %v", gotArgs)
		}
	case "darwin":
		if gotArgs[0] != "open" || gotArgs[1] != "http://127.0.0.1:8080/teacher" {
			t.Fatalf("darwin args = %v", gotArgs)
		}
	default:
		if gotArgs[0] != "xdg-open" {
			t.Fatalf("linux args = %v", gotArgs)
		}
	}
	// 空地址拒绝。
	if err := (SystemBrowser{LookPath: lookPath}).Open(""); err == nil {
		t.Fatal("empty url must fail")
	}
	// Supported 不做真实调用。
	if !browser.Supported() {
		t.Fatal("supported should hold with a stubbed lookpath")
	}
}

func TestTerminalQRAndJoinCard(t *testing.T) {
	qr, err := TerminalQR("http://192.0.2.10:8080/app")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(qr, "█") {
		t.Fatalf("terminal QR should be block-art, got %.40q", qr)
	}
	if _, err := TerminalQR(""); err == nil {
		t.Fatal("empty content must fail")
	}

	out := &bytes.Buffer{}
	if err := PrintJoinCard(out, []string{"http://192.0.2.10:8080"}, "/app"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "http://192.0.2.10:8080/app") {
		t.Fatalf("join card missing url: %s", out.String())
	}
	if !strings.Contains(out.String(), "█") {
		t.Fatal("join card should embed a QR code")
	}

	out.Reset()
	if err := PrintJoinCard(out, nil, "/app"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "未检测到局域网地址") {
		t.Fatalf("empty lan card = %s", out.String())
	}
}

func TestTrayIconAsset(t *testing.T) {
	ico, err := trayIcon()
	if err != nil {
		t.Fatal(err)
	}
	// ICO 头：类型 1（图标）+ 3 个尺寸条目（16/32/48）。
	if ico[2] != 1 || ico[3] != 0 || ico[4] != 3 || ico[5] != 0 {
		t.Fatalf("ico header = %v", ico[:6])
	}
	// BMP 载荷：40 字节 BITMAPINFOHEADER 后紧跟像素（自底向上）。
	if len(ico) < 22+40 {
		t.Fatal("ico payload too small")
	}
	if err := EnsureTrayIcon(); err != nil {
		t.Fatalf("ensure icon: %v", err)
	}
}

// 托盘面板曾用 <a target="_blank"> 做「去」动作，WebView2 会把它交给系统
// 默认浏览器（Edge），于是托盘里点「打开教师台」弹出一个带地址栏的假控制台。
// 面板必须是原生窗口指令驱动，永远不出现新窗口语义。
func TestTrayPanelNeverOpensExternalWindows(t *testing.T) {
	if strings.Contains(trayPanelHTML, `target="_blank"`) {
		t.Fatal("tray panel must not use target=_blank: WebView2 hands it to the system browser")
	}
	for _, want := range []string{
		"data-act=",
		"/ui/command",
		"window.open = function",
		"native=1",
	} {
		if !strings.Contains(trayPanelHTML, want) {
			t.Fatalf("tray panel missing %q: actions must drive native windows via /ui/command", want)
		}
	}
	// 四个「去」动作必须都在外壳指令白名单里，否则 /ui/command 会 400。
	for _, action := range []string{"show-main", "open-bigscreen", "open-join", "open-student"} {
		if !strings.Contains(trayPanelHTML, `"`+action+`"`) {
			t.Fatalf("tray panel action %q missing", action)
		}
	}
}
