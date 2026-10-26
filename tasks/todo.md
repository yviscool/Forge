# Forge 任务清单

Phase 1（品牌与领域重构）、Phase 2（PDF 引擎 + SQLite 适配器 + 现代前端脚手架）
已全部完成并合入 master；本文件现为上线前任务清单，进度详见 git log 与 docs/。

# 商业上线硬化（2026-10 完成）

- [x] React 19 + Vite 前端重写（教师台 / 学生端 / 客观题试卷 / 安全面板），vitest 91 用例 + Playwright E2E
- [x] 鉴权硬化：bcrypt、角色矩阵（教师仅能创建/重置学生账号）、登录限流自清理、过期会话回收
- [x] 判题队列持久化恢复：启动扫描 `queued` 提交 + 周期 reconciler 补投；队列满拒绝并回 429
- [x] 就绪探针 `/readyz`：store 写探针、磁盘水位、判题 worker / 工具链 / 资源墙检查，失败 503 带原因
- [x] 判题风险门禁：`FORGE_AUTOJUDGE` 必须同时显式 `FORGE_ALLOW_UNSAFE_LOCAL_JUDGE=1`；
      Linux 缺 prlimit 时 readiness 报警，production 环境直接拒绝启用本地判题
- [x] 安全默认值：默认绑定 `127.0.0.1`、管理员密码随机生成（不落默认口令）、
      production 强制 sqlite、数据目录/文件权限收紧（0700/0600 + UMask=0077）
- [x] 代理可信边界：`X-Forwarded-For` 仅在 `FORGE_TRUST_PROXY=1` 时取最右合法地址，默认用连接对端
- [x] 可观测性：JSON 结构化日志、`/metrics` 与 HTTP/SSE/判题队列指标（docs/observability.md）
- [x] 备份恢复闭环：`scripts/backup.sh`（VACUUM INTO + 校验和 + 保留策略）与 `backup_test.sh` 进 CI
- [x] CI 五道门：go(ubuntu+windows) / frontend(含覆盖率与 audit) / e2e / contract(OpenAPI) / security(govulncheck)，action 全部固定 commit SHA
- [x] Release 流水线：verify → 4 平台归档 + go version SBOM + SHA256SUMS → 归档解压启动冒烟（cmd/forge-smoke）→ 发布
- [x] 判题 fixture 稳定性：并行偷钟探针改为固定迭代量烧 CPU，调度受限宿主不再误判 AC（wall/CPU 双路径均 TLE）

# 省份表与 CSP 准考证号（2026-10 完成）

- [x] 内置 34 省级行政区引用表（编号/名称/固定排序，编译进二进制），启用状态持久化（sqlite v7 + memory 契约测试）
- [x] 用户关联省份：创建账号（编号或中文全名归一化）、编辑账号（PUT /users/{id}）、批量改省（事务原子）、CSV 导入第 6 列
- [x] CSP 准考证号发号：省份-组别(J/S)+定长序号（默认 5 位，3-8 可配），按（省份，组别）续发永不重号，exam_ticket 唯一索引兜底
- [x] 前端：「省份与考号」管理表（用户数/状态/启停，仅管理员可操作）、名册省份与准考证列、批量改省份、准考证对话框 CSP 模式（预览号段/选中发号）
- [x] CSV 导出对齐导入列序（name,role,group,password,username,province,examTicket），可回导
- [x] 无障碍：管理表全部文本对比度 ≥ 4.5:1，E2E axe 扫描覆盖新面板

# LemonNext 对齐升级 + native 端（2026-10 完成）

- [x] 功能对齐：LemonNext 的在线提交服务 / GESP 机考 / NOI 全真模拟三种开展方式全部落地
      （比赛 mode 字段 + NOI 赛中反馈屏蔽 + 最后一次提交计分）
- [x] 升级：实时监考大屏（SSE 在线状态、机位冲突告警、每题作答动态、编辑动态投屏）
- [x] 升级：代码版本化（保存/提交即版本、版本对比、教师演进历程走真实版本仓）
- [x] native 端：magpie 式桌面壳（LAN 地址发现、扫码加入卡片、浏览器自启、
      `-tags desktop` 系统托盘，headless 服务器部署不受影响）
