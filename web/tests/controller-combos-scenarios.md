# FC／MD 连续触控与组合输入

## 霓虹外观与接触范围组合

用户参考来自 `Downloads/gamepad/gp_index.html` 与 `preview_gp.png`。本次只调整外观和默认布局，不接入编辑布局。MD 保留 X/Y/Z、A/B/C 六键及 Mode/Start，不显示 AC、ABC 或独立的组合连接控件；FC 保留 Turbo B/Turbo A、B/A 与 Select/Start。

实现前追加失败场景：单键中心误触相邻键；宽接触范围同时覆盖 A/B、B/C 的主体区域时只输出一个键；同一触点收窄后旧组合键未释放；没有面积信息时无法在相邻命中区域输入双键；动作手指跨入方向、系统或上排区域串键；已松开的手指释放另一输入来源；透明背景被整块底板替代；圆键霓虹颜色、内环与圆盘方向反馈缺失；320 px、横屏和晚加载游戏 CSS 使按钮重叠或小于 44 px。

原生 Touch 通过 radiusX/radiusY 提供接触椭圆；它是浏览器报告的近似范围，不代表精确的皮肤覆盖形状。有效范围按主按键圆形主体的覆盖采样判断；未知范围沿相邻主按键的共享命中区判断。单键中心、系统键、Turbo 和 X/Y/Z 不参与相邻主键的扩展命中。坐标和面积边界使用定点整数。保留原八向、多指归属、键盘合并和生命周期释放验收。

本轮结果写入 `.local/yuyuhakusho/controller-combos/contact-neon/` 与 `.local/contra/controller-combos/contact-neon/`；组件夹具不冒充真实游戏。重放仍使用 `node web/tests/controller-combos-browser.mjs`。

## 原连接控件方案的历史场景

以下为原方案的验收背景。当前组合输入与按钮数量以前述「霓虹外观与接触范围组合」为准。

实现前冻结失败场景：

- FC 的 A／B 交界触点只触发单键、空操作或先后两次按键；组合没有在同次输入中输出，或两个原按键没有同步高亮。
- MD 的 A／B、B／C 交界不能直接起按；A→AB→B→BC→C 滑动时旧键累加；AC／ABC 辅助键丢键，或发出未注册的逻辑按钮名。
- Turbo 区与普通动作区的行间空白、MD 的 X／Y／Z、Mode／Start 被错误组合；触控从动作区穿入方向区，或方向手指滑入动作区取得攻击。
- 八向仅能从四个方向按钮开始，空白斜角无法起按；↓↘→／↓↙← 连续滑动跳过斜向或保留旧方向。
- RetroArch 默认方向灵敏度 80 的低／高斜率界限被沿用旧的 0.5 判断替代；阈值与相邻量化整数不一致。
- 单键中心误触组合，交界区域无可见连接或没有触摸命中区域；AC／ABC 不足 44 px，覆盖六键、方向或公共浮层，或为新增操作压缩游戏画面。
- 组合手指抬起时释放仍被另一根手指或键盘按住的单键；松开方向手指时连带释放攻击。
- 取消、禁用、换配置、失焦、页面隐藏、旋转、指针模式变化、切换配置图或卸载后仍保留组合及按键高亮。
- 控制器配置模式多出没有实际键盘映射的组合字段，桌面暴露触屏浮层，或全屏切换后几何与命中错位。

原生 Touch 输入经过实际 FCController／MDController，以及魂斗罗 RuntimeInput／幽游白书 MDInput。浏览器输入为 CDP 触屏模拟，不能冒充实体手机、Safari 或完整规则对局。

重放：`node web/tests/controller-combos-browser.mjs`。MD 结果写入 `.local/yuyuhakusho/controller-combos/`，FC 结果写入 `.local/contra/controller-combos/`。

方向依据：[RetroArch input_driver.c](https://github.com/libretro/RetroArch/blob/master/input/input_driver.c) 的 `input_overlay_get_eightway_slope_limits` 与 `input_overlay_get_eightway_state`，以及 [config.def.h](https://github.com/libretro/RetroArch/blob/master/config.def.h) 的默认值 80。坐标先四舍五入到半宽／半高的万分之一，斜率量化为百万分之一；界限比较仅使用整数。平台继续保留中心 22% 的中立区和越界释放。

## 真实房间验收

组件夹具的截图不能当作实际游戏截图。正式应用须经过真实账号、SQLite 房间、WebSocket、游戏加载与原生运行时绘图；输入由原生 Touch 发出，再核对服务端的已应用按键与释放。截图须包含真正的关卡、人物与 HUD，不能以渐变背景或加载／介绍画面代替。

额外失败场景：游戏样式按需晚到时，MD 旧的 440 px 全屏宽度把右侧按键移入画面中央；固定左右 padding 把平板画面缩小；旋转后聊天浮钮覆盖 C 或其他控件；魂斗罗原版关卡介绍尚未通过 Start，画布没有实际关卡内容。

构建当前应用后，使用 `internal/gametest` 的 `TestControllerGamepadBrowserFixture` 启动隔离服务，设置 `WIRE_BOARD_GAMEPAD_BROWSER=1`、指向新建目录的 `WIRE_BOARD_GAMEPAD_BROWSER_OUT`、构建目录 `WIRE_BOARD_GAMEPAD_BROWSER_WEB`，以及当前魂斗罗公开运行时目录 `WIRE_BOARD_GAMEPAD_CONTRA_PUBLIC`。从输出目录的 `url` 读取服务地址，以 `CONTROLLER_ROOM_URL=<该地址> node web/tests/controller-room-browser.mjs` 重放；完成后在服务输出目录创建 `stop` 文件正常关闭。全部路径必须位于仓库内，结果与截图分别保存在 `.local/yuyuhakusho/real-touch/`、`.local/contra/real-touch/`。
