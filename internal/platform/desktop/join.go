package desktop

import (
	"bytes"
	"encoding/base64"
	"net/http"

	"github.com/skip2/go-qrcode"
)

func firstOrLocal(lan []string, listenAddr string) string {
	if len(lan) > 0 {
		return lan[0]
	}
	return LocalURL(listenAddr, "/app")
}

// JoinPageHandler 提供免登录的 GET /join 学生扫码页：
// 页面列出本服务在局域网内的可达地址，并内嵌学生端地址的二维码。
// 其余请求原样透传给 Forge 主服务。windowsgui 构建没有控制台，
// 这张页面同时承担了「学生加入方式」卡片的可见形态。
func JoinPageHandler(next http.Handler, listenAddr string) http.Handler {
	aux := &auxPages{listenAddr: listenAddr}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/join":
			aux.serveJoin(w, r)
			return
		case "/tray":
			aux.serveTrayPanel(w, r)
			return
		case "/tray-launcher":
			aux.serveTrayLauncher(w, r)
			return
		default:
			next.ServeHTTP(w, r)
			return
		}
	})
}

// auxPages 辅助页面的公共状态（监听地址决定 LAN 地址清单）。
type auxPages struct {
	listenAddr string
}

// serveTrayLauncher 在一个小尺寸弹窗里打开速览面板（托盘菜单入口），
// 启动器自身立即关闭。
func (h *auxPages) serveTrayLauncher(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!doctype html><meta charset="utf-8">
<script>
window.open("/tray", "forge-tray", "width=400,height=720,menubar=no,toolbar=no,location=no,status=no");
window.close();
</script>`))
}

func (h *auxPages) serveTrayPanel(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(trayPanelHTML))
}

func (h *auxPages) serveJoin(w http.ResponseWriter, _ *http.Request) {
	lan := LANURLs(h.listenAddr, "/app")
	best := firstOrLocal(lan, h.listenAddr)
	code, qerr := qrcode.New(best, qrcode.Medium)
	var png []byte
	var err error = qerr
	if err == nil {
		png, err = code.PNG(320)
	}
	qrHTML := ""
	if err == nil {
		qrHTML = `<img alt="学生端二维码" src="data:image/png;base64,` +
			base64.StdEncoding.EncodeToString(png) + `" width="320" height="320" />`
	}
	rows := &bytes.Buffer{}
	if len(lan) == 0 {
		rows.WriteString(`<li>本服务仅对本机可见（未检测到局域网地址）。</li>`)
	} else {
		for _, u := range lan {
			rows.WriteString(`<li><a href="` + u + `">` + u + `</a></li>`)
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(joinHTML(rows.String(), best, qrHTML)))
}

func joinHTML(listItems, best, qrHTML string) string {
	return `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<title>Forge · 学生加入方式</title>
<style>
  body { margin: 0; min-height: 100vh; display: grid; place-items: center;
         background: #0b1520; color: #e6edf5; font-family: "Microsoft YaHei", sans-serif; }
  .card { max-width: 560px; padding: 40px; text-align: center;
          background: rgba(255,255,255,.04); border: 1px solid rgba(255,255,255,.1); border-radius: 24px; }
  h1 { font-size: 20px; margin: 0 0 8px; }
  p.sub { color: #8fa3b8; font-size: 13px; margin: 0 0 24px; }
  img { background: #fff; border-radius: 16px; padding: 12px; }
  ul { list-style: none; padding: 0; margin: 24px 0 0; font-size: 14px; }
  li { margin: 8px 0; }
  a { color: #7fb2ff; text-decoration: none; font-family: Consolas, monospace; }
  .hint { color: #8fa3b8; font-size: 12px; margin-top: 20px; }
</style>
</head>
<body>
  <div class="card">
    <h1>Forge 考场 · 学生加入</h1>
    <p class="sub">手机扫码，或在同一局域网的电脑浏览器打开下方地址，登录后即可作答。</p>
    ` + qrHTML + `
    <ul>` + listItems + `</ul>
    <p class="hint">在投屏窗口中请直接展示二维码或口头念出地址；链接为 LAN 直达地址。</p>
    <p class="hint">当前展示首选地址：` + best + `（教师机如更换网络请刷新本页）</p>
  </div>
</body>
</html>`
}
