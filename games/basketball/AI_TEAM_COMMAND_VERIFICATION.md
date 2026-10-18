# 原版队友指令恢复

本轮接通真人无球、电脑搭档时长按 A/B 的原指令流程。仅使用原版策略、请求和动作接口；未新增自创电脑策略。平台可选战术范围尚未因此扩大。

## 先验失败场景

测试先于实现，覆盖：错误接令、纽约高台限制、队友受伤时拒绝、负状态忙碌标记、球权变化取消、错误战术子表、重复首入写回、遗失回复／按键窗口／已接受指令、辅助物绑定以及 JSON 恢复差异。旧「非零请求一律拒绝」测试改为非法请求拒绝，未放宽到任意源表索引。

## 原证据与实现

- 完整原 `99F0` 调度和 `9AF6` 策略 4–7：2,456 个受控原调用，比较全部 `TeamAIState` 与两队 `HumanCommandState`。包括 42 个个人模式的接令标志、六套战术、球权变化、空中／受伤、辅助物可用性和纽约高台拒绝。
- REA：`ev_db06f3d08efb996a803730e9d3fa224c2a1a174b9f2107ebf6a2df3080773248`。原始证据、输入、源码摘要和初次失败日志位于 `.local/basketball/rea/team-commands/`。
- 模式 10／11／30：6,304 次原调用和 4 条 `28 → 10 → 1` 连续链，见 [传球指令](AI_COMMAND_PASS_VERIFICATION.md)。
- 模式 36／37／38／41：6,552 次原调用和 36 条连续链，见 [进攻指令](AI_COMMAND_ATTACK_VERIFICATION.md)。
- 模式 13／3／14：8,248 次原调用、16 次完整输入状态及 8 条耦合阶段链，见 [辅助物合作](AI_AUXILIARY_COOPERATION_VERIFICATION.md)。

输入状态仍只保存在 `WorldLoopMemory.Human`。准备阶段接收该状态的值并返回更新，世界循环在同一事务中提交；不在 `WorldPreparationState` 另存一份。新增 `Accepted` 保存原 `068A/068C` 写入，JSON 省略零值，旧存档仍以零值恢复。请求、忙碌标记、回复、计时器和按键窗口按原写入顺序更新。

## 正常输入验收

六套战术 × A/B 的 12 条路径从正常冷启动进入。只发送真实逻辑按键，未改球权、人物位置、种子或计时。全部观察到回复 1、相应原个人模式和最终请求／忙碌状态清零；战术 3 的 B 指令真实经过 `13 → 3 → 14`。每 30 视频帧执行 JSON 保存恢复对照。

另使用完整比赛门禁，对这 12 条指令路径分别运行两场，固定种子 `0x12345678`、正常上下半场、每 300 帧与中场恢复，48,000 帧上限。Native／Wasm 均各完成两次，共 48 场，12 种配置的轨迹摘要跨后端一致，源码在每次运行中保持不变。日志和来源绑定分别保存在 `full-matches-native-final.json`、`full-matches-wasm-final.json`，比较结果见 `full-match-comparison.json`。这些整场记录随后未因纯 HUD 字形布局修正而重跑；该表现修正不改变时钟或电脑状态，另用原上传字形、display／match 测试和最终浏览器验收验证。

浏览器通过 `accept_live.py --court 1 --commands` 执行六套战术的 12 条原生键盘路径，以及两条 320px 原生触屏路径，复用实际 Go/Wasm、客户端、Canvas 和 FC 控制器。记录位于 `.local/basketball/live-browser-commands/`。这些是开发验收页面的本地操作，不是双人网络整场、手机物理设备或原整机自然菜单轨迹证明。

## 可重复执行

```sh
source scripts/env.sh
python3 -B scripts/basketball/verify_team_commands.py
go test ./games/basketball/engine -run '^(TestOriginalTeamCommandPhases|TestRuntimeHumanCommandNormalButtons)$' -count=1
BASKETBALL_COMMAND_FULL_MATCH=1 go test ./games/basketball/engine -run '^TestRuntimeHumanCommandCompleteMatches$' -count=1 -v -timeout=15m
BASKETBALL_COMMAND_FULL_MATCH=1 GOOS=js GOARCH=wasm go test -exec="$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/basketball/engine -run '^TestRuntimeHumanCommandCompleteMatches$' -count=1 -v -timeout=15m
python3 -B scripts/basketball/accept_live.py --court 1 --commands
```

## 未完成范围

个人模式 15 的异常邻接表索引、投篮计划 11／12／13 的直接入口、其余避让／别名域、完整战役与装备、全部多人网络与素材发布验收仍未完成。原悬空球政策仍待用户选择，本轮未修正原缺陷或用自动结算绕过。战术 HUD 已按原版「6 格指令＋1 格空白＋2 格回应」修正。78 组原上传字形对照通过，最终浏览器验收通过 101 项，包含 14 条路径的可见完整字形及 PC／320px 截图。原 `TacticCache` 未擅自更新，源指令与原运行证据都没有显示写回，见 [HUD 记录](HUD_COMMAND_LAYOUT_VERIFICATION.md)。

统一结果索引：`.local/basketball/rea/team-commands/result.json`。旧 1,197 个全队阶段、1,997 个准备阶段、1,998 个输入阶段及单初态世界轨迹回归通过；篮球服务端测试通过，未运行其他游戏规则或完整对局矩阵。
