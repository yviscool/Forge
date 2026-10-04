# Forge 竞赛工坊

<p align="center">
  <strong>现代化、跨平台的信息学竞赛与教学评测工作台</strong><br>
  Modern, Cross-Platform Competitive Programming Contest Hosting & Evaluation Platform
</p>

<p align="center">
  <a href="SPEC.md">系统规格 (SPEC)</a> •
  <a href="docs/architecture.md">系统架构</a> •
  <a href="docs/pdf.md">CCF 试卷排版规范</a> •
  <a href="AGENTS.md">开发守则</a>
</p>

---

## 🌟 核心特性 (Key Features)

- **多比赛并发调度 (Multi-Contest Concurrency)**：同时独立配置、启动、管理多场比赛，比赛生命周期（草稿/进行中/已结束）、提交流与实时排行榜相互隔离。
- **全局独立用户与灵活编组 (Independent Users & Grouping)**：
  - 用户体系与比赛解耦，一次录入长期有效；
  - 支持创建全局分组（如“提高组集训A班”、“高一初赛组”），学生可加入多个组；
  - 比赛支持按「整组批量授权」或「单独添加个人」，支持非授权拦截。
- **结构化题面校验与多语言版本 (Problem Validation & Multilingual)**：
  - 强制检验试题必要字段（题目描述、输入格式、输出格式、数据范围与样例成对匹配）；
  - 支持试题多语言版本（`Problem.Locales`）。
- **零依赖 CCF CSP 官方规范 A4 PDF 导出 (Zero-Dependency CCF PDF Engine)**：
  - 自动探测系统原生 Edge / Chrome / Chromium 浏览器（Windows 10/11 开箱即用）；
  - 一键无头导出标准 CCF A4 试卷（含封面表格、编译选项 `-O2 -std=c++14 -static`、考生守则、跑头、带行号与蓝边框的样例框、数据范围表格）；
  - **教师机器完全不需要安装 Node.js，也不需要安装 Python！**
- **全栈五维 i18n (Full-Stack Internationalization)**：
  - 前端 UI 双语字典 (`zh-CN` / `en-US`)；
  - 后端 API 响应基于 `Accept-Language` 自动本地化；
  - 评测专业术语 Verdict 中英映射；
  - 试题内容双语版与 PDF 模版多语言。
- **单二进制便携分发 (Zero-Dependency Distribution)**：
  - 前端网页完全内嵌于 Go 单可执行文件中（`//go:embed`），终端用户无需配置任何前端环境。
- **实时事件推送 (Real-Time SSE Streaming)**：
  - 基于 Server-Sent Events，提交流、判题结果与排行榜变动毫秒级向全网广播。

---

## 🚀 快速启动 (Quick Start)

### 方式一：直接运行服务

```powershell
# 启动 Forge 核心服务
go run ./cmd/forge
```

服务就绪后，访问对应端点：
- **学生比赛大厅**：`http://localhost:8080/`
- **教师管理控制台**：`http://localhost:8080/teacher`
- **实时事件流**：`http://localhost:8080/api/events`

*(说明：`go run ./cmd/arena` 作为历史入口同样保持兼容)*

---

## 📡 核心 API 端点概览 (REST API, `/api/v1`)

| 模块 | 方法 | 端点 | 描述 |
|---|---|---|---|
| **认证** | `POST` | `/api/v1/auth/login` | 登录签发 Bearer token |
| **认证** | `POST` / `GET` | `/api/v1/auth/logout` `/api/v1/auth/me` | 注销 / 当前用户 |
| **认证** | `POST` | `/api/v1/auth/password` | 本人改密 |
| **比赛** | `GET` / `POST` | `/api/v1/contests` | 列表 / 创建（`rankingMode: oi/acm`） |
| **比赛** | `GET` | `/api/v1/contests/{cid}` | 详情 |
| **比赛** | `POST` | `/api/v1/contests/{cid}/start` `/finish` | 启动 / 结束 |
| **比赛** | `GET` | `/api/v1/contests/{cid}/statistics` | 每题通过率/首 AC 统计 |
| **试题** | `GET` / `POST` | `/api/v1/contests/{cid}/problems` | 列表 / 新增（含特判/交互/文件 IO/测试点） |
| **试题** | `POST` | `/api/v1/contests/{cid}/problems/{pid}/validate` | 题面结构校验 |
| **试题** | `POST` | `/api/v1/contests/{cid}/problems/{pid}/rejudge` | 全量重判 |
| **试题** | `PUT` | `/api/v1/contests/{cid}/problems/{pid}/subtasks` | 子任务分数与限额 |
| **数据** | `PUT/GET/DELETE` | `/api/v1/contests/{cid}/problems/{pid}/files[/{name}]` | 测试数据文件仓 |
| **试题** | `GET` | `/api/v1/contests/{cid}/problems/{pid}/export` | CCF 打印视图 |
| **试题** | `GET` | `/api/v1/contests/{cid}/problems/{pid}/pdf` | CCF A4 PDF |
| **用户** | `GET` / `POST` | `/api/v1/users` | 查询 / 创建（教师） |
| **用户** | `POST` / `GET` | `/api/v1/users/import` `/export` | CSV 批量导入 / 导出 |
| **编组** | `GET` / `POST` | `/api/v1/groups` | 查询 / 创建 |
| **编组** | `POST` / `DELETE`| `/api/v1/groups/{gid}/members` | 添加 / 移除成员 |
| **授权** | `POST` | `/api/v1/contests/{cid}/groups` `/participants` | 绑定组 / 个人 |
| **提交** | `POST` / `GET` | `/api/v1/contests/{cid}/submissions` | 提交（身份即归属）/ 提交流（脱敏） |
| **判题** | `POST` | `/api/v1/submissions/{id}/judge` `/cases` | 总分回写 / 按点回写 |
| **榜单** | `GET` | `/api/v1/contests/{cid}/ranking` | OI 或 ACM 实时榜 |
| **事件** | `GET` | `/api/v1/events?contestId=` | SSE 实时流 |
| **语言** | `GET` | `/api/v1/i18n` | 双语字典 |

## 🔌 环境变量

| 变量 | 缺省 | 说明 |
|---|---|---|
| `FORGE_ADDR` | `:8080` | 监听地址 |
| `FORGE_DATA_DIR` | `./data` | sqlite 文件 + 测试数据仓 |
| `FORGE_STORE` | `sqlite` | `sqlite` / `memory` |
| `FORGE_ADMIN_PASSWORD` | `admin123` | 种子管理员（首启即改） |
| `FORGE_AUTOJUDGE` | `1` | `0` 关闭本地自动评测 |
| `FORGE_TOOLCHAINS` | 空 | 工具链 JSON 配置路径 |

> 沙箱说明：Windows 用 Job Object（内存墙+单进程+关闭即杀树），Linux 用 prlimit 包裹（地址空间/CPU/进程数）；网络隔离请用教室网关控制，判题机勿直连外网。

---

## 🛠️ 测试与构建 (Test & Build)

```powershell
# 运行全部单元与集成测试（含 PDF 无头引擎测试）
go test -v ./...

# 编译全部包为单一独立二进制
go build ./...
```

---

## 📚 详细文档导航 (Documentation)

- [SPEC.md](SPEC.md) - 单源真理技术规格书 (V2 定稿版)
- [docs/architecture.md](docs/architecture.md) - 系统分层架构与数据流蓝图
- [docs/pdf.md](docs/pdf.md) - CCF CSP 试卷标准与排版规范
- [AGENTS.md](AGENTS.md) - 智能助手开发守则与安全边界
