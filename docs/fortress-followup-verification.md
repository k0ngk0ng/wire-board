# 疯狂坦克、版本提醒与手机长按修复验收

本轮针对补给太密、箱子附加文字和边框、快速经过无法拾取、第二武器差异缺失，以及网页版本更新提醒和 iPhone Safari 长按菜单反馈。

## 行为变化

- 补给：新局第 8 个行动回合首次投放，之后间隔 8～12 个行动回合。原版箱子不再附带“补给”文字牌和金色外框。已排定的旧存档投放保持不变，后续使用新间隔。
- 拾取：首次有效接触即锁定领取者；180ms 只用于同步移动画面与库存，驶离不撤销。结算仍检查存活、库存容量和实体状态。已验证触碰后立即反向离开仍能拾取，且不重复加道具、不自动回血。
- 第二武器：14 车使用各自原版主／副弹体帧，保留方向、动画和锚点。恢复已证实的多弹数量、一次下落锁定、地雷、毒箭／毒雾、离子炮、移动限制和同队范围治疗。九发齐射不截断；每枚弹独立命中和结算，恢复不重复扣血。原始像素和行为证据可核验，不把未知的散布、检测半径、伤害、持续回合及动画毫秒称为原版精确参数，详见[游戏契约](../games/fortressblue/CONTRACT.md)。
- 地雷只向对应队伍投影，未揭示的地雷不出现在对手／观众视图和独立埋雷事件中。飞行及已发生爆炸仍是公开战场信息。
- 公共玩家栏图片补齐 CDN 失败后的同源回退。
- 页面每 60 秒以及切回可见状态时检查版本和入口资源，不读取完整游戏快照。更新提示可稍后查看，不强制刷新或挤动棋盘；[版本提醒](version-update.md)记录机制及验证。旧页面需先刷新一次才具备这项新能力。
- 泡泡堂摇杆和放泡按键接入原生触摸默认行为拦截，覆盖拖出控件、遗留普通选区、取消及多指；保留聊天复制。主要反馈环境为 iPhone Safari，也考虑其他浏览器。不将 Chromium 模拟触屏说成实体 iPhone 实测。

## 验收证据

| 范围 | 产物 |
| --- | --- |
| 14 车 × 主／副弹共 28 种真实 UI／HTTP 发射 | `.local/fortress-secondary/browser-projectiles/report.json`；按实际 Canvas 绘制检查资源、原生帧、弹数及每弹时间边界 |
| 锁定标记、双雷、延迟离子光束 | `.local/fortress-secondary/browser-effects/report.json`；另保留对应原生截图 |
| 断 CDN 后公共玩家栏／弹体回退 | `.local/fortress-secondary/browser-portrait-fallback/report.json` |
| 补给频率、运输、快速接触、刷新、手机布局 | `.local/fortress-supply-cadence/browser-final/report.json` |
| 手机触摸默认行为合同 | `.local/version-update/touch-accepted/report.json`，包括先有普通选区再按摇杆 |
| 实际泡泡堂 320／390px 输入与聊天选择 | `.local/version-update/bubble-final/report.json` |
| 版本提醒真实 60 秒计时、延后／重开／刷新、旧页面首次检测 | `.local/version-update/browser-accepted/report.json`；包含按需预加载资源不误报、无布局变化、未发送输入不丢失 |
| 公共平台 race、版本 HTTP | `.local/ci/shared/result-0.json` |
| 疯狂坦克完整规则／对局／race | `.local/fortress-secondary/rule-results.json` 绑定最终源码；完整 race 记录及友方伤害计分修正后的服务端 race／34 局补验均通过 |
| 公共前端与两款游戏前端 | `.local/fortress-followup-release/frontend.txt`，218 项通过 |
| v5 原图与发布 | `.local/fortress-secondary/source-pixels.json`、`oss-publish.json`；17 个图集、1085 帧、115326 字节，OSS／CDN 摘要及 MIME 一致 |

规则、素材分组和接口检查在实现前列出失败场景并记录失败结果；复杂交互通过浏览器保留 JSON 和截图。所有操作均由服务端判定，浏览器不注入游戏结果。

## 重复执行

先在仓库根目录执行 `. scripts/env.sh`。对应规则与竞态为 `go test -race ./games/fortressblue/server ./games/fortressblue/tests -count=1`，公共平台为 `python3 -B scripts/ci/scope.py run --game shared`。不因本轮修改运行其他游戏的规则矩阵。

浏览器运行前执行 `npm --prefix web run build`，运行期间不重建 `web/dist`：

```sh
python3 -B scripts/fortressblue/check-refactor.py --only projectiles --out .local/fortress-secondary/repeat-all
python3 -B scripts/fortressblue/check-refactor.py --only projectiles --tanks 6 9 10 --out .local/fortress-secondary/repeat-effects
python3 -B scripts/fortressblue/check-refactor.py --only supplies --out .local/fortress-supply-cadence/repeat
python3 -B scripts/mobile-controls/check-browser.py --games bubbleparty --out .local/version-update/repeat-touch
python3 -B scripts/mobile-controls/check-context-menu-contract.py --out .local/version-update/repeat-contract
python3 -B scripts/check-version-update.py --out .local/version-update/repeat
python3 -B scripts/fortressblue/verify-assets.py --dist
```

验收结束删除本次临时浏览器、测试数据库、socket、构建临时目录及被替代的中间运行，保留最终报告、截图和必要原始证据。不删除用户素材、原版客户端、已发布 OSS 对象或其他游戏文件。正式发布沿用 main 验证产物 → 版本 tag → GitHub Release → 生产版本更新，最后核对健康和上下架设置，并按约定保留两份可读数据库备份及当前／上一版镜像。
