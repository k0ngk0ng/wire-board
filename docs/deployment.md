# 服务器部署与运维

## Caddy HTTPS 反向代理

```caddy
board.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

设置 `PUBLIC_ORIGIN=https://board.example.com`、`COOKIE_SECURE=true`。Caddy 默认支持 WebSocket。域名 DNS 指向服务器，开放 80/443 端口。

## Nginx（已有证书）

在已有 HTTPS server 块中添加：

```nginx
location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_read_timeout 90s;
}
```

应用不会信任任意 `X-Forwarded-*` 头。认证限速按后端看到的客户端地址计算，反向代理后的多个朋友可能共用每分钟 20 次登录/注册限额。正常游戏行动不受该限额影响。

## systemd（二进制部署）

以下为服务器管理员执行的示例路径；根据部署目录调整。程序以专用低权限账号运行，二进制和静态资源只读，仅数据目录可写。

```ini
[Unit]
Description=Wire Board private board game lobby
After=network.target

[Service]
Type=simple
User=wire-board
Group=wire-board
WorkingDirectory=/opt/wire-board
EnvironmentFile=/opt/wire-board/.env
Environment=ADDR=127.0.0.1:8080
Environment=DATA_DIR=/var/lib/wire-board
ExecStart=/opt/wire-board/wire-board
Restart=on-failure
RestartSec=3
TimeoutStopSec=15
StateDirectory=wire-board
UMask=0077
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/wire-board

[Install]
WantedBy=multi-user.target
```

创建 `wire-board` 系统账号，将服务文件保存为 `/etc/systemd/system/wire-board.service`，然后执行 `systemctl daemon-reload` 和 `systemctl enable --now wire-board`。这是部署说明，GitHub Actions 不会自动执行这些服务器操作。

## 备份和恢复

数据库含账号密码哈希、有效会话和秘密游戏状态，应作为私有数据保存。备份时不要只复制正在写入的 `.db` 而漏掉 WAL。

最简单的可靠方式：

1. 停止服务或执行 `docker compose stop`。
2. 备份整个数据目录或整个 `board-data` 卷，包括数据库及存在的 `-wal`、`-shm` 文件。
3. 启动服务。已经提交的对局动作不会丢失。

也可以在服务器上安装 SQLite CLI 后使用 `sqlite3 wire-board.db '.backup backup.db'` 在线一致性备份。恢复时停止服务，替换数据目录，保留正确的文件归属权限后再启动。

## 更新和回滚

- 容器：`docker compose pull && docker compose up -d`。可以把镜像标签固定为 `v1.0.0` 或 Actions 输出的 `sha-...` 标签。
- 二进制：先备份，停止进程，替换二进制，重新启动。
- 当前版本只在启动时创建缺失表，不执行破坏性迁移。
- Actions 的 `verify` 工作执行规则测试、竞态检查、前端构建、Linux 二进制构建和真实 Linux 启动检查；通过后才发布容器。

## 常见问题

- **启动提示 INVITE_CODE**：程序不直接解析 `.env`。使用 Compose、systemd 的 EnvironmentFile，或通过 shell 导出环境变量。
- **登录后反复回到登录页**：HTTP 页面不能设置 secure cookie；内网 HTTP 设 `COOKIE_SECURE=false`，HTTPS 设 `true`。
- **请求来源不匹配**：`PUBLIC_ORIGIN` 必须与浏览器地址的协议、域名和端口一致，不要加末尾斜杠；代理需要保留 Host。
- **掉线重连中**：检查代理的 WebSocket Upgrade 设置。页面有定期同步兜底，已提交进度保留。
- **无法拉取 GHCR**：私有仓库镜像需要有权限的 GitHub 账号和 `read:packages` 令牌。也可使用无需镜像仓库认证的本地二进制部署。
- **忘记账号密码**：当前无邮箱找回；管理员应保留可用账号，或用邀请码注册新账号。不要直接分享数据库。
