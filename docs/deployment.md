# Forge 部署手册

## 1. 支持的生产形态

当前推荐单节点部署：

- 一个 Forge Go 进程。
- 一个反向代理终止 TLS 并提供域名。
- 一个本地持久目录保存 SQLite 和测试数据。
- 外部对象存储或文件备份保存定期快照。

SQLite 是单写者模型，不支持把同一数据库目录挂到多个 Forge 副本。需要水平扩容时，
应先新增共享 `ports.Store` 适配器并完成并发评测与会话一致性设计。

## 2. 主机准备

Linux 示例：

```bash
sudo useradd --system --home /var/lib/forge --shell /usr/sbin/nologin forge
sudo install -d -o forge -g forge -m 0750 /opt/forge /var/lib/forge
sudo install -d -o forge -g forge -m 0750 /var/lib/forge/data
sudo install -m 0755 forge /opt/forge/forge
sudo install -m 0640 -o root -g forge /dev/null /etc/forge/forge.env
```

环境文件示例：

```dotenv
FORGE_ENV=production
FORGE_ADDR=127.0.0.1:8080
FORGE_DATA_DIR=/var/lib/forge/data
FORGE_STORE=sqlite
FORGE_ADMIN_PASSWORD=replace-with-a-long-random-secret
FORGE_AUTOJUDGE=0
FORGE_TRUST_PROXY=1
FORGE_MIN_FREE_DISK_MIB=2048
```

生产模式会拒绝 `FORGE_STORE=memory` 和未隔离的本地自动评测。只有开发/演练环境才可
设置 `FORGE_AUTOJUDGE=1` 与 `FORGE_ALLOW_UNSAFE_LOCAL_JUDGE=1`；这两个开关意味着
本机将执行考生代码。商业生产必须接入独立低权限沙箱 worker 后才能重新开放自动评测。

## 3. systemd

创建 `/etc/systemd/system/forge.service`：

```ini
[Unit]
Description=Forge Contest Platform
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=forge
Group=forge
WorkingDirectory=/opt/forge
EnvironmentFile=/etc/forge/forge.env
ExecStart=/opt/forge/forge
Restart=on-failure
RestartSec=3
UMask=0077
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/forge/data
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

启用：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now forge
sudo systemctl status forge
curl --fail http://127.0.0.1:8080/healthz
curl --fail http://127.0.0.1:8080/readyz
```

## 4. 反向代理与 TLS

Nginx 示例：

```nginx
server {
    listen 80;
    server_name contest.example.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name contest.example.com;

    ssl_certificate /etc/letsencrypt/live/contest.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/contest.example.com/privkey.pem;

    client_max_body_size 12m;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Real-IP $remote_addr;
    }

    location /api/v1/events {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_buffering off;
        proxy_cache off;
        proxy_read_timeout 1h;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

只有确认所有入口流量都经过可信代理时才能设置 `FORGE_TRUST_PROXY=1`。否则学生可
伪造转发 IP 头，绕过机位绑定。

生产必须使用 HTTPS。Forge 在 TLS 直连或可信代理传入
`X-Forwarded-Proto: https` 时，会为普通会话、试卷解锁和 SSE 票据 Cookie 加上
`Secure`；`HttpOnly` 与 `SameSite=Strict` 始终启用。不要通过明文 HTTP 开放考场。

## 5. 数据备份

至少每日备份：

- `forge.db`
- `forge.db-wal`
- `forge.db-shm`
- 整个 `testdata/` 目录
- 当前部署的环境配置（不含明文密钥）与二进制版本

仓库提供带锁、完整性校验和校验和的冷备份脚本：

```bash
sudo FORGE_DATA_DIR=/var/lib/forge/data \
  FORGE_BACKUP_DIR=/var/backups/forge \
  FORGE_SERVICE=forge \
  bash ./scripts/backup.sh
```

脚本会短暂停止服务、执行 `PRAGMA integrity_check`、打包数据库/测试数据、生成
SHA-256，并在退出时恢复服务。恢复时先停止服务，将快照展开到全新的数据目录，核对
`SHA256SUMS`，确认所有者为 `forge:forge`，再启动服务。恢复完成后必须访问
`/readyz`，并执行一场草稿比赛的导入和开赛预检。

## 6. 发布流程

1. 在测试库运行完整验收命令。
2. 构建前端并确认 `internal/web/dist/index.html` 存在。
3. 交叉编译目标平台二进制，记录 SHA-256。
4. 创建数据库与测试数据备份。
5. 停止旧进程，替换二进制，启动新进程。
6. 检查 `/healthz`、`/readyz`、登录、比赛详情、一次答案题提交和一次教师写操作。
7. 观察 15 分钟错误日志与 SSE 重连指标。

回滚只需恢复上一版二进制和对应数据快照。若数据库迁移不可逆，必须先停写并整体恢复。

## 7. 上线检查表

- [ ] 管理员密码已从一次性引导文件改成长随机密码，并删除 `bootstrap-admin-password.txt`。
- [ ] 默认演示账号和测试数据库没有进入生产。
- [ ] `FORGE_TRUST_PROXY` 与真实拓扑一致。
- [ ] 自动评测是否开启已经明确决策；未隔离环境保持关闭。
- [ ] 数据目录权限仅服务账号可读写，备份可恢复。
- [ ] TLS、DNS、反向代理 SSE 超时和上传上限已验证。
- [ ] 浏览器开发者工具确认 `forge_session`、`forge_exam_unlock`、`forge_stream`
      均为 `HttpOnly`，且 HTTPS 下均为 `Secure`。
- [ ] 严格时间窗、IP 绑定、试卷口令按学校考务策略配置。
- [ ] 教师完成了 PDF、CSV、比赛包导入导出验收。
- [ ] 值班人员知道如何查看日志、重连 SSE、回滚版本和恢复备份。
