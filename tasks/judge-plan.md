# 评测基石最终执行规划(定稿)+ 逐项评估

> **状态:已全部实施(2026-10-26),完成清单见 tasks/todo.md「评测基石第二批」。**
> 实施中的两处修正:静态内存 fixture 必须以运行期偏移访问全数组(否则 -O2
> 收缩消除 BSS);交互题发现父进程管道写端不关的真实缺陷并已修复。

基线:Forge `e676165` / upstream LemonLime `69aa223`。本文件是执行定稿;
背景研究见 `judge-roadmap.md`。评估维度:

- **可靠性**:改动引入回归/平台差异的风险,及缓解手段(降级路径、build tag、CI 双平台)。
- **测试性**:能否被自动化测试证明;在 CI(ubuntu+windows)上的可见度;不可测部分的补偿。
- **合理性**:对标 upstream 的依据;不做/缓做的理由(避免盲目复刻上游的死代码与坏语义)。

评级 高/中/低 + 一句话依据。

## 批 1 —— 正确性(P0)

### 1. 子任务依赖清零接通回写链路
改动:`judge.SubtaskMaps` 增返 `depsOf`(由 TestCases 聚合各子任务的 DependsOn);
`app.Service.JudgeCases` 增加 `depsOf` 参数并改用 `AggregateScoreWithDeps`;
两个调用点(`wire.go` 自动回写、`server.go:831` 人工回写)同步;全部受影响测试更新。
- 可靠性 **高**:聚合逻辑已在 domain 层实现且有单测,只是接通;无并发/平台面。
- 测试性 **高**:domain 纯函数已有覆盖;新增 wire 级端到端(依赖子任务未 AC → 依赖者 0 分落库)。
- 合理性 **高**:这是当前唯一"规则已定义但生产不生效"的正确性缺陷,教师端分数会算错,优先级最高。

### 2. checker/interactor 收进资源墙(含 scrubEnv)
改动:新增统一的 `runConfined`(scrubEnv + confineArgv 内存墙 + Job/进程组杀树),
`checker.go` 三协议与 `interact.go` 双进程全部走它;checker 内存墙 512MiB 常量。
- 可靠性 **高**:复用 runner 已验证的 confine/jobLimiter 基建,不新增机制;
  checker 文件均在临时目录,prlimit/bwrap 不涉(与编译器同策略)。
- 测试性 **中**:内存炸弹 checker 被墙拦下(双平台)、环境变量剥离(程序打印 getenv 断言为空)
  可自动化;Windows Job 杀树时序无法精确断言,靠 10s ctx 兜底覆盖。
- 合理性 **高**:当前特判路径连 `scrubEnv` 都没过,是评测面最裸露的洞;上游对 SPJ 也有独立时限态。

### 3. 文件 IO OLE 上限 + stdin 异步写
改动:runner 读输出文件改为限额读取(超 `OutputLimitKiB`/缺省 16MB → Truncated→OLE);
stdin 写入挪到 goroutine(io.Copy + Close),消除"输入大于管道缓冲且程序不读"时靠硬墙解锁的隐患。
- 可靠性 **高**:纯执行路径局部改动;goroutine 随进程退出解锁管道,无泄漏。
- 测试性 **高**:写超大输出文件的 fileio 集成测试(真实 g++)+ 大输入不读的程序计时断言。
- 合理性 **高**:`OutputLimitKiB` 声明了却不管文件 IO,属于配置语义未兑现;stdin 死锁是隐性计时污染。

### 4. Unix 进程组杀树(Setpgid + 负 pgid kill)
改动:新增 `sysproc_unix.go`(Setpgid)与 runner/interact 的组杀;Windows 已由 Job KILL_ON_CLOSE 覆盖。
- 可靠性 **中**:进程组语义三平台一致(POSIX),但与 prlimit/bwrap 的 exec 链交互需小心——
  prlimit 是 exec 型、bwrap 有 `--die-with-parent`,组杀是第三层兜底,只做加强不改变主路径。
- 测试性 **中**:linux 上"程序 fork 子进程后自身超时"fixture,子进程 PID 由程序打印,
  Run 返回后 `kill(pid,0)` 必须 ESRCH;windows 跳过(Job 已覆盖,不重复断言)。
- 合理性 **高**:交互题双进程、编译器孙进程都依赖完整杀树;上游 Windows 走 Job、Unix 走 watcher 收割,等价。

## 批 2 —— 多平台收口(P1)

### 5. confineArgv 裸跑分支收口
改动:linux 无 prlimit 与其他 Unix 的 default 分支统一降级 `/bin/sh ulimit` 墙(与 darwin 同路径);
`toolPaths` 加测试注入点。readiness 仍 fail-closed 不变。
- 可靠性 **高**:sh ulimit 是 best-effort 墙,失败不阻塞执行(`2>/dev/null`),不会把可用环境改坏。
- 测试性 **中**:argv 构造纯函数已有测试;分支选择通过注入空 prlimit 路径断言;真实降级路径仅非主流平台走到。
- 合理性 **高**:同是 Unix,linux 缺 prlimit 时从"无墙裸跑"变"有降级墙",纯增益。

