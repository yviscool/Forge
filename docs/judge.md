# 评测基石:三平台语义矩阵

评测核(`internal/judge`)在三个桌面/服务器平台上的能力与降级路径。
所有探针**单向降级**:基础设施不可用时退回更弱的墙,绝不把探针失败
变成选手程序的 RE 或误判。

## 资源墙

| 维度 | Windows | Linux | macOS |
|---|---|---|---|
| 内存墙 | Job Object(Process+JobMemory) | cgroup v2 `memory.max`(委任时)→ prlimit `RLIMIT_AS` → 无 | `sh ulimit -v`(best-effort) |
| OOM 判定 | 峰值 ≥95% 墙 | cgroup `oom_kill`(确定性)→ 峰值贴墙 / 致命信号+bad_alloc 启发 | 峰值贴墙(经 rusage) |
| CPU 软限 | wall ∪ user-CPU 双口径(CCF 口径) | 同左 | 同左 |
| CPU 硬墙 | ctx 超时杀 + Job | prlimit `RLIMIT_CPU`(秒+2)/ ctx + 组杀 | ctx + 组杀 |
| 进程数墙 | Job ActiveProcessLimit | cgroup `pids.max` / prlimit `--nproc=256` | — |
| 杀树 | Job KILL_ON_CLOSE + TerminateJobObject | 进程组(Setpgid)+ cgroup.kill + 组杀看门狗 | 进程组 + 组杀看门狗 |
| 峰值上报 | Job QueryInformationJobObject | cgroup `memory.peak` → wait4 rusage 回填 | wait4 rusage |
| 解释型运行时 | Job 内存墙照常(工具链可配 `disableMemoryLimitCheck`) | 跳过 RLIMIT_AS(cgroup memory.max 仍生效,RSS 口径) | 跳过 ulimit -v |

## 沙箱(文件系统/网络隔离)

| 维度 | Windows | Linux | macOS |
|---|---|---|---|
| 机制 | LowBox 令牌(Chromium 同路 `NtCreateLowBoxToken`)+ 包 SID 工作目录 ACL | bwrap `--unshare-all`(可用且计量可穿透时) | 无 |
| 文件系统 | 工作目录 ACL 内可读写;目录外拒绝(跟随链接目标校验,符号链接攻击天然失效) | 命名空间内:/usr 只读、/tmp 私有、工作目录绑定 | 工作目录即边界(无强制) |
| 网络 | 无 capability → 套接字拒绝 | `--unshare-all` 断网 | 可达 |
| 降级 | 探测(容器内自证 + 越界读必须失败)失败 → Job-only | 探测(建命名空间 + CPU 计量穿透)失败 → prlimit-only | — |
| 开关 | `FORGE_WIN_SANDBOX=auto/off` | 自动探测,无开关 | — |
| 运行时目录 | 编译器 bin / Python prefix 对包 SID 只读(前缀经 `-I -S` 发现) | /usr 只读绑定 | — |

## 判定口径(三平台一致)

- 时间软限:wall 或 user-CPU 任一超限 → TLE;硬杀墙 = 软限 +
  max(2s, 2×软限) × 25%;软硬墙之间的完成视为边界抖动,自动重测
  (`RejudgeTimes`,缺省 1)。
- 比较:`line_by_line` / `ignore_space`(token 相同行分布不同 → PE,
  多尾部 token → OLE)/ `real_number`(绝对+相对误差)/ 三种特判协议 /
  `external_tool`(外部 diff)。
- 特判与交互器运行于独立资源墙(`runConfined`:最小环境 + 内存墙 + 组杀),
  超时 10s,故障落 CheckerError 不误报学生 RE;交互器可经 stderr
  testlib 标记(`ok` / `partially correct (N)` / `points X`)给部分分。
- 基础设施故障(目录创建失败、评测器自身崩溃)落 `SystemError`,
  与学生程序故障(RE)分离,支持一键重判。

## 诊断

- `forge doctor`:工具链解析 / 资源墙 / 沙箱探测 / 编译缓存可写性,
  一键矩阵输出;`/readyz` 的 judge 检查与 doctor 共用探针集。
- `/metrics`:`forge_judge_workers`、`forge_judge_jobs_*`、
  `forge_judge_cache_{hits,misses}_total`、`forge_judge_verdict_cases_total`、
  `forge_judge_duration_{count,sum_ms}`。

## 已证边界(测试锚点)

- 双平台(windows/ubuntu)每次推送:corpus 全矩阵、MLE 双路径、
  fork 炸弹遏制、TLE 双口径、三特判、交互、文件 IO、编译缓存、
  子任务依赖、pool 并发(-race 子集)。
- macOS:纯函数 + 物理核探测(sysctl)+ 轻量判题(heavy corpus 因
  clang 编译 bits 头过慢留在 linux/windows 跑)。
- fuzz:`parseTestlibResult` / `parseLemonScore` / `tokenLineSeq`(domain)/
  `SafeFileName` 常驻种子语料。
