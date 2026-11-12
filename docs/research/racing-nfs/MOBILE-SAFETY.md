# 赛车触屏输入与防选中验收

2026-11-12。用户报告手机左转持续按住、游戏区出现网页选择效果。本轮修复保留左侧双箭头、右侧油门／刹车及右下边缘小手刹的布局。

## 原因与修复

原按钮仅依赖 pointer 松开／取消／捕获丢失事件释放输入，且没有声明平台长按保护所需的 `data-game-hold-control`。当浏览器菜单或触摸路由中断 pointer 通知时，旧方向可能残留。HUD 和 Canvas 也没有覆盖整个游戏区的禁止选择规则。

持续按钮现使用原生 Touch identifier 管理各根手指，并在每次触摸事件中按 `event.touches` 核对存活触点。pointer 输入只负责鼠标和笔；触摸不重复进入 pointer 来源。转向和踏板分别允许组内滑动，滑出命中区释放当前动作。松开或取消触点不清除其他手指、键盘和 Gamepad 的输入。

整个比赛区域使用 `user-select:none`、`-webkit-user-select:none` 与 `-webkit-touch-callout:none`，拦截区域内的 `contextmenu`、`selectstart`，并清除与比赛区域相交的选区。实际编辑控件保留选择和呼出菜单；普通点击按钮不声明持续按住，不阻止视角、地图等触摸点击。

## 可重复执行的验证

先行失败清单为 `games/racing/tests/touch-contact-failures.md`。原版本浏览器复现报告为 `.local/racing-nfs-browser/safety-before-02/product-failures.json`，包含九项有效产品失败。故障注入仅阻断 window 捕获阶段的 pointer 松开通知，不伪造原生触点列表或游戏状态。

```sh
. scripts/env.sh
node --experimental-strip-types --test games/racing/tests/touch-contacts.test.mjs
npm --prefix web test -- racing
npm --prefix web run build
python3 -B scripts/racing/check-nfs.py --out .local/racing-nfs-browser/safety-new --phase safety
```

冻结版本的 `.local/racing-nfs-browser/safety-green-01/report.json` 为通过，包含 26 项检查。覆盖双指分别抬手、丢失 pointer 松开通知、全部抬手、新手势恢复、滑出按钮、焦点与键盘重复、多来源混合、普通点击、HUD／Canvas／按钮长按、拖动、原生右键菜单，以及聊天编辑选择。原生触点单独留下油门时，转向为 0；触点全部离开后，转向和油门均为 0。

隔离触点检查 4 项通过；赛车前端合计 106 项通过，全树 TypeScript 检查及正式前端构建通过。日志保存在 `.local/racing-touch-safety/`，浏览器报告、输入记录和截图保存在上述 safety-green-01 目录。

## 验证边界

本轮浏览器验证为 320px Chromium 原生 CDP 触屏仿真，不是实体 iOS／Safari 测试。pointer 通知中断为有明确边界的故障模拟；未将 Chromium 未复现的 iOS 系统「全选」菜单写成已实机复现。`-webkit-touch-callout` 在 Chromium 中不受支持，因此核对发布 CSS 声明，不声称取得 Safari 计算样式。Gamepad 验证模拟 API 硬件边界，未声称实测具体实体手柄。
