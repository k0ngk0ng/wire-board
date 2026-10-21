# 网页版本更新提醒

页面打开后读取 `/api/version`，随后每 60 秒检查一次；页面重新可见及从浏览器历史恢复时立即检查。隐藏页面不发起轮询。接口只读取入口 HTML 和发布版本，不读取房间、完整游戏快照或数据库。

检测同时比较当前页面加载的 JS／CSS 资源地址与服务器实际入口资源，以及本页首次确认的发布版本。这样可以发现仅后端更新、仅 CSS 更新，以及旧页面首次请求就碰到新服务器的情况。入口资源在组件加载时固定，游戏之后按需追加的预加载资源不会误报新版本。接口与入口 HTML 使用 `Cache-Control: no-store`，请求使用 `cache: no-store`。

发现变化后显示可收起、可重开的更新提示。提示位于既有弹窗和错误提示下方，不占棋盘布局位置、不清除输入、不自动刷新；只有玩家点击「刷新更新」才重新加载页面。相同版本重复响应不会反复展开提示，网络失败不清除已经发现的更新。请求不并发，10 秒超时，组件卸载时取消请求及监听。

首次上线这项能力后，旧页面需先正常刷新一次才具备版本检测逻辑。

## 验收

- 服务端实际 HTTP 验收：`TestPublicVersionMetadataHTTP`，覆盖未登录访问、资源与版本绑定、资源变化和禁缓存。
- 前端四项针对性检查：`web/tests/version-update.test.mjs`，覆盖首次旧资源、CSS／后端更新、回滚、无效响应、失败、并发及卸载。
- 浏览器入口：`python3 -B scripts/check-version-update.py --out .local/version-update/repeat`。使用实际 React 组件与 HTTP 元数据夹具，运行真实 60 秒定时器、原生点击、320px／桌面截图，检查棋盘位置与未发送输入不变。可见性检查使用显式可见性事件，不冒称实体手机后台切换。
- 本轮结果：`.local/version-update/browser-accepted/report.json`；截图 `mobile-update.png`、`desktop-update.png`。

脚本结束清理本次浏览器 profile、临时站点和 socket，保留 JSON 与截图。
