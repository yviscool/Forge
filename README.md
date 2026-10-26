# Forge 竞赛工坊

Forge 是面向学校、集训队和竞赛机构的现代信息学竞赛平台。后端使用 Go
单二进制提供 API、SQLite 持久化、评测调度、SSE 实时事件和嵌入式 SPA；前端使用
React 19、TypeScript、Vite、Tailwind CSS 与 Monaco Editor 构建。

## 产品能力

- 多比赛并行管理：草稿、进行中、已结束生命周期相互隔离。
- 用户与编组：独立账号体系、批量 CSV 导入、临时准考证、密码重置。
- 多种题型：传统编程题、只交答案题、交互题、GESP 单选/多选/判断题。
- 题面与数据：Markdown/KaTeX、双语题面、测试数据文件仓、子任务 DAG 与限额。
- 评测：工具链探测、编译缓存、逐点结果、OI/ACM 榜单、重判与人工复核。
- 考场安全：严格时间窗、IP 机位绑定、试卷口令、定向广播、提交硬截止。
- 交付：CCF A4 PDF/HTML、成绩 CSV、完整比赛包导入导出。
- 可观测性：健康/就绪探针、请求 Trace ID、SSE 实时状态、结构化日志。

## 快速启动

要求 Go 1.27.3+（含标准库安全修复）。仅在需要重新构建前端时需要 Node.js 24.15+（仓库固定为 24.21.0）。

### 桌面模式（教师机推荐）

```powershell
go build -tags "desktop wails" -ldflags="-H windowsgui" -o forge-desktop.exe ./cmd/forge
./forge-desktop.exe
```

桌面模式基于 Wails v3（与 magpie 同技术栈，教师端真原生窗口）：

- **主窗口**即教师台（WebView 直连本机 HTTP 服务，SSE/Cookie 与学生端同链路）；
- **托盘左键**弹出锚定在图标旁的无边框速览面板：比赛倒计时、在线/提交/评测队列、
  机位冲突告警、最近提交动态；右键菜单可直达教师台/学生大厅/扫码页；
- **关闭主窗口 = 隐藏到托盘**；退出走托盘菜单或速览面板的「退出服务」（优雅停机）；
- 单实例：重复启动自动唤起已运行实例。

学生端不受影响（浏览器访问 LAN 地址）。双击默认进桌面模式，`--server` 强制 headless；
`-tags desktop`（不带 wails）为无 WebView 依赖的托盘降级版（浏览器打开教师台），
Linux/Darwin 服务器部署继续使用 headless 方式。

### 服务模式（服务器 / 机房主机）

```powershell
cd frontend
npm.cmd ci
npm.cmd run build
cd ..

$env:FORGE_ADMIN_PASSWORD = "change-this-before-first-start"
go run ./cmd/forge
```

打开：

- 学生比赛大厅：`http://localhost:8080/`
- 教师控制台：`http://localhost:8080/teacher`
- 健康探针：`http://localhost:8080/healthz`
- 就绪探针：`http://localhost:8080/readyz`

首次启动若未设置 `FORGE_ADMIN_PASSWORD`，服务会生成一次性随机管理员密码并写入
`FORGE_DATA_DIR/bootstrap-admin-password.txt`（权限 0600），启动日志只输出文件路径。
该密码只应在首次登录时使用；修改密码后应立即删除该文件。

开发前端时可单独运行 Vite；生产环境应由 Go 服务托管 `internal/web/dist` 中的构建产物。
登录成功后浏览器只保存 `HttpOnly`、`SameSite=Strict` 的 `forge_session` Cookie；
JavaScript 不接触、不持久化长期会话凭证。服务端数据库只保存会话 token 的 SHA-256
摘要，泄露数据库也不能直接重放有效会话。

## 配置

| 环境变量 | 默认值 | 说明 |
|---|---|---|
| `FORGE_ADDR` | `127.0.0.1:8080` | HTTP 监听地址（局域网部署需显式改为可访问地址） |
| `FORGE_DATA_DIR` | `./data` | SQLite 与测试数据目录 |
| `FORGE_STORE` | `sqlite` | `sqlite` 或 `memory` |
| `FORGE_ADMIN_PASSWORD` | 随机生成 | 首次创建管理员时使用的密码 |
| `FORGE_LOCALE` | `zh-CN` | 服务端默认区域 |
| `FORGE_AUTOJUDGE` | `false` | 是否启用本地自动评测 |
| `FORGE_ALLOW_UNSAFE_LOCAL_JUDGE` | `false` | 明确确认本地评测器没有完整沙箱 |
| `FORGE_JUDGE_WORKERS` | CPU 数 | 评测并发数，最小为 1 |
| `FORGE_TOOLCHAINS` | 内置配置 | 工具链 JSON 路径 |
| `FORGE_TRUST_PROXY` | `false` | 仅可信反向代理后启用转发 IP 头 |