### 6. MLE 信号启发式 + 全平台 MLE 断言(去陈旧 Skip)
改动:runner 增加判定——Unix 下进程死于信号且峰值≥95% 墙 → MLE;
静态内存预检路径补一个确定性 MLE 集成测试(大全局数组,双平台);
`judge_test.go`/`lemon_test.go` 三处 windows-only Skip 改为"windows 或有 prlimit 的 linux"。
- 可靠性 **中**:信号启发式可能把"恰好贴墙时自己 abort"的程序判成 MLE——这与上游
  watcher 的 SIGKILL/SIGABRT→MLE 口径一致,属有意对齐而非风险;静态预检测试完全确定性。
- 测试性 **高**:静态预检路径双平台可断言;CI ubuntu 从"永远跳过"变为有真实内存墙证据。
- 合理性 **高**:Linux 内存墙已落地但 CI 零断言,是测试完备性最明显的洞。

## 批 3 —— 功能对齐(P2)

### 7. ignore_space 比较补 PE 语义
改动:domain 增加 `CompareOutputDetail`(三元组:等值/仅格式差/不等),
ignore_space 下"token 序列相同但行内分布不同"→ PE;`judge.go` 与答案题路径接入。
- 可靠性 **中**:会改变部分存量判定(WA→PE),CSP corpus 全量回归验证;
  PE 与 WA 在聚合里都计 0 分,分数不变,只影响 verdict 展示。
- 测试性 **高**:domain 纯函数逐例测试 + corpus 不回归。
- 合理性 **高**:PE 是 upstream IgnoreSpaces 唯一产出途径(`casejudger.cpp:276-340`),
  Forge 的 CasePE 态目前只有特判 exit 2 能触发,语义缺半边。
- 注:line_by_line 已含"行尾空白忽略"口径,与上游一致,不动。

### 8. 交互题升级
改动:双进程接 limiter 内存墙/峰值上报、capped stderr、组杀;
interactor 编译失败/异常退出 → **CheckerError**(评测器侧故障,不再误报学生 RE);
exit 0 时解析 stderr 的 testlib 打分标记(`ok/partially correct (n)/points X`)支持部分分。
- 可靠性 **高**:全部复用既有基建;协议是 Forge 自有扩展(上游交互是编译期耦合,无可对标)。
- 测试性 **高**:ok/bad 既有测试 + 新增部分分 interactor fixture 与内存墙变体。
- 合理性 **高**:交互器故障归 RE 会误导学生申诉方向;部分分让交互题能配子任务计分。
- **不做**:新增 CaseInteractorError 枚举——上游 `InteractorError` 定义了从未产出(死代码),
  CheckerError 已能表达"评测器侧故障",少一个枚举少一整条前端/校验/聚合链路的涟漪。

### 9. CaseSystemError 产出路径
改动:pool.do 兜底、Compiler/Runner 返回 error 的路径、checker 目录创建失败等
基础设施故障 → SystemError 扇出,不再与"学生程序 RE"混淆。
- 可靠性 **高**:纯 verdict 映射变更,fake 三行可测。
- 测试性 **高**:fake 编译器返回 error → 全点 SystemError;域校验白名单已含该态。
- 合理性 **高**:SPEC 承诺教师可区分程序错误与评测器错误,当前 SystemError 是死枚举。

### 10. ExternalTool(diff)比较模式
改动:domain 新增 `external_tool` 模式 + `Problem.DiffCommand`(JSON 字段,缺省 `diff
--ignore-space-change --text --brief`,对标 upstream Settings diff 路径+参数);
judge 侧经 runConfined 执行,exit 0=AC 否则 WA;工具缺失 → CheckerError。
- 可靠性 **中**:Windows 无自带 diff,教师需自备(与上游一致的约束,README 注明);
  Problem 新字段走 JSON 序列化,旧包兼容(omitempty)。
- 测试性 **中**:exit-code 映射可 fake 单测;真实 diff 集成测试 ubuntu 有/windows 跳过。
- 合理性 **中**:为从 Lemon 迁移、自带 diff 工作流的教师保留等价能力;低频但成本低。

### 11. 题目级工具链覆盖
改动:domain 新增 `Problem.ToolchainOverrides`(每语言 Command/Args/Std/倍率/禁内存墙,
结构与 ToolSpec 对齐但不 import judge,避免环);`Orchestrator` 在编译前把覆盖合并进
`Toolchain` 值拷贝;validate 白名单 + 倍率范围校验。
- 可靠性 **高**:值拷贝合并,不影响全局工具链;fake 编译器路径零改动。
- 测试性 **中**:合并逻辑纯函数单测;真实覆盖(如 std=c++17)需 g++ 集成,requireTool 守护。
- 合理性 **高**:对标 upstream"每题每编译器可选配置或 disable"的核心场景(某题要求 C++17),
  是多题多语言赛场的刚需。

## 批 4 —— 性能(P3)