- [x] 测试完备：Go 全量 + storetest 契约 + HTTP 集成 + vitest 101 用例 + Playwright E2E 9/9
      （E2E 抓出并修复：监考快照 JSON null 击穿前端、Monaco diff 编辑器销毁时序、深色大屏对比度）

# 赛季准考证与 CSP 收题包（2026-10 完成）

- [x] 准考证独立成表 `Ticket(userID, season, group, province, number)`，唯一键
      (userID, season, group)：号是「身份」而非「场次」，同一赛季所有比赛共用
      一套号，跨赛季重新起号；users.examTicket 降级为冗余展示字段，
      sqlite v9/v10 迁移把历史号按「号内解析组别 + 当前年份」搬进新表（幂等）
- [x] `Contest.Season + TicketGroup`：建赛即声明口径（空 = 普通在线赛），
      settings PUT 可改；比赛永远不拥有号，收题时按（赛季, 组别）反查
- [x] `Problem.Slug` 题面英文名：小写字母开头、[a-z0-9_]，收题包文件名的唯一权威
- [x] CSP 收题包导出 `GET /contests/{cid}/collect`：zip 按
      「准考证号/题面英文名/英文名.扩展名」组织（无号退登录名，缺 slug 退题号小写），
      每人每题收最后一次提交（口径与 NOI 榜单一致），附 manifest.csv 与
      `?format=json` 清单（缺号/缺英文名/白卷告警）
- [x] 考点发号体验：发号请求带赛季（缺省当前年份）、账号列表带教师可见明文口令
      （鉴权仍只认 bcrypt hash）、DELETE /users 级联清理名单/凭据/会话/准考证
- [x] 前端：建赛与设置面板的「赛季 + 组别」、题目编辑器 slug 字段、
      实时提交页「导出收题包」（先清单后 zip）、准考证对话框赛季输入
- [x] 测试：storetest Ticket/明文口令/删号级联契约（memory + sqlite 双实现）、
      domain 布局纯函数、app 发号赛季隔离与收题用例、HTTP 集成（zip 结构 + 越权）、
      vitest 收题流与收题口径保存；OpenAPI 对齐（redocly lint 通过）

# 评测基石第二批(2026-10 完成,对齐 upstream Project_LemonLime@69aa223)

规划与逐项评估见 tasks/judge-plan.md;全部 16 项已完成,验收全绿
(gofmt/vet/build/test/cross-compile linux+darwin/race 子集)。

- [x] P0-1 子任务依赖清零接通回写链路:`SubtaskMaps` 三返回(含 depsOf),
      `JudgeCases` 换 `AggregateScoreWithDeps`,自动评测与人工回写两条链路同语义
- [x] P0-2 特判/交互器/diff 收进资源墙:`runConfined` 统一最小环境 + 内存墙 +
      组杀,checker 三协议超时显式落 CheckerError;发现并修复交互题父进程
      管道写端不关导致 EOF 无法传播、交互器崩溃被误判 TLE 的缺陷
- [x] P0-3 文件 IO 输出文件限额回读(OLE 配置兑现)+ stdin 异步写(防软限计时污染)
- [x] P1-4 Unix 进程组杀树:Setpgid + watchGroupKill,孙进程不再逃逸
      (Linux 集成测试用 fork fixture 断言子进程死亡)
- [x] P1-5 confineArgv 裸跑分支收口:无 prlimit 的 Unix 统一降级 sh ulimit 墙
- [x] P1-6 MLE 信号启发式(SIGABRT/SEGV + bad_alloc/贴墙 → MLE,对标 upstream
      watcher 信号映射)+ 全平台静态内存 MLE 断言,三处陈旧 windows-only Skip 清除
- [x] P2-7 ignore_space 补 PE/OLE 语义(对标 compareIgnoreSpaces 行号分叉判定)
- [x] P2-8 交互题升级:双进程共享 Job 内存墙、峰值上报、capped stderr、
      部分分协议(stderr testlib 标记)、交互器违约落 CheckerError 不误报学生 RE
