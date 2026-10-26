# 评测基石路线图 —— 基于 upstream Project_LemonLime 的对齐研究

- 研究基线:Forge `e676165`(评测基石对齐 LemonLime 第一批);
  upstream `Project-LemonLime/Project_LemonLime@69aa223`(已克隆至 `upstream/Project_LemonLime/`,2026-10-26 与 origin/master 同步)。
- 方法:逐文件比对 upstream 评测核心(`src/core/`、`src/base/`、`unix/`)与 Forge `internal/judge` + `internal/domain/judging.go`,核对上游源码验证关键语义(非仅凭报告)。
- 结论先行:第一批对齐(部分分特判、限额倍率、硬墙重测、静态内存、多平台墙)方向正确、覆盖了上游"最小对齐集"的大部分。**剩余缺口里最实质的是 3 个正确性问题(P0),其次才是功能面与测试面。**

## 一、上游能力清单(对齐基准)

| 能力域 | upstream(LemonLime) | 证据 |
|---|---|---|
| 结果态 | 15 态 ResultState + 6 态 CompileState | `src/base/LemonType.hpp:11-37` |
| 比较模式 | LineByLine / IgnoreSpaces(token,唯一 PE 来源)/ ExternalTool(diff)/ RealNumber(long double,绝对+相对误差)/ LemonSPJ / TestlibSPJ | `casejudger.cpp:218-689` |
| 计分 | 点内 min(各 case)、子任务依赖 1e6 比例折算、被依赖失败→钳到折算分 | `taskjudger.cpp:398-543`、`subtaskdependencelib.h` |
| SPJ | 不代编 checker;Lemon 协议 `_score` 任意 0..满分;testlib:`ok`/`FAIL`/`partially correct (n)`(n×满分/100)/`points X`(X×满分,X 可小数) | `casejudger.cpp:460-689` |
| 交互 | **编译期耦合**:interactor 与选手源码一起编进同一可执行文件(`__grader.cpp`),无独立 interactor 进程;`InteractorError` 有定义无产出 | `taskjudger.cpp:116-123` |
| 题型 | 传统 / 答案题 / Interaction / Communication(part / exec 两种,仅 Unix) | `task.h:22` |
| Windows | CreateProcessW 轮询(10ms);内存=轮询 GetProcessMemoryInfo;计时=GetProcessTimes **user time**;2026 新增可选 **AppContainer 沙箱**(ACL+Job Object+UI 限制+运行时发现授权缓存) | `processrunner_win.cpp`、`windowssandbox.cpp` |
| Linux | bwrap `--unshare-all` 强制 + watcher(setrlimit AS/STACK/CPU,wait4 rusage,**ELF 静态内存预检**);cgroup 仍是 TODO | `processrunner_unix.cpp:32-113`、`unix/watcher_*.cpp` |
| macOS | watcher + Mach-O 静态内存;Apple Silicon 内存 ×4/÷4 折算特例 | `watcher_macos.mm:146-148` |
| 时间口径 | 三平台统一报 **user CPU time**;额外时间比例默认 0.1;边界 TLE 重测(默认 1 次,上限 12) | `settings.cpp:558-565`、`casejudger.cpp:787-795` |
| 编译 | 3 种编译器类型(典型/字节码解释/纯解释);多命名配置+每题每编译器可选/可 disable;高级项:timeLimitRatio/memoryLimitRatio/disableMemoryLimitCheck/interpreterAsWatcher | `compiler.h:18-82`、`advancedcompilersettingsdialog.cpp:67-111` |
| 调度 | QThreadPool,默认线程数按 **CPU 拓扑(物理核)**;QFutureWatcher 取消传播到进程终止 | `judgingcontroller.cpp`、`cputopology.cpp:214-229` |
| 其他 | 测试点调整器(ext,CCR 移植,标记"测试阶段")、防休眠三平台、评测完 notify-send(Linux)、XLS 导出(可选) | `src/exttestcase*`、`sleepinhibitor.cpp` |

上游明确没有的(stop-on-error、validator/statistician、插件式评测 API、cgroup)不必追。

