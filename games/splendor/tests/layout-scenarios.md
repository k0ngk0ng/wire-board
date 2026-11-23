# 手机与平板牌桌布局验收

实现前冻结的失败场景：

- 普通模式的旧样式覆盖牌市尺寸，导致发展卡和牌堆被压扁，费用数字重叠或超出牌面。
- 钱包六列容纳不下永久折扣和筹码，数字跨列重叠；贵族费用被裁切。
- 横屏按宽度放大牌市，忽略浏览器工具栏占用后的高度，操作区溢出或产生大片空白。
- 全屏的退出、倒计时和聊天按钮覆盖卡牌、筹码、玩家资源或确认操作。
- 手机竖屏或四人扩展局的玩家栏挤出牌市，操作区不能独立滚动。
- 横竖屏旋转或切换全屏清空筹码选择、重挂载牌市或重置回合。
- 只检查 DOM 存在，遗漏数字真实可见性、触摸目标和遮挡。

重放使用 `TABLETOP_SPLENDOR_ONLY=1 TABLETOP_SOURCE=1 TABLETOP_LAYOUT_REPORT=responsive TABLETOP_ROOM_URL=http://127.0.0.1:<port> node web/tests/tabletop-room-browser.mjs`。服务端须使用独立的 `.local/splendor/` 数据目录及测试邀请码 `gamepad-browser`。`TABLETOP_SOURCE=1` 在工作区内启动源码预览并代理该测试服务端，不运行其他游戏的对局。

服务端可设置 `ASSETS_BASE_URL=https://assets.example.test/media`；浏览器仅对该测试域名从 `.local/splendor/fullscreen/media/` 读取已有宝石素材，不下载或发布资源。结果、真实房间截图和源码摘要保存在 `.local/splendor/fullscreen/<report>/`。测试覆盖普通模式、原生全屏和网页全屏，包含 320 px 手机、手机横屏、平板横竖屏、四人扩展及真实触摸选取和确认。
