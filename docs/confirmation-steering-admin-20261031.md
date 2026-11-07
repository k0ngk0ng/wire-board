# 确认框、转向与管理员战绩查看验收

2026-10-31。本批包括宝石确认框与保留卡确认、斯诺克左右细调、赛车速度相关转向，以及管理员用户统计／历史入口。斯诺克保留「按住击球键，再用上下调力度，松开击球」的原操作。

## 修改与验证

| 范围 | 结果与证据 |
| --- | --- |
| 宝石购买／盲预留 | 购买面板改为平台对话框，居中并留在游戏全屏目标内；优先聚焦可执行确认按钮，保留付款调整、查看牌桌、关闭和焦点恢复。修复前可购买按钮未获焦点、面板中心为 `(322,553)` 而视口中心为 `(720,422)`。 |
| 键盘短按 | 公共导航在按下／松开时立即采样，避免按键完整发生在两帧之间时被吞掉。保留长按单次确认和原有方向重复节奏。 |
| 宝石实际操作 | 桌面键盘自然完整局 `.local/splendor-animation-1440-e6030df7/`、320px 全屏模拟手柄局 `.local/splendor-animation-320-85cdfa12/`、390px 键盘局 `.local/splendor-animation-390-0a7f6d92/` 通过，包含实际购买保留卡、付款／贵族计分、取消不改状态、确认焦点可见、Tab 范围、盲预留及刷新不重播。 |
| 斯诺克 | 普通左右转向由每秒约 60° 调整至 15°，精细调整仍为十分之一；摇杆幅度保持连续比例。`.local/control-dialog-snooker-20261031/report.json` 通过，实际短按普通约 `0.0436rad`、精细约 `0.00454rad`；真实 HTTP 摆球／击球、原生键盘响应、混合输入、断连取消、全屏与 320px 触屏通过。 |
| 赛车规则与预测 | 原公式随车速增加转向强度。新公式在起步阶段保留必要转向，正常道路速度下随速度增大减弱；服务端与客户端预测同步修改，键盘、鼠标、手柄共用规则。先写速度、对称性、模拟量、静止与手刹边界测试并复现失败，再修改实现。 |
| 赛车完整比赛 | `go test ./games/racing/... -count=1 -timeout=25m` 通过（测试包 225.760 秒）。包含五赛道 × 二至六人 × 一／三／五圈的 75 场自然完整比赛；每辆车必须真实完赛，不能依靠比赛总时限结束。证据 `.local/racing-steering-go.log` 与 `.local/racing/module/`。 |
| 赛车输入与延迟 | `.local/racing-steering-inputs-20261031/report.json` 验证原生键盘、鼠标转向／松开、手机踏板、重连与画面；`.local/racing-steering-prediction-20261031/report.json` 验证 650ms 延迟下先预测后确认、模拟转向／油门／刹车、混合来源、断连复位、全屏和触屏。旧固定按键的极速探针在新转向下驶出道路，未将该失败记为通过，也未用它证明道路极速。 |
| 管理员历史权限 | 先写真实 HTTP 用例 `TestAdminInspectsUserHistoryThroughPublicHTTP`，通过 21 次真实开局／中止验证超级管理员、管理员、本人、普通用户、撤销权限和分页；不泄露手牌、密码或聊天。中止记录不增加胜场／已完成局数。 |
| 管理员实际页面 | 用户名复用统一玩家页入口，按需读取统计与历史。实际双人托管自然完赛生成一条记录，再生成一条中止记录；管理员看到两条，普通用户只看到公开的一条。`.local/admin-user-history-38c701a6/result.json` 保存自然结算（第 31 轮）；`.local/admin-user-history-41d8a181/result.json` 保存 1440／320px 页面、统计与历史滚动截图及普通刷新后仍保留查看目标的检查。 |

前端检查：公共 210、宝石 15、斯诺克 16、赛车 43 项通过。管理员、历史、资料和好友的现有 HTTP 回归通过。生产前端构建通过。

手柄证据使用虚拟 Gamepad API 硬件来源，不代表某个实体手柄型号已经实测。规则、HTTP、导航和最终动作接口保持真实；没有注入胜负、得分或车辆位置。临时本地 UI 服务只接入所需游戏，避免无关原生游戏运行时初始化影响验收。

## 复跑入口

先执行 `. scripts/env.sh` 与 `npm --prefix web run build`。使用已有的 `TestControllerGamepadBrowserFixture`，设置 `WIRE_BOARD_GAMEPAD_BROWSER=1`、新的工作区内 `WIRE_BOARD_GAMEPAD_BROWSER_OUT` 和 `WIRE_BOARD_GAMEPAD_BROWSER_WEB="$PWD/web/dist"` 后，运行 `go test ./internal/gametest -run '^TestControllerGamepadBrowserFixture$' -count=1 -timeout=12m`；服务地址写入该目录的 `url`，最后向同目录 `stop` 写入内容以结束服务。

```sh
INVITE_CODE=gamepad-browser python3 -B scripts/accept_splendor_animation.py --base-url "$fixture_url" --width 320 --acquisition --confirmation gamepad --fullscreen
python3 -B scripts/accept_admin_user_history.py --url "$fixture_url" --admin-name 手柄验收管理员 --admin-password gamepad-browser-test-only --invite gamepad-browser
python3 -B scripts/controllers/gamepad/check-browser.py --out .local/new-snooker-evidence --games snooker --web web/dist
python3 -B scripts/controllers/gamepad/check-browser.py --out .local/new-racing-delay-evidence --games racing --web web/dist
python3 -B scripts/racing/check-remake.py --out .local/new-racing-input-evidence --skip-maps --tracks metro
```

`fixture_url` 从测试服务生成的文件读取；账号和密码仅为该临时测试服务的固定测试账号。产物包含 JSON、截图、来源文件摘要及实际 HTTP／输入结果。
