# 服务器部署与运维

## Caddy HTTPS 反向代理

```caddy
board.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

设置 `PUBLIC_ORIGIN=https://board.example.com`、`COOKIE_SECURE=true`。Caddy 默认支持 WebSocket。域名 DNS 指向服务器，开放 80/443 端口。

## Nginx + Docker + Certbot

服务器需要 Docker Compose v2+、Nginx、Certbot 和 `flock`。以下域名、端口是示例；实际值只写入服务器配置，不写入源码。应用部署目录可使用 `/opt/wire-board`。

1. 将 `compose.yaml`、`scripts/update.sh`（安装为 `update.sh`）、`.env.example` 放入部署目录；复制 `.env.example` 为 `.env` 并设为 `0600`，配置随机邀请码、实际 `PUBLIC_ORIGIN`、`COOKIE_SECURE=true`、`BIND_ADDRESS=127.0.0.1` 和空闲的 `PORT`。
2. 执行 `chmod +x update.sh && sudo ./update.sh`。只下载 GitHub 发布的镜像，不在服务器构建。
3. 域名解析到服务器，开放 80/443。创建 `/opt/wire-board/acme/.well-known/acme-challenge`，确保 Nginx 可以遍历父目录。先配置仅监听 80 的 server 块：

```nginx
server {
    listen 80;
    server_name board.example.com;
    location ^~ /.well-known/acme-challenge/ {
        root /opt/wire-board/acme;
        try_files $uri =404;
    }
    location / { return 503; }
}
```

执行 `nginx -t && systemctl reload nginx`，确认公网能读取 challenge 目录的测试文件，再申请证书（将域名和联系邮箱替换为服务器实际配置）：

```sh
certbot certonly --webroot -w /opt/wire-board/acme \
  -d board.example.com --non-interactive --agree-tos --email admin@example.com
```

4. 证书签发后，将配置替换为：

```nginx
map $http_upgrade $wire_board_connection {
    default upgrade;
    '' close;
}
server {
    listen 80;
    server_name board.example.com;
    location ^~ /.well-known/acme-challenge/ {
        root /opt/wire-board/acme;
        try_files $uri =404;
    }
    location / { return 301 https://$host$request_uri; }
}
server {
    listen 443 ssl;
    server_name board.example.com;
    ssl_certificate /etc/letsencrypt/live/board.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/board.example.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $wire_board_connection;
        proxy_read_timeout 90s;
    }
}
```

执行 `nginx -t && systemctl reload nginx`。创建专属续期钩子 `/etc/letsencrypt/renewal-hooks/deploy/wire-board-nginx`，并赋予执行权限：

```sh
#!/bin/sh
set -eu
[ "${RENEWED_LINEAGE:-}" = /etc/letsencrypt/live/board.example.com ] || exit 0
/usr/sbin/nginx -t
/bin/systemctl reload nginx
```

启用 `systemctl enable --now certbot.timer`，运行 `certbot renew --cert-name board.example.com --dry-run` 验证续期。实际签发后钩子会重载 Nginx 使新证书生效；旧版 Certbot 的 dry-run 不执行 deploy hook，需要单独验证钩子。

## Nginx（已有 HTTPS 站点）

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

