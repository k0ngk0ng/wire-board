# 触屏布局与试玩修正

## 参考与选择

[PUBG Mobile 官方操作说明](https://pubgmobile.helpshift.com/hc/en/3-pubg-mobile/faq/37-what-are-the-controls/)将左侧移动、空白区域拖动观察、独立攻击和右侧瞄准／蹲跳分开。[官方布局说明](https://pubgmobile.helpshift.com/hc/en/3-pubg-mobile/faq/280-i-want-to-change-the-location-of-the-control-key/)支持按玩家习惯调整控件位置。此次只参考操作分工，没有复制其美术或宣称操作布局完全一致。

反恐精英默认保留独立大开火键，按住后可继续拖动瞄准；左侧换枪和装弹，移动摇杆贴近底部安全边缘。右侧只保留开火、蹲下、跳跃及当前武器需要的开镜。使用、购买、战绩及不常用的武器模式放入「更多」。可选择右侧瞄准区按住开火，左侧移动、换枪、菜单和其他按钮不因此变成射击入口。

FC／MD 的游玩浮层只作用于手机触控。宽横屏中，控制器优先放在游戏画面两侧；小视区使用半透明贴底布局，竖屏保留操作空间。PC 和配置页面继续使用原控制器外观。坦克大战沿用方形战场，触控与场地图层共用布局区域，横屏将队伍资源放在两侧。

## 视区矩阵

[Apple iPhone Duo 技术规格](https://www.apple.com/iphone-duo/specs/)给出的内屏像素为 1878 × 2670，外屏为 1398 × 2034。626 × 890、466 × 678 及其旋转值按像素除以 3 构造，用于覆盖其显示比例；这不等于已经测得 Safari 的实际 CSS 视口或安全区。

控制器矩阵使用以下 CSS 视区：320 × 760、844 × 390、466 × 678、678 × 466、626 × 890、890 × 626、744 × 1133、1133 × 744、820 × 1180、1180 × 820、1024 × 1366、1366 × 1024。四款游戏共 48 组，每组覆盖普通布局与实际公共 GameFullscreen 壳，共 96 个布局状态；使用真实 Chromium 触摸事件检查双指、滑动换键、单指释放及旋转后的旧触点清理。全屏检查接入实际聊天定位 Hook，验证聊天按钮避让触控和必要画面，并覆盖坦克大战六名玩家的资源栏与方形画布。界面夹具不冒充完整规则比赛。

反恐精英另在正式 HTTP／WebSocket 页面验证左侧换枪、鼠标到触屏接管、可选瞄准区射击、相同画布跨视区与像素密度更新。890 × 626、2 倍像素密度时检查实际 1780 × 1252 绘图缓冲及独立绘图帧率。原键鼠 33 ms、触屏 51 ms 和显示预算仍在原测量窗口执行；截图和额外视区功能检查在原测量结束后进行，并明确记录范围。

桌面 Chromium 的触屏、尺寸和像素密度仿真不能验证实体 iPhone／iPad 的 Safari、刷新率、温度、续航或 GPU 性能；报告不作这些声明。

## 具体修正与证据

- 默认十席原先在前端强制满员，服务端也拒绝较少实际席位。真实 HTTP 验证现覆盖 2、3、6、10 人开局、空席换队，以及少人数返回房间后新增玩家再次开局。队伍人数只统计实际玩家。
- 原始 BSP 光照样本被统一按 sRGB 颜色贴图解码，暗部进一步衰减。光照贴图改按线性数据读取，颜色贴图保留 sRGB。
- 原枪口使用没有 alpha 贴图的 Sprite，形成矩形。现采用透明火光，并读取已有模型中从 MDL 导出的 attachment 节点，跟随骨骼与持枪镜像；弹道起点使用该枪口的投影。短暂弹道与弹着点通过有界对象池复用，伤害与人物命中仍由服务器事件决定。
- 首轮正式验收记录到首次显示射击特效时发生 299 ms 长任务，首个鼠标像素响应达到 301.4 ms，超出 33 ms 门槛。隐藏对象的 GPU 程序现与火光纹理一起在加载就绪前准备；验收继续保留首枪，不预射击或丢弃首个样本。原失败记录保存在 `.local/cs16/browser/reports/release-playability-d9b64c18/`。
- 冰雪世界的慢连接发送窗口占满时，旧脚本固定等待 60 ms 后读取已发消息，错误地把排队中的松手当作触点残留。现先在下一显示帧核对本地触控状态，再独立验证释放操作在 1 秒内离开发送队列；原生输入到像素的 51 ms 门槛与全部实测样本保持不变。
- 浏览器临时拒绝鼠标锁定后继续允许真实点击重试，不显示操作错误。游戏内误操作静默处理，无法加载时保留重试入口。
- 滚动页面的截图使用实际 scrollX／scrollY 作为页面裁剪起点，避免几何检查位于战场时却保存页面顶部；全屏裁剪沿用零滚动偏移。
- 本次共享控制器、聊天定位及四款消费者的完整文件 lint 范围登记在 `games/yuyuhakusho/build/lint-rules.json`；反恐精英及公共入口新增行的范围登记在 `games/cs16/build/lint-rules.json`。这两项检查不代表全仓库检查。

复跑：`node web/tests/controller-overlay-browser.mjs`、`node games/yuyuhakusho/tests/md-touch-browser.mjs`、`node --test games/cs16/tests/web-shot-effects.test.mjs games/cs16/tests/web-preparation.test.mjs games/cs16/tests/web-touch-ownership.test.mjs`，以及 `python3 -B games/cs16/tests/accept_browser.py`。产物分别位于 `.local/<game-id>/controller-overlay/`、`.local/yuyuhakusho/mobile-controls/` 和 `.local/cs16/verification/playability/`／`browser/reports/`。