`FORGE_AUTOJUDGE=1` 必须同时设置 `FORGE_ALLOW_UNSAFE_LOCAL_JUDGE=1`。当前本地评测器
只提供进程/内存/时间限制，不提供文件系统、网络和独立用户级隔离。生产环境若处理
不可信代码，应接入独立的沙箱评测集群；否则保持自动评测关闭并采用人工评测流程。

工具链 JSON（`FORGE_TOOLCHAINS`）除 `command/args/std/compileTimeoutSec/
compileMemoryMiB` 外，还支持按语言的限额倍率（对标 LemonLime Compiler）：
`timeLimitRatio`（运行时限倍率，解释型语言建议 2~3）、`memoryLimitRatio`（内存限制
倍率）、`disableMemoryLimitCheck`（对自带内存管理的运行时关闭硬内存墙）。示例：

```json
{
  "python": { "command": "python", "timeLimitRatio": 3, "memoryLimitRatio": 2 }
}
```

题目级覆盖（对标 LemonLime 每题每编译器配置，可与工具链倍率叠加）：
`compileStd`（仅 C/C++，如 `"c++17"`）、`timeLimitRatio`、`memoryLimitRatio`、
`disableMemoryLimitCheck`。比较模式 `external_tool` 另需 `diffCommand`/
`diffArgs`（对标 LemonLime 外部 diff 工具，缺省参数
`--ignore-space-change --text --brief`）。

评测基石平台口径：比较模式 `ignore_space` 对标 LemonLime 的 token 比较语义——
token 相同但换行位置不同记 `PE`，多出尾部内容记 `OLE`；时间软限按 wall 或
CPU 任一超限判定，硬杀墙 = 时限 + max(2s, 2×时限)×25%，软硬墙之间的边界
抖动自动重测；内存墙 Windows 走 Job Object、Linux 走 prlimit（可选 bwrap
命名空间沙箱）、macOS 降级 sh ulimit，均可从 BSS 静态足迹起跑前判 MLE；
交互器与特判程序运行在独立资源墙内，交互器可在 exit 0 时向 stderr 写
testlib 打分标记（`ok` / `partially correct (N)` / `points X`）给出部分分；
编译产物按（源码, 生效工具链, 编译器版本, 平台）缓存并 LRU 淘汰，worker
缺省取物理核数（`FORGE_JUDGE_WORKERS` 显式配置优先），评测期间 Windows
教师机阻止系统休眠。

特判支持三种协议：`special_judge`（退出码 0=AC/1=WA/2=PE）、`lemon_special_judge`
（Lemon scoreFile 协议，支持 0..满分 部分分）、`testlib_special_judge`（testlib
stderr 协议，`ok` / `partially correct (N)` / `points X` 支持部分分）。

## 测试与验收

```powershell
gofmt -l internal cmd
go mod verify
go vet ./...
go test ./...
go build ./...

cd frontend
npm.cmd audit --audit-level=high
npm.cmd run typecheck
npm.cmd test -- --run
npx.cmd playwright install chromium
npm.cmd run test:e2e
npm.cmd run build

cd ..
npx.cmd --yes @redocly/cli@2.67.1 lint api/openapi.yaml --format=stylish
```

当前前端测试覆盖登录门禁、会话恢复、SSE 重连、倒计时、选择答题卡、教师题面与数据
工作流、答案题、比赛包导入、安全设置、Markdown/KaTeX 与可访问性组件。

## 运维与文档

- [系统规格](SPEC.md)
- [架构说明](docs/architecture.md)
- [部署手册](docs/deployment.md)
- [CCF 试卷排版](docs/pdf.md)
- [OpenAPI 契约](api/openapi.yaml)
- [开发守则](AGENTS.md)

API 统一位于 `/api/v1`。项目尚未承诺预发布兼容性，升级前请先备份 `FORGE_DATA_DIR`，
并在测试库完成比赛包导入、开赛预检和完整提交链路验证。