- [x] P2-9 SystemError 兑现:评测器基础设施故障扇出 SystemError,与学生 RE 分离
- [x] P2-10 external_tool 比较模式(diff 命令,exit 0=AC/1=WA/其他=CheckerError)
- [x] P2-11 题目级工具链覆盖(compileStd/倍率叠加/禁内存墙)+ 校验白名单
      顺带修复 lemon/testlib 比较模式被 ValidateProblem 拒收的存量缺陷
- [x] P3-12 编译缓存 v2:键含编译器版本指纹/GOOS/GOARCH/全量 ToolSpec,
      LRU(256 条/1GiB)+ singleflight 同键去重 + hit/miss 指标(/metrics),
      go build 产物进缓存
- [x] P3-13 worker 缺省物理核(Windows GetLogicalProcessorInformation/
      Linux /proc/cpuinfo/darwin sysctl,失败回退 NumCPU)
- [x] P3-14 Windows 防休眠:评测期间 SetThreadExecutionState(专用锁定线程),
      其余平台 no-op 注明
- [x] P4-15 依赖方向归正:AutoJudgePool/EnqueueQueuedSubmissions/Reconciler
      迁入 internal/app,judge 包不再反向 import 应用层(符合 SPEC 分层约束)
- [x] P4-16 CI race 门禁加 internal/judge 并发子集;README/architecture 同步

# CI 清零(2026-10-27,四连败→全绿)

- [x] go-ubuntu vet:linux 专用测试文件(killtree)缺 []byte 转换——Windows 本地
      vet 不编译 build-tagged 文件而漏网;CI 新增 cross-vet 步骤
      (GOOS=linux/darwin go vet)把平台测试文件纳入每个 runner 的类型检查
- [x] frontend lint 六个既有错误:未用导入(detectSnapZone/TOP_BAR_H/
      LemonUploadFileInput)、未用解构(title)、未用变量(toast)、
      useMemo 多余依赖(messagesOpen → 改 seenMessageId state 真响应式,
      未读徽标行为不变)
- [x] e2e axe 无障碍(3 浏览器 × 教师台):横向滚动数据表补 tabIndex=0
      (TeacherConsole 四处 + Scoreboard);学生答卷卡两处对比度
      (white/45→60、white/40→60,4.2/3.62 → ≈6.2:1)
- [x] 本地全量验证:eslint/tsc/vitest/build + Playwright 9/9
      (chromium+firefox+webkit)+ go vet/test + GOOS=linux/darwin vet

# 覆盖率门禁重建(2026-10-27)

- [x] filesubmit 特性(0 覆盖 ~2600 行)补 6 个测试文件 53 用例:
      keys/time 纯函数、Lemon mockApi(含 lemonState=pre/ended 动态导入)、
      河南版 mockApi(校验顺序/封禁/冻结/publish)、TaskSubmitPanel
      (草稿/提交/覆盖确认/Ctrl+Enter/文件上传/答案题)、ChoiceAnswerPanel
      (切题/多选/草稿/回填/部分作答确认/窗口外锁定)
- [x] monacoMock 补 updateOptions/layout(挂载同步缺方法会让组件树卸载)
- [x] 合理排除:类型-only 模块、main/App 挂载壳、UbuntuDesktop/DesktopPreview
      桌面专用外壳(由桌面构建与 E2E 验证,jsdom 无法承载)——理由落配置注释
- [x] 覆盖率 43.2/41.7/40.3/45.4 → 60.4/55.5/54.2/62.3,四项阈值(55/51/50/58)全越线

# BwrapArgv 真缺陷修复(2026-10-27,ubuntu CI 首次跑通 test 步骤暴露)

- [x] BwrapArgv 只拼旗标、从未把 bwrap 二进制放进 argv 头部——exec 去找
      "--dev" 这个"可执行文件"(`exec: "--dev": executable file not found`),
      所有 Linux 真实工具链测试全灭。batch-1 起就是死代码,ubuntu 的 test
      步骤此前永远被 doctor(137)/vet 挡住,今天是首次执行。修复签名
      (bwrap 路径入参入 argv[0])+ align_test 断言二进制必须居首
- [x] bwrap 可用性探测:Ubuntu 23.10+ 默认限制非特权 userns(AppArmor),
      探测失败退回 prlimit-only,沙箱封装本身不得变成选手程序的 RE