## 二、Forge 现状(已达成,保持)

- 三特判协议 + 逐点部分分 + 子任务短路跳过 + 依赖 DAG(domain 层);
- 限额倍率 / 禁内存墙 / 解释型 2x、软限(wall∪CPU)与硬杀墙(软限+max(2s,2×时限)×0.25)、边界重测、稳定 TLE 不重测;
- 静态内存预检(PE/ELF/Mach-O,比上游还多覆盖 Windows);
- 资源墙层级:Win Job Object(内存+ActiveProcessLimit)/ Linux bwrap+prlimit / darwin ulimit 降级,峰值 rusage 回填、MLE 贴墙(≥95%);
- 编译缓存(Get 复制到 workdir 防并发)、worker 池、重启恢复 + reconciler、就绪探针、fail-closed 配置解析;
- 比较模式 line_by_line / ignore_space / real_number + 三特判,缺省 ignore_space 与上游一致;
- 测试 13 文件 ≈ 1:1 代码量,CSP corpus 40 点 × 变体矩阵,CI ubuntu+windows 双平台。

## 三、缺口与路线图

### P0 —— 正确性(先于一切功能)

1. **依赖清零语义未接回写链路**。`AutoJudgePool` 回调只带 `so, fo`,`svc.JudgeCases` 用无依赖版 `AggregateScore`(`internal/app/usecases.go:420`);`AggregateScoreWithDeps` 的"被依赖子任务未全 AC 则清零"只在 domain 单测里生效。教师看到的成绩会与规则不符——这是当前最实质的对齐缺口。
   改:wire 回传 DependsOn(已在 BuildRequest 里有,补 report 侧),`JudgeCases` 切换到带 deps 版并端到端测试。
2. **checker / interactor 裸 exec**。三种特判与交互器无 Job/prlimit/ulimit 包裹,`checkExit` 路径连 `scrubEnv` 都没过(`internal/judge/checker.go:243`),仅 10s ctx。上游对 SPJ 至少区分 `SpecialJudgeTimeLimitExceeded` 态。改:复用 runner 的 limiter/scrubEnv 基建包一层;超时落 CheckerError 并附注(或新增专态)。
3. **文件 IO 模式无 OLE**:`os.ReadFile` 读选手输出无上限(`internal/judge/runner.go:122`),`OutputLimitKiB` 只约束 stdout 管道。改:读文件加 cap,超限落 OLE。同批修 **stdin 同步写**死锁隐患(`runner.go:108-112` 输入超管道缓冲且程序不读时靠硬墙解锁,污染软限计时)——goroutine 化 + ctx 关闭。

### P1 —— 多平台

4. **Unix 杀树**:`Setpgid` 未用、无 KILL_ON_CLOSE 等价物,裸 prlimit 路径的孙进程可残留 ctx 之外(bwrap 路径有 `--die-with-parent` 缓解);交互题双进程天然产生孙进程。改:SysProcAttr{Setpgid} + 负 pgid kill(`job_other.go`/`runner.go`)。
5. **裸跑兜底收口**:Linux 无 prlimit 时 `confineArgv` default 分支裸跑(靠 readiness 拦截,但允许跳过);其他 Unix(`host_other.go`)无任何墙。改:default 分支挂 sh ulimit 降级墙,与 darwin 同路径;readiness 保持 fail-closed。
6. **macOS 记录 Apple Silicon 折算特例**(上游 ×4/÷4):`ulimit -v` 在 darwin 上 best-effort,文档写明不承诺精确 MLE。
7. **CI 平台证据**:MLE/内存上报/forkbomb 测试仍 windows-only Skip,C 的 prlimit 已在 CI 落地但只有 doctor 无断言。改:ubuntu 上加 prlimit MLE 断言用例 + fkbomb 杀树断言,去掉过时的 Skip 注释。

### P2 —— 功能对齐(增量)

