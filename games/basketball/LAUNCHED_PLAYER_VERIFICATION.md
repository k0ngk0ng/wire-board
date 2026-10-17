# 设备抛起人物动作验证

实现前核验：把动作 `$3E` 当成携带玩家；原随机方向错误读取新随机值而不是返回进位；附着来源丢失或多消耗随机数；物理前写入被局部旧值覆盖；起跳声 6 排到设备声 21 后；用正常跳跃动画和速度替代原抛物线；落地丢弃原请求的低 7 位；持球人被错误清空；装备、方向键及电脑计数的高度选择混用；继续帧重复发射或重置随机数。

原人物入口为 bank 6 `$9E3D`，其 `$3E` 分支调用固定页 `$E003` 和 `$FD91`，再执行动画、`$EEFF` 物理和落地请求。已有 REA 入口证据为 `ev_6c6d152fafac8c75465f555b8cd76b59422ebfbfacf59925a495fae560dd27fe`。新采样明确属于受控原指令执行，不宣称原版自然对局。

## 已恢复行为

`PrepareLaunchedPlayer` 对应 `$9E3D..$9E99`，`CompleteLaunchedPlayer` 对应后续动画、物理和落地分支。首入时先发声 6；未附着人物消耗一次原随机数，朝向取旧随机寄存器最低位形成的返回进位。附着人物保留随机数，使用来源对象的高度／朝向；两者均在物理前写入姿态源、解除附着并计算原抛物线。继续帧不重复发射。

目标位置沿用 `$E003` 的 X=68／460、纵深 47、高度分档 88／184／280；装备和原按钮／电脑计数按 `$E078` 覆盖高度。角色 0 的特定装备分支会消费当前持球人的特殊投篮标记，不能误清当前被抛起人物的标记。弧高使用原选中对象标记选择 24／48。辅助物所有者显示姿态 18 并跳过动画推进；其余使用原 `$AB70` 动画。落地写入请求的低 7 位，保留球权和请求状态。

World 接线由 `world_launched.go` 完成：先提交准备阶段的随机数、姿态源、轨迹及特殊投篮标记，再调用物理。当前事务内的物理写入不被旧局部状态覆盖，声音 6 在设备声音 21 前。Court1 的继续帧附着旁路沿用原整数坐标跟随；Court2 使用完整原物理绑定。

目标高度函数的 REA 证据为 `ev_a5a9880155a1395bfe2d2d59cef703b8cc7dc8bc3c3b4b18469993d1fc31fd5f`，保存在 `.local/basketball/rea/launched-player/analysis/`。本次只恢复 `$3E`，同入口的 `$3F` 分支仍未开放。

## 验证和限制

64 次受控原指令调用覆盖 Court1／Court2、首入／继续、随机两种朝向、附着、辅助物所有者、装备、真人上键、电脑计数、请求落地与持球人标记消费。每次分别比较原 `$9E99` 准备边界及最终世界字段、声音序列。源 RAM、ROM 和夹具摘要在 `launched-player-source.json`；没有把 Go 输出作为原版预期值。

定向 race／Wasm 验证通过，包含本次矩阵、World 提交时序、原 1,998／57 次连续更新、原完整动作阶段及球跟随偏移。结果在 `.local/basketball/rea/launched-player/race.json` 和 `wasm.log`。

两份保留的 Court2 真实 Go 故障快照经运行时协议重放后均通过原动作 `$3E` 的阻塞点，但在后续 video tick 3 遇到未恢复的 AI mode 31。`fixed-runtime-probes.json` 保留源 SHA、原时钟及错误；24 tick 请求没有完成，不能据此声称 Court2 完整比赛已通过。该后续 AI 分支已交接处理。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/extract_launched_player.py
source scripts/env.sh
go test -race ./games/basketball/engine -run 'Test(OriginalLaunchedPlayer|WorldLaunched|WorldLoop|WorldActionsEntireCapturedCorpus|HeldPoseOffsets)'
```
