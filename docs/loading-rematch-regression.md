# 重开加载回归

## 实现前失败场景

- 同一页面的 StrictMode、资源地址更新、卸载重挂和重试重复发送加载开始，较早请求后到达并覆盖当前会话。
- 新标签页必须具有独立会话，旧标签页的加载完成不能确认新页面。
- 重开后的 gate、房间、席位和构建仍由服务端 token 校验，不能通过接受旧会话绕过保护。
- 手机和平板加载时自动全屏，加载层位于全屏内部，不能被外层 inert 或隐藏的 staging 容器屏蔽。
- 加载失败后，重试、刷新、退出全屏都可以实际点击。
- 主动退出全屏后，加载层回到普通页面，页面菜单恢复操作；完成加载不能重新强制进入全屏。
- 连续重开沿用房间、席位和设置，加载完成后进入新对局，不遗留旧全屏、inert 或错误状态。

## 重放入口

先运行 `node games/yuyuhakusho/tests/recovery-server.mjs "$PWD/.local/web-public" --terminal`，再将打印的地址传入 `LOADING_REMATCH_URL`，执行 `node web/tests/loading-rematch-browser.mjs`。该入口使用真实登录、Go 房间、游戏加载和结算重开按钮；初始位置采用已有终局前一帧检查点，由原游戏规则正常完成比赛。延迟第一条加载开始请求，确定性覆盖网络乱序，并注入加载确认失败验证恢复操作。StrictMode 保持启用。报告和截图保存在 `.local/yuyuhakusho/loading-rematch/`。
