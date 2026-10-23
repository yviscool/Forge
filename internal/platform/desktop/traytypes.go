package desktop

// TrayURLs 托盘菜单需要的一组地址（桌面模式共用，tray 实现按构建标签切换）。
type TrayURLs struct {
	Teacher   string   // 教师台（本机）
	Student   string   // 学生端（本机）
	StudentQR string   // 学生端加入二维码内容（首选局域网地址）
	LAN       []string // 局域网可达地址清单
}

// TrayHooks 托盘行为回调（由 main 注入：打开浏览器/优雅停机）。
type TrayHooks struct {
	Browser BrowserOpener
	OnQuit  func()
}
