// Package desktop 提供教师机的「桌面模式」体验（参考 upstream/magpie 的纯 Go 路径）：
// 局域网地址自动发现、系统托盘、自动打开浏览器与考生扫码加入二维码。
// 全部为可选增强：不开启 --desktop 时 Forge 与普通 headless 服务器完全一致。
package desktop

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// LANURLs 枚举本机非环回 IPv4 地址，生成本服务对局域网暴露的访问地址。
// listenAddr 是服务监听地址（如 ":8080" 或 "0.0.0.0:8080"）；
// 仅当监听在通配地址或本机 IP 上时才对局域网可见。
func LANURLs(listenAddr, path string) []string {
	host, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		// 无端口信息则无法生成对外地址。
		return nil
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil && !ip.IsUnspecified() {
		// 绑定在具体地址上：只暴露该地址（环回不进局域网清单）。
		if ip.IsLoopback() {
			return nil
		}
		return []string{lanURL(ip.String(), port, path)}
	}
	var out []string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok || !ipnet.IP.IsGlobalUnicast() || ipnet.IP.To4() == nil {
			continue
		}
		// 跳过链路本地（169.254.*）与虚拟网卡常见网段之外的本机回环。
		if ipnet.IP.IsLoopback() || ipnet.IP.IsLinkLocalUnicast() {
			continue
		}
		out = append(out, lanURL(ipnet.IP.String(), port, path))
	}
	return out
}

func lanURL(ip, port, path string) string {
	clean := "/" + strings.TrimPrefix(path, "/")
	if clean == "/" {
		clean = ""
	}
	return fmt.Sprintf("http://%s:%s%s", ip, port, clean)
}

// LocalURL 把监听地址转为本机可访问的 URL（0.0.0.0 → localhost）。
func LocalURL(listenAddr, path string) string {
	host, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return path
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	clean := "/" + strings.TrimPrefix(path, "/")
	if clean == "/" {
		clean = ""
	}
	return fmt.Sprintf("http://%s%s", net.JoinHostPort(host, port), clean)
}

// SafePath 校验并规整 URL 路径片段，防止拼接出异常链接。
func SafePath(path string) string {
	u, err := url.Parse("/" + strings.TrimPrefix(path, "/"))
	if err != nil {
		return "/"
	}
	return u.String()
}
