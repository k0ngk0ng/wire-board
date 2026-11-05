# 公共界面与三／四人模式验收

本修订通过真实平台的认证、SQLite、WebSocket、加载门槛和本地独立运行时验收。公共能力由模块显式启用；默认模块的创建、等待、玩家栏、棋盘、回合及全屏 DOM 契约保持一致。没有修改规则、网络时钟、RPC 或控制器映射。

## 变更

- 幽游白书的公共玩家栏同时显示体力、灵力与必要状态。四人组队用公共分组框表达 A 队和 B 队；三人自由战不显示队伍分组。
- `preparationOwnsOptions` 只移除启用模块在等待页的重复规则表单，创建页继续使用 Options。
- `viewportFit` 按实际公共区域及可视视口分配游戏剩余高度，合并布局事件并只写变化的尺寸。开启后跳过进入游戏的自动滚动。没有隐藏或移动公共界面。
- compact 回合栏保留有效倒计时和同尺寸的隐藏占位。托管使用既有统一入口及指令；限域移除重复横幅，并通过 `GameFullscreen.toolbarActions` 在原 36 px 全屏工具栏显示「托管中」和「取消托管」。

## 实际结果

| 验收项 | 三人自由战 | 四人组队战 |
| --- | --- | --- |
| 平台席位对应原版槽位 | 0、1、3 | 0、1、2、3 |
| 原版 baseTeam | 1、2、4 | 1、1、4、4 |
| 公共分组 | 无 | 0／1 与 2／3 |
| 普通页开场／进行／暂停／托管／结算 Canvas 顶边 | 均为 396 px | 均为 401 px |
| 57 个权威帧内额外 fit 测量 | 0 | 0 |
| 公共文字最低计算对比度 | 4.989:1 | 4.698:1 |
| 准备下拉框最低计算对比度 | 11.748:1 | 11.748:1 |
| 320 px 普通页 C 按钮宽度 | 29.95 px | 29.50 px |
| 740×360 全屏 C 按钮宽度 | 26.95 px | 26.95 px |
| 原版 AI 自然结束帧 | 5090 | 4628 |

两种模式都覆盖桌面、320×740 竖屏和 740×360 横屏：真实触屏按钮、ESC、同 Canvas／epoch、托管期间手动输入阻断、同 epoch 实际刷新，以及全屏触屏取消托管后直接恢复原生 D 按键。全屏接手前后 Canvas 顶边差均为 0 px，入口与退出按钮不重叠。禁用手柄按钮和聊天输入框没有游戏输入穿透。原生滚轮滚动没有造成 fit 扩张或收缩。

完整测试的初次零值截图仍显示平台结算浮层，因此只作为几何记录。后续通过真实登录重新打开这两局已完成的 SQLite 对局，核对同一 epoch／结束帧，正常关闭结算浮层，再检查全部体力／灵力文字的实际 ink 边界、可见性和命中遮挡。三人 6 项、四人 8 项零值均在 320×740 视区内清楚可见；关闭浮层前后画布顶边保持一致。未写入规则状态或伪造零值。

## 证据与复跑

- 完整场景：[三人结果](../platform-mode-ui/public-ui-three-fullscreen03/result.json)、[四人结果](../platform-mode-ui/public-ui-four-fullscreen01/result.json)。每份结果绑定执行脚本、正常 app build、席位、原生输入和截图。
- 零值真实可见性：[恢复验收](../platform-mode-ui/public-zero-visible-restored02/result.json)、[三人截图](../platform-mode-ui/public-zero-visible-restored02/three-free-zero-resources-uncovered-320.png)、[四人截图](../platform-mode-ui/public-zero-visible-restored02/four-team-zero-resources-uncovered-320.png)。
- 默认组件与可选能力：[host 契约](host-ui-contract-final.json)、[Turn 契约](host-turn-contract-final.json)、[计时器计算样式](host-turn-browser.json)、[全屏契约](fullscreen-slot-contract-final.json)、[fit 契约](viewport-fit-contract-final.log)。TypeScript 与差异空白检查通过。
- 测试先行与实际失败：[公共失败范围](PUBLIC_UI_OPTIN_FAILURES.md)、[test-first](public-ui-optin-test-first.json)、[全屏缺失入口 red](../platform-mode-ui/public-fullscreen-autoplay-red02/result.json)、[全屏 test-first](fullscreen-slot-test-first.json)。刷新观察器及尺寸等待的测试设置修正单独留有 lineage，没有把设置错误当作产品回归。

先执行 `python3 scripts/yuyuhakusho/build_platform_app.py` 生成本地验收构建，再用新的 `--label` 执行 `python3 scripts/yuyuhakusho/check_platform_modes.py --label <新标签> --mode three-free --public-ui --zero`，以及 `--mode four-team`。零值关闭浮层后可按 `check_public_zero_visibility.py` 的相同 UI 流程检查；该定向脚本绑定此处保留的两局完成数据库。

本地 app build 为 `9bd242d381ba458b0c48041fbe75c073ce5922ca0f24c38b9f9fbd9e7ecf66bd`。Wasm 为 `5d8febf5b5fa91d1348314e41f62e26f3a3f7ccd1cb10b418465dff42a7ad92b`，4,911,229 字节；与原 `2ac14ca9` 的 110 个链接源码及嵌入输入逐字相同，二进制差异来自较新的 Git 元数据。原先各阶段 manifest 和构建证据保留不变。本修订不覆盖此前的网络性能验收，也不声明完成音频或正式发布。

只清理本次已关闭的浏览器临时 profile；记录见 [清理范围](cleanup-public-ui-profiles.json)。验收证据、当前本地构建、共享缓存、数据库和其他工作者文件保留。
