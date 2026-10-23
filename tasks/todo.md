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