- 容器：进入部署目录运行 `sudo ./update.sh`；可传版本标签。脚本先拉取 GHCR 镜像、校验 Compose 配置，再通过新镜像内的 SQLite 工具在线备份数据库、校验并压缩到私有 `backups/`；这些准备工作不停止正在玩的服务。准备完成后才由 Compose 停止旧容器，按不可变 digest 启动新容器。重复运行仍会执行备份与容器更新。
- 在线备份包含已提交的 WAL 数据，归档内只有完整的 `wire-board.db`，不依赖额外的 WAL 文件。它反映备份时点；备份后提交的动作继续写入原数据卷，更新直接使用该卷，不会用较早的备份覆盖它。恢复在线快照时必须先停服，将旧数据库及其 `-wal`、`-shm` 一并移走留存后再放入快照。
- 下载、备份、压缩耗时不再计入停服窗口。正常情况下目标为几秒内切换；数据库规模、磁盘速度、正在处理的请求与容器启动失败都可能增加耗时，不能保证固定秒数。应用退出会等待正在处理的 HTTP 请求；容器每 2 秒探测健康状态，浏览器断线后约 250–400ms 开始重连，失败时逐步退避至约 2 秒。旧页面刷新后才获得新的重连逻辑。
- 脚本固定 Compose 项目名 `wire-board`。既有手动部署若使用其他项目名，应先安排数据迁移，不要直接用脚本创建第二套服务。
- 成功的镜像 digest 保存到 `.image.env`；手动启动时使用 `docker compose -p wire-board --env-file .env --env-file .image.env up -d --no-build`，避免绕过版本固定。
- 下载、配置校验或在线备份失败不停止现有服务；切换阶段新容器启动失败会尝试重启旧镜像。数据库不会自动回退，以免丢失新写入。若跨版本出现不兼容迁移，应停服并从备份手动恢复。备份包含敏感数据，保存在服务器上，不自动删除，需自行管理保留周期和异机备份。
- 更新脚本不会改写 `.env`、Nginx 或证书配置，也不会自动更新自身。升级部署工具时，从所选 GitHub Release 取出 `update.sh` 和 `compose.yaml` 后替换。在线备份脚本需配合含 `sqlite3` 的新版镜像；不满足时会在停服前退出，自动回滚旧镜像不受此限制。
- 二进制：先备份，停止进程，替换二进制，重新启动。
- 当前版本只在启动时创建缺失表，不执行破坏性迁移。
- main 提交触发 `Check main and build`，执行规则分片、竞态检查、扩展组合验收和构建。该工作流不发布镜像。版本 tag 触发 `Publish verified build`，复用相同提交的已验证构建产物。

## 常见问题

- **启动提示 INVITE_CODE**：程序不直接解析 `.env`。使用 Compose、systemd 的 EnvironmentFile，或通过 shell 导出环境变量。
- **登录后反复回到登录页**：HTTP 页面不能设置 secure cookie；内网 HTTP 设 `COOKIE_SECURE=false`，HTTPS 设 `true`。
- **请求来源不匹配**：`PUBLIC_ORIGIN` 必须与浏览器地址的协议、域名和端口一致，不要加末尾斜杠；代理需要保留 Host。
- **掉线重连中**：检查代理的 WebSocket Upgrade 设置。页面有定期同步兜底，已提交进度保留。
- **无法拉取 GHCR**：本项目镜像公开，可匿名拉取。若 fork 使用私有镜像，需要有权限的 GitHub 账号和 `read:packages` 令牌；源码仓库和镜像的可见性是独立设置。
- **忘记账号密码**：当前无邮箱找回；管理员应保留可用账号，或用邀请码注册新账号。不要直接分享数据库。

## 发布已经验证的构建产物

完整检查只在 main 提交时运行，版本 tag 不重复执行规则分片、竞态检查或完整对局矩阵。发布约定同时记录在仓库根目录的 [AGENTS.md](../AGENTS.md)。

正常流程：将提交推送到 main，等待该提交的 `Test and Linux amd64 binary` 作业成功，再为同一提交推送 `vX.Y.Z` tag。tag 自动触发 `Publish verified build`，查找同一 SHA 的 main 构建及未过期的 `wire-board-linux-amd64` 产物。若 tag 比构建先到，工作流会短暂等待；找不到匹配产物、构建未通过或来源存在已知失败检查时，停止发布，不重跑整套检查，也不改用其他提交。

也可手动运行 `Publish verified build`，指定 `source_run`、40 位 `source_sha` 和 `version`。这保留了旧版发布入口，并允许显式选择同一版本历史 tag 流程中已成功的构建作业，便于迁移尚未发布的版本。现有版本 tag 必须指向指定 SHA；已公开的 Release 不会被覆盖。已有 tag 而没有 Release 时，按该 tag 自动创建草稿；发布内容以已核对的 tag SHA 为准。手动发布尚无 tag 的版本时，须先由发布者创建目标为完整 SHA 的草稿。

发布流程下载并核验原构建附件的 SHA-256，沿用来源提交 Dockerfile 的运行阶段打包，不重新编译应用。容器启动及 SQLite 备份工具检查通过后，发布版本、提交和 `latest` 三种镜像标签，上传原附件并公开 Release。main 构建产物保留 30 天；产物过期时须先在 main 恢复对应提交的有效构建，不能让 tag 发布绕过来源校验。

功能代码的验证在 main 的检查流程和开发验收中完成。发布复用成功的二进制作业，不要求再等一套由 tag 触发的矩阵；已知失败不得被当作通过。不要为了发版取消正常的 main 检查或另启重复的完整检查流水线。

服务器更新执行既有 `update.sh vX.Y.Z`，由脚本在线备份数据库并固定镜像 digest。部署不得自动上架新游戏。