- [x] MLE 信号启发式补 bwrap 包装器口径:致命信号经包装器以 128+信号码
      退出码上报,一并识别(SIGABRT/BUS/KILL/SEGV)
- [x] 教训固化为 CI 门禁:cross-vet(GOOS=linux/darwin go vet)已入 go job;
      judge doctor 纯诊断步骤容错化,不再挡住后续步骤

# NPROC 配额修正(2026-10-27,ubuntu 第三轮)

- [x] prlimit --nproc 64→256:RLIMIT_NPROC 按同真实 UID 全局计,CI/多租户
      宿主上 runner 账号常驻进程已达几十个,64 的配额让 cc1plus fork、
      std::thread、交互 fork 随机 EAGAIN(Cc1plus CE / parallel RE /
      killtree fork=-1 / checker 间歇 RE,doctor 单跑能过、测试并发即撞墙)。
      256 仍防 fork 炸弹(forkbomb fixture 连发 500 必然撞墙),Windows
      ActiveProcessLimit=1 单进程墙不变

# CI 全绿(2026-10-27,run 40381440198 @ a63d20a)

六 job 全过:frontend(覆盖率门禁)/ go-ubuntu(Linux 评测线首次真实验证)/
go-windows / e2e(3 浏览器 axe)/ contract / security。

本轮连环修复链(每轮 CI 日志驱动):
1. killtree_linux_test 缺 []byte → CI 加 cross-vet(GOOS=linux/darwin vet)
2. BwrapArgv 缺二进制头部(死代码首跑)→ 修签名 + 探测降级
3. bwrap CPU 计量不穿透(ubuntu-26.04 实测 CpuMs 恒 1ms)→ 探测验证计量,
   不穿透自动退 prlimit-only
4. 探测同步执行吃光首单运行 ctx("case 0 RE deadline exceeded / case 1 AC")
   → 探测异步化(上轮补丁静默失配未入库,带断言重写后落地)
5. prlimit --nproc 64:CI runner 同 UID 进程数逼近配额,cc1plus/thread/fork
   随机 EAGAIN → 提到 256(防炸弹语义不变)
6. forkbomb fixture:三平台确定性拦截(500 连发 + 子进程先关 stdio + 
   spawn 失败收割),Linux 分支补 vector/cstdio/signal 头
7. 判题可观测性:runner/interactor 基础设施故障的 RE 携带错误原文

# 阶段三 P0:Windows LowBox 沙箱(2026-10-27 完成)

- [x] LowBox(AppContainer 类)令牌:NtCreateLowBoxToken(Chromium 同路)从
      当前令牌派生,经 SysProcAttr.Token 交给 os/exec——管道/等待/rusage/Job
      全保留;包 SID 固定种子派生(S-1-15-2-<7 字 LE>,8 子授权;9 子授权被
      内核拒收 0xC000A202 实测)
- [x] 工作目录 ACL:包 SID GRANT + 继承传播覆盖既有子树;x/sys 类型化
      EXPLICIT_ACCESS + SetEntriesInAclW/SetNamedSecurityInfo
      (TRUSTEE_IS_SID=0 填错会 87;EA 结构体用官方类型防布局漂移)
- [x] 运行时目录只读 ACL:编译器 bin / Python prefix(前缀经 -I -S 发现),
      同步首调 + 按 command 去重(异步授权赶不上首次运行,解释型首跑即 RE)
- [x] 探测:容器内自证 + 越界读必须失败;失败自动退回 Job-only 并记录原因;
      FORGE_WIN_SANDBOX=auto/off,readiness 与启动日志可见
- [x] 环境收口:TMP/TEMP 覆盖到工作目录(容器读不到用户 %TEMP%)
- [x] 测试(全部真实沙箱内执行):探针机制、沙箱内 AC、用户目录读拒绝、
      网络拒绝(ping 套接字)、Python 判题、off 回落 Job-only
- [x] 全套既有评测测试(corpus/MLE/forkbomb/TLE/特判/交互/文件 IO)在
      默认沙箱路径下全绿——最强的回归证明
- [x] 修复过程中顺带:EXPLICIT_ACCESS 布局教训(勿手填,用 x/sys 类型)、
      GetNamedSecurityInfoW 描述符释放语义(勿 free 内部 DACL 指针)、
      SID 子授权小端
