# 控制器全屏尺寸回归

修改实现前记录的失败场景：WebKit 中，真实四人幽游白书从普通游戏区进入网页全屏，画面高度由 440px 增至 744px，绝对定位的控制器浮层仍为 440px；旋转和窗口 resize 后仍可能保留旧高度。此前 Chromium 静态夹具没有真实房间的 GameViewportFit 和容器查询嵌套。

验收使用真实 Go 服务、SQLite 房间、WebSocket 和正式 CDN 运行时。覆盖幽游白书四人三电脑与魂斗罗，分别检查手机、Duo 双屏视区、平板横竖屏。每个尺寸从普通页面进入网页全屏，检查浮层与游戏区四边一致、控制器内部两层等高、方向盘和主按键行位于下半屏、全部按键至少 44px 且命中可见。全屏中旋转、缩小可视区域后复查；退出后确认页面滚动和浮层恢复；切换不能替换 Canvas 或重置房间。

`CONTROLLER_ROOM_URL` 指向 `internal/gametest/TestControllerGamepadBrowserFixture`；`CONTROLLER_BROWSER_MODULE` 为已安装 Playwright 的绝对模块地址，`PLAYWRIGHT_BROWSERS_PATH` 指向已安装的 WebKit。运行 `node web/tests/controller-fullscreen-browser.mjs`。默认使用编译产物，`CONTROLLER_SOURCE_STYLES=1` 仅用于在正式构建前将当前幽游白书 CSS 注入旧版真实房间，报告明确登记；发布前必须以新构建关闭该开关再次通过。

结果、源码摘要、每个尺寸的实际几何和截图保存在 `.local/yuyuhakusho/fullscreen-layout/<CONTROLLER_FULLSCREEN_RUN>/`。该测试是 WebKit 模拟环境，不代表实体 iPhone、Duo 或 iPad 已实测；Chromium 使用既有 `controller-overlay-browser.mjs` 和 `controller-room-browser.mjs` 补充验证。