8. **verdict/compile 态映射表补全**:对照上游 15+6 态,Forge 缺:PresentationError(ignore_space 下 token 分布不同→PE,上游唯一 PE 来源)、CannotStartProgram、FileError、InvalidSpecialJudge(现并入 CheckerError)、SpecialJudge TLE/RE 区分、InteractorError、NoValidSourceFile/NoValidGraderFile 编译态。建议出一张 domain 映射表 + 逐态补测试,不必逐字复刻,但 PE 与 InvalidSpecialJudge 值得先补。
9. **交互题升级**:接入 limiter 内存墙、峰值内存上报、interactor stderr 截获(capped)、部分分协议(复用 `parseTestlibResult` 让 interactor 走 stderr 打分)、`InteractorError` 态。Forge 的独立 interactor 协议本就比上游编译期耦合更现代,保留并加强。
10. **比较模式补齐**:ExternalToolMode(可配 diff 工具+参数,上游默认 `--ignore-space-change --text --brief`)——教师自备 checker 的轻量替代;RealNumber 补上游"绝对+相对双判"口径核对。
11. **题目级工具链覆盖/禁用**(上游每题每编译器选配置或 `disable`):`ToolSpec` JSON 已 fail-closed,加题目级 override 字段即可,拓展性好抓手。
12. **testlib `points X` 语义**:已核对上游 `casejudger.cpp:661-666` 公式即 `X×fullScore`(X 可小数);Forge"比例×满分+钳制"在 [0,1] 区间等价,X>1 时钳满分更稳(上游会记超满分分,是上游潜在 bug)。保持现状,在 `align_test.go` 注释里记录该有意差异。

### P3 —— 性能

13. **编译缓存 v2**(`internal/judge/cache.go`):键补 编译器版本指纹(`g++ -dumpversion`)+GOOS/GOARCH+工具链 args 全量;加容量上限+LRU 淘汰(现在 `os.TempDir()/forge-ccache` 只增不减);go build 进缓存;同源码并发提交 singleflight 去重;hit/miss 打进 observability 指标。
14. **worker 默认按物理核**(上游 `cputopology.cpp` 语义):SMT 超订阅会污染 CPU-time 计时,`FORGE_JUDGE_WORKERS` 缺省建议取物理核数。
15. **评测期间防休眠**(上游三平台 sleep inhibitor):Wails 桌面模式托盘期间挂 PowerCreateRequest,教师机过夜评测适用,nice-to-have。

### P4 —— 维护性 & 拓展性

16. **反转 judge→app 反向依赖**:`internal/judge/wire.go` import `internal/app`。把回写收口成 ports 接口(评测器依赖接口,main 注入),分层回到 SPEC.md 的约束。
17. **race 门禁**:`-race` 快包列表(CI go job)加入 `internal/judge` 的池/缓存/wire 子集(重 corpus 排除),pool.go 并发面目前零 race 覆盖。
18. **CaseSystemError 产出**:池错误兜底现为整单 RE;基础设施故障应落 SystemError 态,便于教师区分"程序错了"与"评测器错了"并一键重判。
19. **拓展点定型**:comparator 以 domain 接口注册(新模式=实现+校验白名单各一行);题目级工具链覆盖(见 11);通信题(part/exec)明确**不做**(上游仅 Unix 且 CSP/GESP 无需求)。

## 四、建议执行批次

| 批次 | 内容 | 验收 |
|---|---|---|
| 批 1(正确性) | P0-1/2/3 + P1-4 | `go test ./...`;新增依赖清零端到端、checker 加墙、文件 IO OLE 测试;ubuntu CI prlimit 断言 |
| 批 2(平台+对齐) | P1-5/6/7 + P2-8/9/10 | 双平台 CI 绿;verdict 映射表测试全绿;交互题带内存墙的 corpus 变体 |
| 批 3(性能+维护) | P2-11/12、P3-13/14、P4-16/17/18 | 缓存命中率指标可见;`-race ./internal/judge/...` 快子集;`go vet`/gofmt 门禁 |

每批遵守仓库门禁:gofmt / vet / build / test / cross-compile(linux+darwin guard)/ 前端五道门不变。
