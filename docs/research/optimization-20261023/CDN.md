# 游戏代码分发核查

2026-10-23 核查线上 v1.29.1（提交 `19894946bb353534fc2481c8542e0c17861f12f1`）：所有 25 款游戏的 69 个前端代码依赖均使用 `https://cdn.ichenj.com/bga/web/<提交 SHA>/`。这已在线上生效，不是本轮才迁移的功能。

## 实测

- 清单中的所有代码对象均从 CDN 回读成功，大小与清单一致，带 gzip、长期不可变缓存和跨域响应头。
- 独立浏览器首次打开登录页仅请求两项 JS/CSS，共传输 199,734 字节；刷新后两项资源传输均为 0，直接命中浏览器缓存。
- 首次入口 JS 实测约 82 ms、CSS 约 65 ms。测量来自开发网络，不代表玩家手机网络；尚不能据此确定玩家所述慢加载的原因。
- 服务器 `ASSETS_BASE_URL` 为 `https://cdn.ichenj.com/bga/v20261002-1`，普通素材优先走 CDN；部分游戏保留素材失败时的本地回退。
- 公共结算组件直接随应用加载。最新本地构建中，开局后阻断新增 JS/CSS 请求，经过 54 步合法操作结束掼蛋对局，结算展示、关闭、重新打开、刷新和再来一局均通过。

服务器继续提供入口 HTML、资源清单、鉴权、房间、存档和对局通信，并运行权威规则。公开前端代码由 OSS/CDN 分发；服务端规则和私有对局数据不发布到 OSS。

## 发布约束

main 的必需检查通过后上传编译资源，回读校验 SHA-256、MIME、CORS，再从服务器包移除编译资源。tag 复用同一提交的已验证构建，不重新编译或重复上传，不覆盖旧代码对象。规范已写入仓库 `AGENTS.md` 的「游戏加载」段落。

## 复验

在仓库根目录执行：

```sh
. scripts/env.sh
python3 -B scripts/game-loading/audit-online.py --browser
python3 -B scripts/game-loading/check-results.py
```

第一项只读取线上公共资源，不登录、不创建房间、不上传素材。产物为 `.local/game-loading-online/http.json`、`browser.json` 和登录页截图。第二项使用本地生产构建及测试房间，产物为 `.local/poker-layout/results/result.json` 和结算截图。HTTP 回读大小检查不能替代发布时的 SHA-256 校验。
