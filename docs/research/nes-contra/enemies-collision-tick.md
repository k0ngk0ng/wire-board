# 日版完整敌人阶段调度

`EnemySystem.Tick` 已按原版固定入口 `$E54F` 接起敌人状态机、玩家接触、玩家弹命中、图形请求和声音。入口目前只接受已恢复的第一关室外场景；其他关卡明确返回未恢复错误。

## 调度与接入契约

World 提供当前事务副本中的 `EnemyStepContext` 和 `EnemyCollisionContext`。Tick 清除双方 `OnEnemy`，随后从槽位 15 到 0 读取当前有效槽位。它不会预先复制待执行实体列表：高槽位创建、销毁或改变低槽位后，低槽位在同一次循环中看到新状态。

每个实体执行前，目标位置、`State` 和 `GameOver` 都从当前玩家状态重新投影；若有 `SharedRandom`，则读取其最新 `Value`。`Frame`、`AttackEnabled` 等本帧已确定的输入不重算。返回前再次刷新目标，供后续随机跑兵生成阶段使用。

原版 `$06` 还作为后续生成器的 `SpawnVariantSeed`。Tick 不在入口归零这个值，只清掉上次的变更通知；实际生产者按执行顺序发布新值，Tick 消费后回写 context。整阶段观察确认了方向计算的 `$F3DD`（回调报告 PC `$F3DF`，写 12／24）、子弹生成的原始／限幅速度写入，以及碰撞更新循环最终的 `$FF`。没有生产者时保留上游传入值。它是原版跨阶段工作值，不是替代随机数发生器。

状态机执行后按原版规则检查当前精灵及 `StateWidth`。玩家接触后只重新读取 `StateWidth` 的高位决定是否检查玩家弹，不额外增加一次精灵存在判定。红炮台的历史别名由 `JungleShooterTarget` 读取真实 `WeaponSystem` 语义字段，没有提供通用 RAM 读取器。

图形回调必须绑定真实 `GraphicsQueue.RequestSupertile`、当前超图块映射和 `JungleGraphicsTileset`。队列拒绝、属性查找结果与碰撞地址会影响敌人的后续状态，不能把「请求成功」写死。Tick 本身不重置图形队列，也不调用 `$FA22` 的固定出生或 `$FA32` 的随机生成；它们仍由 World 按原版阶段顺序调用。

`PendingSounds` 是本次阶段的临时请求。Tick 在入口清掉旧请求，每个实体状态机后立即取出它的新声音，再拼接该槽的玩家接触、玩家弹命中声音；返回时队列为空。声音不能积到末尾再整体拼接。

## Boss 信号和声音重置

`StepBossDoor` 返回信号后，Tick 立即调用 `OnBossDoorSignal` 更新唯一的 World／Campaign 状态。已验证的映射是：

| 信号 | 原版状态变化 |
| --- | --- |
| `BossDoorDefeated` | `$3B=1`、`DelayLow($2A)=$FF`，保留 `DelayHigh($2B)` |
| `BossDoorTunnelFinished` | `DelayLow=$30`、`DelayHigh=0`，不在这里跳关或结算平台胜利 |

Boss 击破路径中的 `$F9D4` 会停止之前的音频。Tick 对应清除此前已汇总的声音，并设置 `EnemyCollisionResult.AudioReset=true`，再接入重置后的 `$77` 等请求。World 收到此标记时，还须清掉更早的玩家阶段声音。敌人 HP、击杀分和玩家受伤状态不会因音频重置而回滚。

回调失败时返回错误，由 World 丢弃整个事务副本；Tick 不保存另一份 Boss 进度标记。后续实体读取实时槽位和共享状态。

## 完整阶段差分

先建立原版完整入口／返回夹具和 red 测试，再实现调度。通过 **5,396 次完整 `$E54F` 调用**，包含 **15,635 次实体状态调用、120 次真实图形请求**。

