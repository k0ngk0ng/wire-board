# Court7 低台边缘悬空球调查

结论：当前失败与 Court1 的高台悬空球属于同类原程序缺陷。球所在点的实际支撑高度是 0，不是 24；原整机在同态受控起点之后也以同样的物理、AI 和输入进入卡局。未修正规则，未改变 seed、时钟或 48,000 帧上限，Court7 平台选择仍须保持关闭。

## 实际支撑面与 airborne 含义

保留的末端状态为 X=153.4765625、depth=79.8515625、height=24。原 Court7 地形表以整数 X／depth 的 8 像素格取样：整数 153／79 对应格值 0、支撑高度 0；相邻 depth=80 的格值才是 24。

对同一原 `$E149..E14C` 球分派执行 6 次探针：

| depth | 实际支撑高度 | 初始 airborne | 原执行结果 |
| --- | --- | --- | --- |
| 79.8515625 | 0 | false／true | airborne=true；动作 0；重力仍为 0 |
| 80 | 24 | false／true | airborne=false；转动作 6；深度速度 −0.5 |
| 80.8515625 | 24 | false／true | airborne=false；转动作 6；深度速度 −0.5 |

所以 airborne 不能解释为「绝对高度非零」。球确实在台面上且没有竖直运动时，原地形会清除该标记，并让静止球按原动作 6 离开高台。当前失败发生在台面外，缺少重力恢复。

## 最早触发过程

保留的原配置为 Court7、球队 0／1、球员 0／1、四个原版电脑、seed `0x12345678`。

- video 13980：球仍在 24 像素台面滚动，depth=80.828125。
- video 13986：球处于动作 5，depth=80.09375，横速变为 0，深度速度因原摩擦跨零为 −62／256。
- video 13989：动作 5 在物理前发现横速 0，转为动作 0；同次深度运动来到 79.8515625，离开台面。原物理设置 airborne，却仍保留高度 24、竖直速度 0、重力 0。
- video 13992：原动作 0 清除横向及深度速度，球停止。到 video 14025 已连续 12 个逻辑更新保持同态；随后直到 48,000 帧上限仍无结束。

此处与 Court1 的区别在于离台方向和入口：Court1 是最后一次弹跳转滚动后跨过 X 边缘；Court7 是滚动结束时，深度摩擦跨零产生的最后一次位移跨过 depth=80 边缘。二者共同缺口是松散球失去支撑后仍然零重力。

## 原指令及原整机对照

`probe_rest_stall.py --directory .local/basketball/rea/court-seven-stall` 比较触发前后 50 个真实 Go 球状态：分别独立执行原完整球分派，50／50 相同；只初始化一次连续执行原球分派，也是 50／50 相同。

进一步取 Go video 13980、仍在台面滚动的完整世界作为唯一受控起点。通过 `encode_world_probe.py` 将当前动作、球、篮筐、AI、输入、接触事件、位置、随机数、成绩和时钟等已建模语义字段写入原机，逐字段清单保留在 `controlled-patch.json`。原编码重新解码为 World，和起点语义完全一致。这个起点不是原菜单自然路径，也不是逐帧注入。

随后不再写任何游戏状态，原 CPU／准备／AI／输入／动作／物理和节末逻辑正常执行：

- 13,000 视频帧中，保存 3,864 个完整世界。
- 第 4 个后续世界起，球进入与 Go 相同的 X=153.4765625、depth=79.8515625、height=24、动作 0、无主、airborne、零速度及零重力状态；3,860 个世界持续悬空。
- 原机最终时钟为 0、第二节比分 23∶26，仍在比赛阶段 `0x81`。
- 只保留同一起点写入、移除后续观察代码的 reference，其全部 13,000 视频帧 RAM 和 5 张截图与 observed 完全一致。
- Go 从同一起点独立推进，3,864／3,864 个世界的规则语义字段与原机一致，包括人物、球、TeamAI 和输入。比较明确排除已被后续阶段复用的零页 contact scratch 及渲染投影，不宣称全 RAM 等价。

## 证据与复跑

目录 `.local/basketball/rea/court-seven-stall/` 包含首触发前态、50 帧历史、原球调用、支撑面探针、受控起点字段、原整机结果和 Go／原机世界对照。原始整机 RAM／逐世界 RAM／截图在 `.local/nes-basket/runs/court7-rest-stall-observed/`，无观察参考在 `court7-rest-stall-reference/`。

原世界轨迹 SHA-256：`f51e25a70d7e57ca868bf117e55c0f1083ab8906e5065b9cb9e89803d5ff65f0`。整机全视频 RAM SHA-256：`90a8ef0b3ea4c24f91c07e06a0fb70585da06e947fd14ca3312007eaf53f6dab`。

```sh
source scripts/env.sh
BASKETBALL_REST_STALL_OUTPUT="$PWD/.local/basketball/rea/court-seven-stall/first-entry-repeat" BASKETBALL_REST_STALL_SOURCE="$PWD/.local/basketball/rea/court-seven/runtime-all-computer-limit-before.json" go test ./games/basketball/engine -run '^TestBallRestStallDiagnostic$' -count=1 -v
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/probe_rest_stall.py --directory .local/basketball/rea/court-seven-stall
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_court_seven_stall_support.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/capture_court_seven_stall.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/compare_rest_stall_worlds.py --directory .local/basketball/rea/court-seven-stall --source .local/nes-basket/runs/court7-rest-stall-observed/world-after.ram --initial-updates 913 --go go-worlds.jsonl --output world-comparison.json
```

## 修复候选边界，尚未实施

若确认修复原版缺陷，判据应使用实际地形／设备支撑面，不能仅检查非零高度或直接清 airborne。无主、动作 0／5、竖直速度与重力同时为 0 且球高于真实支撑面时恢复正常下落；球确实停在高台上时，保留原清 airborne、转动作 6 的处理。回归须同时覆盖本例 depth 边缘、Court1 X 边缘和台面上实际受支撑的静止球，保留持球、挂筐、特殊动画的例外及全部原失败证据。是否启用这一修正仍由用户决定。
