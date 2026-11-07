# 玩家修改密码

点击右上角自己的头像或昵称，在「玩家页面」中选择「修改密码」。填写原密码、新密码及确认密码；新密码为 8–72 字节，不能与原密码相同。

修改成功后，当前浏览器保持登录并继续保留当前牌桌，其他旧登录会话失效。不会把自己的席位切为托管，也不改变昵称、角色、好友和战绩。密码以 bcrypt 保存；操作记录只保留「修改密码」，不记录密码内容。

通过 `ADMIN_USERNAME` 和 `ADMIN_PASSWORD` 配置的专用管理员仍由服务器私有配置维护密码，玩家页面会显示相应提示。通过 `ADMIN_EXISTING_USERNAME` 指定的现有账号可在校验原密码后自助修改，重启后保留。

## 接口

`POST /api/account/password`，使用当前登录会话和同源 JSON 请求，字段为 `currentPassword`、`newPassword`，不接受目标用户 ID。

| 状态 | 含义 |
| --- | --- |
| 200 | 修改成功，返回 `{ "ok": true }` 并更新当前浏览器会话 Cookie |
| 400 | 字段、长度、原新密码相同或请求格式不符合要求 |
| 401 | 未登录、账号已停用或并发操作撤销了当前会话 |
| 403 | 原密码不正确、请求来源不匹配或密码受服务器配置管理 |
| 409 | 处理期间账号密码发生变化，未覆盖另一项修改 |
| 429 | 同一账号一分钟内超过 5 次有效格式尝试 |
| 500 | 保存失败；密码、会话和操作记录在同一事务中回滚 |

密码校验和哈希计算在平台状态锁之外执行。写入前重新验证会话，密码更新、旧会话撤销、新会话创建及操作记录共同提交；失败时保留原凭据。

## 验证

失败场景先记录于 [HTTP 验收](../internal/server/account_password_e2e_test.go) 和[浏览器验收](../scripts/accept_account_password_and_starfort_result.py)，再编写实现。

```sh
. scripts/env.sh
go test ./internal/server -run '^TestAccountPassword' -count=1 -timeout=3m
go test -race ./internal/server -run '^TestAccountPassword' -count=1 -timeout=3m
npm --prefix web run build
python3 -B scripts/accept_account_password_and_starfort_result.py --skip-build
```

HTTP 记录位于 `.local/account-password/http/`，浏览器记录、截图和源码／构建摘要位于 `.local/account-password/browser/`。浏览器覆盖桌面鼠标和 320px Chromium 触屏仿真，不代表手机真机验收。