| 来源 | E54F 调用 | 验证范围 |
| --- | ---: | --- |
| 自然 M 拾取路线 | 700 | 拾取、射击、箱体和图形请求 |
| 自然普通枪路线 | 1,200 | 双人受伤、跳跃、桥体、敌弹及共享状态 |
| 自然遍历输入复放 | 3,480 | 复放已记录的 5,000 视频帧输入，覆盖至第 10 屏、旋转与红炮台；后续不再执行 E54F 的帧不补造调用 |
| 明确受控的排序输入 | 11 | 双向声序、即时目标更新、Falcon、低槽新生实体、队列边界及真实弹池别名 |
| 明确受控的 Boss 输入 | 5 | 音频重置、回调、高字节保留、隧道请求／拒绝与连续两次重置 |

每条记录从对应原版入口态运行一次独立 Go 阶段，比较双方玩家 44 字段、全部敌人／玩家弹槽、原版游戏分数和阈值、Flash、超图块、碰撞窗口、完整图形缓冲及声音顺序。这不是从一个 Go 初态连续运行完整关卡的验收，也不代替 World、联机或八关对局测试。

当前用于验证的 `*-bias.json.gz` 版本还记录 `$06` 的入口／出口及每次写入的 PC、bank 标记、槽位和值。自然遍历路径中，仅瞄准而没有发射的炮台也会写入方向整圈常数；若只跟踪真正发射的速度，会丢失这些变化。该缺口先由 red 暴露，再由实际生产者标记修复；没有从夹具的返回值给运行代码补数。

受控声序由原版实际产生：普通次序分别为 `2F → 2C` 和 `2C → 2F`；Boss 重置案例丢弃先前 `2F`，保留 `77 → 2F`；同一阶段两次重置后只保留最后的 `77`。这些记录与自然输入分开保存。

完整阶段验证还发现并推动修复了飞行补给的第 3 状态：日版 `$8385` 实际跳转 `$82D4` 转换为道具，并非空返回。修复先补真实单实体 red 夹具，再由本阶段验证集成结果。

## 复查与产物

- [阶段验证记录](evidence/enemies-collision-tick-verification.json)：输入和代码摘要、red／green、重复录制及清理范围。
- `games/contra/engine/testdata/original-enemy-collision-tick-*.json.gz`：自然和受控完整阶段记录。
- [碰撞规则与得分说明](enemies-collision.md)：两个碰撞子阶段的独立恢复依据。

从仓库根目录复查：

```sh
source scripts/env.sh
go test ./games/contra/engine -run '^(TestOriginalWholeEnemyPhase|TestEnemyTickClears|TestOriginalEnemyCollisions|TestArcadeScoreInitialization)' -count=1 -v
```

重新录制时使用新的名称，不覆盖旧运行：

```sh
.local/board-art-env/bin/python -B scripts/contra/enemies_collision_tick_capture.py --name phase-machine-recheck --route machine --bias
.local/board-art-env/bin/python -B scripts/contra/enemies_collision_tick_capture.py --name phase-ordinary-recheck --route ordinary --bias
.local/board-art-env/bin/python -B scripts/contra/enemies_collision_tick_capture.py --name phase-traverse-recheck --route traverse --bias
python3 -B scripts/contra/enemies_collision_tick_cases.py --bias
.local/board-art-env/bin/python -B scripts/contra/enemies_collision_tick_capture.py --name phase-order-recheck --route controlled --bias --cases .local/contra-enemies-collision/tick-cases-bias.json
python3 -B scripts/contra/enemies_collision_tick_cases.py --boss --bias
.local/board-art-env/bin/python -B scripts/contra/enemies_collision_tick_capture.py --name phase-boss-recheck --route boss --bias --cases .local/contra-enemies-collision/tick-boss-cases-bias.json
```

全部录制已重复核对完整调用记录和 RAM。仅清理本任务的重复运行、临时 profile 和中间日志，保留主录制及自包含夹具；未删除用户 ROM 或其他任务的工作文件。
