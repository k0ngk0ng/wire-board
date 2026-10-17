# 原版投篮计划 2 与 7

## 实现前失败场景

- 固定冷配置第 1363 次更新选中计划 2，此前仅恢复 0／1／14／15。不得把计划 2 替换成普通投篮或换随机种子。
- 普通动作进入计划 2 时请求 CarrierJump、清 JumpPhase 为 0、方向设 None；它不同于既有计划 1。
- CarrierJump 的第二段起跳要求 JumpPhase 非负、腾空且速度高字节按无符号比较小于 2。负速度不能误当已满足条件。
- JumpPhase 为负时，仅速度高字节小于 3 转 AimAirShot；不重复加跳，不在落地标志上附加原代码没有的要求。
- AimAirShot 只在速度高字节为 0 时消费原 shotCycle 随机数、AirShoot 并进入 PossessionWait；保留原 nextPlan，不能照搬计划 1 写入 17。
- 所有原阶段保持请求、球所有权、物理坐标与非参与人物不变；只发原人物动作／JumpPhase／ShotCycle 命令。未通过原速度门槛时不得消耗 RNG。
- 先保留自然 B413 入口至 AI 返回的全部完整记录，再比较全 WorldInputState，不筛除不匹配的循环或出手阶段。

## 后续计划 7：实现前失败场景

计划 2 后冷轨迹到 update1592，选中计划 7（B4EE）。JumpGrade>=3 时按原跑步朝 zone60，抵达后 Mode18／nextPlan22；不是直接射门。JumpGrade<3 时调用原 B58E 低跳方案，再强制nextPlan22。跳跃等级由现有 JumpVelocity 与原一一对应表严格还原，非法／缺失值报错，不引入另一份持久等级。低跳落地仅在Entered时设置角色奇偶对应NextAction5A/5C；人类队友保持cycle0，纯电脑按原随机选择后反转普通/特殊cycle。物理与NextAction执行仍留给动作阶段。先加自然 B4EE 全状态对照、grade边界、目的地门槛和低跳命令测试。

## REA 与自然计划 2 对照

复用 `.local/basketball/rea/shot-planning/dispatch/batch.json`，REA 证据 `ev_8bac8418e07eaa8a98c2ac6472d25618bfc7cb159ee632963fe08b31581cafda`。该证据覆盖 B413（计划 2）、B4EE（计划 7）与 B58E（低跳分支），原分派表和完整 B383 证据为 `ev_1c67e123737322edbaaed8ac4bc6d1899caaa8f66b392f8d1fd10e8e5bec0d2b`。

计划 2 自然捕获来自 18,000 帧 `special-ball-cpu-command` 时间线，进入 B413 后到固定 FD8E 的全部 57 次完整调用均保留；全部 RAM／PNG 和参考相同。以完整 WorldInputState 比较后两次起跳、AimAirShot、ShotCycle、RNG 和未参与对象全部匹配。样本不是人为注入模式，也没有截短未通过的记录。

计划 7 在同一参考和额外的“传球后闲置”18,000 帧自然时间线均没有触及 B4EE，零命中记录保留；不能据此声称已自然验证计划 7。

## 隔离原函数验证计划 7

使用工作区已有 py65 执行原 bank5 B4EE 及其原辅助函数，不修改指令或替换辅助函数。通过测试输入构造 8,192 组：4 个席位、8 个跳跃等级、4 个区块、4 个动作、Entered、队伍是否有人类、角色奇偶和 2 个固定 RNG 种子。返回地址使用测试 sentinel 7000，单次最多 117 条原指令。比较动作／进入位、方向、全部 AI 控制字段、NextAction、JumpPhase、ShotCycle 和随机流，全部相同。

这是**隔离原函数执行**，不是自然对局或完整比赛。原程序只在测试工具内执行；生产代码仍是可维护 Go 分支，没有 ROM 解释器。来源声明和完整矩阵分别为 `testdata/ai-shot-plan7-function-source.json`、`ai-shot-plan7-function.json.gz`。

跳跃等级由现有 profile.JumpVelocity 严格匹配原一一对应表还原；AIShotContext 的 JumpVelocity／Character 仅是临时输入，没有新增永久 config 字段。低跳使用显式 NextAction 命令，统一 ApplyOriginalAIActorDecision 提交；不在 AI 层执行跳跃、投篮物理或评分。

## 结果与下一边界

所有新增测试均先于实现。原生竞态和 Wasm 验证通过：计划 2 的 57 次自然完整调用、计划 7 的 8,192 组隔离原函数执行、原 283 次投篮规划、单人物命令集成、1,998 输入／连续世界更新与 57 主动完整更新。日志位于 `.local/basketball/rea/shot-plan7/{native-tests.json,wasm-tests.log}`；初次失败测试位于各 `shot-plan2/shot-plan7/tests-before.log`。

固定冷配置从 1363 经 1592 推进至 1598／6000，下一边界是人物 1 动作103／variant93／frame0 的攻击碰撞资料；完整前态在 `shot-plan7/cold-next.json`，已交主线程核对来源。未为未知攻击 variant 添加默认几何，也未降低 6000 目标。

```sh
PYTHONDONTWRITEBYTECODE=1 .local/monopoly/venv/bin/python scripts/basketball/capture_ai_shot_plan2.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_shot_plan7_function.py
source scripts/env.sh
go test -race ./games/basketball/engine -run 'TestOriginalAIShotPlan|TestShotPlan(Two|Seven)|TestComputerActor|TestWorld(Inputs|Loop)' -count=1
GOOS=js GOARCH=wasm go test -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/basketball/engine -run 'TestOriginalAIShotPlan|TestShotPlan(Two|Seven)|TestComputerActor|TestWorld(Inputs|Loop)' -count=1
```
