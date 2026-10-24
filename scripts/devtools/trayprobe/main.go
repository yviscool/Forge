package main

// 面板验证台：在另一个端口上用「当前源码里的」面板 HTML 回应 /tray，
// 其余请求全部反代到正在运行的 Forge 实例（默认 127.0.0.1:8080）。
// 这样能在不重启教师机实例的前提下，验证新版面板对真实外壳发指令。
//
//	go run ./scripts/devtools/trayprobe
//	go run ./scripts/devtools/trayprobe -addr 127.0.0.1:18099 -upstream http://127.0.0.1:8080
import (
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/yviscool/forge/internal/platform/desktop"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:18099", "listen address")
	upstream := flag.String("upstream", "http://127.0.0.1:8080", "running forge instance")
	flag.Parse()

	target, err := url.Parse(*upstream)
	if err != nil {
		log.Fatalf("bad upstream: %v", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.FlushInterval = -1 // SSE 实时流

	// JoinPageHandler 接管 /tray、/tray-launcher、/join，其余原样透传。
	h := desktop.JoinPageHandler(proxy, *addr)
	log.Printf("tray probe on http://%s/tray?native=1 -> %s", *addr, *upstream)
	log.Fatal(http.ListenAndServe(*addr, h))
}
