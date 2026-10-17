# 队伍特殊投篮球 14／15

实现前排除：把动作 14 当普通投篮重新计算轨迹；首入冲量错误覆盖旧竖直速度／重力；落地后误转普通弹跳而跳过动作 12；动作 15 把旧朝向另存一份而丢失 `$0564` 原别名；用无符号计数判断替代 `$0511` 的有符号下溢；计时结束提前修改本次已经完成的位移；尾动画游标、姿态及翻转位不同；落地震屏／声音先后错误；释放球权和清除接球排除晚于场地物理；动作 12 使用已释放玩家的旧高度；碰撞请求忽略原接受条件。

原分派：bank 6 `$B4FC` 中动作 14 → `$BACE`、15 → `$BB09`、后继 12 → `$B65F`。场地物理、玩家接触和篮筐计分仍各属原独立阶段，不能在球轨迹函数里直接制造碰撞伤害或得分。

## 原行为与接口

动作 14 首入增加 `$EA4C` 的竖直速度／重力冲量，横向速度设为朝向对应的 ±2；落地后横速改为 ±8，穿越 `$BDD2` 的原边线后转动作 12。它不调用普通投篮的目标计算，也不直接计分。

动作 15 首入沿用原发射计划，把原朝向写入 `$0564`，强制当前翻转位，设置 `$0511=13` 并清除接球排除。计数在物理之后递减，下溢后停止速度、设置半像素重力并转尾动画；落地恢复 `$0564` 的朝向、震屏 11、发声 10，再转动作 12。`Ball.Deflection` 和 `Ball.BouncesRemaining` 继续承担这些原字段的别名，不另设重复存档状态。

动作 12 使用原普通投篮函数 `$B65F`，首帧从动画游标 0 开始。球已释放时，通过共享 `originalShotLaunchPlan` 使用球自身高度及原无主特殊标记，不能重新绑定旧玩家。共享 owner-6 路径由 `ball_device` 工作一并验证。

`PrepareOriginalTeamSpecialBall` 返回物理前命令，World 必须先提交球权、接球排除、特殊标记、选中篮筐、发射目标和准备音效，再调用 `CompleteOriginalTeamSpecialBall`。完成段清除已提交的准备命令，仅保留物理后新增的震屏和声音。准备和完成都使用同一 `OriginalShotBallStep` 元数据，没有第二份世界状态。

REA 确认记录：`ev_4e6f03893cd259169fd795ee3de8fa880241515e4686c8e5a392ea8c6bf39f0f`，保存在 `.local/basketball/rea/shot-team-special/analysis-verified/`。原动作 14／15 的完整函数体及 `$BDD2` 阈值分支同时保留指令证据。

## 验证范围

- 84 次受控原球函数调用，逐项比较准备边界、最终世界字段、震屏和声音。
- 4 条只初始化一次的原特殊球连续链，分别执行 90／75／83／83 步，覆盖左右朝向以及 14／15 → 12 → 普通弹跳的衔接；后续原状态只用于断言，不回填 Go 状态。
- 32 次原玩家接触响应，覆盖两种球、两侧、不同候选席位、普通／攻击姿态及 Court1／Court2。接触结果由既有原接触阶段计算，球函数不直接写伤害。
- 定向 race 和 Wasm 通过，并检查旧特殊球、原 1,998／57 次连续轨迹。组件最终验收为 `final-race.json` 和 `final-wasm.log`；完整 World 接线、原轨迹和既有球 4／9 回归记录为 `world-race.json`（PASS，11.244 秒）和 `world-wasm.log`。

夹具来源为受控原指令执行，使用原 ROM、保留的 RAM 基底和 MMC3 PRG 映射。APU 写入只记录寄存器值，不用其输出决定规则；声音资源另由原音频驱动采样。此验证不代表全场景自然比赛或浏览器视觉验收。

原位图素材已包含所需球姿态 0～3、6～10。声 30 已注册；缺少的声 10 已用原驱动采样两次并核验字节一致，输出长度约 0.839 秒，通过现有扩展音效清单注册到平台播放。素材仍仅在本地，未发布，人工试听标志保持未验收。来源及波形校验见 `.local/basketball/audio/export/additional-effects-manifest.json` 和 `additional-effects-verification.json`；`assets-audit.json` 只证明素材可用性，不冒充浏览器验收。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/extract_team_special_shots.py
source scripts/env.sh
go test -race ./games/basketball/engine -run '^TestOriginalTeamSpecialBall'
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/export_additional_effects.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_additional_effects.py
```

其他特殊球动作仍需分别恢复；本次没有以动作 14／15 的效果替代它们。World 已接入 12／14／15，并用物理加载前的权威字段、真实设备声 21 → 落地声 10、投影后的震屏 11 及连续原链验证时序；整局仍由主任务统一验收。

已确认一条尚未恢复的边界：场地设备在动作 14 的物理阶段把动作改成 4 后，原 `$BDD2` 按新动作查表，`$BE96` 指向 `$00FF`，后续读取依赖原音频工作字节 `$00FF` 和图形上传前缀 `$0100..$0102`（并非普通调用栈）。当前未用动作 14 的常量 32 替代该读取，而是明确返回未恢复错误，且整 World 调用原子回滚。固定原路径和替换选择器来源在 `changed-selector-before.log`、`replaced-selector.json`；不能将普通链通过扩展为所有设备碰撞通过。


该边界的后续源审计见 `.local/basketball/rea/special-turn-alias/audit.json` 和 `court2-counterexamples.json`。612,544 次原 `$BDD2` 指令调用核对了真实 X 范围、两方向和上传前缀约束；上传已完成时，右侧阈值为 `207 + 音频工作字节`，不能普遍化为常量。Court2 的原 music61／shot30 指令产生的不同工作字节，在合法设备 X=280／380 的受控调用中分别得到动作 12 或 4。该证据证明现有规则上下文不足，不等同于自然整局轨迹；保护错误继续保留，未引入任意 RAM 快照或运行时 APU。