### 12. 编译缓存 v2
改动:键 = sha256(lang + src + 全量生效 ToolSpec JSON + 编译器版本指纹 + GOOS/GOARCH);
版本指纹 `command --version` 首行,sync.Once 每命令缓存;容量上限(256)+ LRU 淘汰;
singleflight 同键并发去重(手写 inflight map,不引依赖);hits/misses 计数进 RuntimeStats/metrics;
go build 产物进缓存。
- 可靠性 **高**:Get 仍复制到 workdir 防并发执行同文件;淘汰只删缓存文件不动评测。
- 测试性 **高**:键组成(注入指纹)、LRU 淘汰、并发去重(计数断言)、命中率指标均可单测。
- 合理性 **高**:当前键漏编译器版本(升级 g++ 后命中旧产物是正确性风险而不只是性能)、
  只增不减的临时目录、python/go 不受益,三个问题一次收口。

### 13. worker 缺省物理核
改动:config 缺省从 `runtime.NumCPU()` 改为物理核数——Windows 手写
GetLogicalProcessorInformationEx(x/sys 未导出,~60 行)、Linux sysfs 拓扑去重、
darwin sysctl;解析失败回退 NumCPU。
- 可靠性 **高**:仅影响缺省值(FORGE_JUDGE_WORKERS 显式配置优先);所有路径有回退。
- 测试性 **中**:跨平台实现各自 build tag;CI 断言 1 ≤ 物理核 ≤ NumCPU 边界;
  真实 SMT 折叠效果无法在 CI 断言。
- 合理性 **高**:SMT 超订阅会污染 CPU-time 计时(两个超线程竞争同一物理核),
  upstream 即按 CPU 拓扑取缺省(`cputopology.cpp`)。

### 14. 评测期间防休眠(Windows)
改动:pool 执行期 `SetThreadExecutionState(ES_CONTINUOUS|ES_SYSTEM_REQUIRED)`
(手写 kernel32 lazy DLL),Unix no-op 并注释说明。
- 可靠性 **高**:失败仅记日志不影响评测;作用域限单任务执行期,不长期占用。
- 测试性 **低**:电源状态在 CI 无法断言,仅冒烟(Acquire/Release 不报错)。
- 合理性 **中**:教师机过夜评测被系统休眠打断是真实痛点;upstream 三平台都做了,
  Linux DBus/IOKit 引依赖过重,先覆盖主力部署平台(Windows 教师机)。

## 批 5 —— 维护性(P4)

### 15. judge→app 依赖反转
改动:`wire.go` 整体迁至 `internal/app/autojudge.go`(AutoJudgePool/Enqueue/Reconciler),
层间变为 transport→app→judge→domain,judge 包不再 import app;main.go 接线更新;
wire_test.go 随迁。SubtaskMaps 留在 judge(domain 类型出入参,供 server.go 复用)。
- 可靠性 **高**:纯搬移 + import 调整,无逻辑变更;CI 全量测试即回归网。
- 测试性 **高**:既有 wire 测试随迁继续跑。
- 合理性 **高**:SPEC.md 分层约束明文要求;当前反向 import 是唯一的破口。

### 16. CI race + 文档收口
改动:go job race 步骤加 `./internal/judge/ -run 'TestPool|TestCache|TestSubtask|TestEnqueue|TestReconciler'`
(重 corpus 不进 race,控时长);README(新配置/新比较模式/平台差异)、
docs/architecture.md 评测章节、tasks/todo.md 勾选更新。
- 可靠性/测试性/合理性 **高**:纯 CI 与文档,无产品代码风险。

## 明确不做(与 upstream 比对后的取舍)

| 项 | upstream 状态 | 不做理由 |
|---|---|---|
| 通信题 Communication part/exec | 仅 Unix 保证,grader 需 -pthread 特殊编译 | CSP/GESP 无此题型;等真实需求再立项 |
| testlib `points X` 改 `X×fullScore` | 上游公式允许 score>fullScore 落库(潜在 bug) | Forge 钳制到满分更稳,[0,1] 区间与上游等价;注释记录差异 |
| cgroup v2 资源墙 | upstream 自己也是 TODO | prlimit+bwrap 已覆盖 AS/CPU/NPROC/网络/FS;cgroup 引系统依赖,收益边际 |
| Linux DBus/macOS IOKit 防休眠 | upstream 有 | 引 dbus/IOKit 绑定代价大;Windows 先行,其余平台 no-op 注明 |
| 新增 InteractorError 枚举 | 上游定义了但从未产出(死代码) | CheckerError 语义已覆盖,避免全链路枚举涟漪 |
| stop-on-error 开关 | 上游无此逻辑 | 子任务短路跳过已是更细粒度等价物 |

## 验收门禁(每批完成即跑,最终全量)

```powershell
gofmt -l internal cmd; go vet ./...; go build ./...
GOOS=linux go build ./...; GOOS=darwin go build ./...
go test ./...
go test -race ./internal/judge/ -run 'TestPool|TestCache|TestSubtask|TestEnqueue|TestReconciler'
```
