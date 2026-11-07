# 日版敌人碰撞、拾取与游戏内得分

本轮恢复固定窗口中的 `$E10A` 玩家接触、`$E276` 玩家弹命中及其命中框、伤害、击毁、得分和增命路径。实现位于 `games/contra/engine/enemies_collision*.go`。使用日版原版 ROM、REA/Ghidra 和真实 FCEUX 调用结果；不执行 ROM 指令，也不修改平台合作积分流水。

## 接入顺序

`EnemySystem.CollidePlayers` 与 `CollideProjectiles` 对应原版已经准入的碰撞阶段。World 仍须按 `$E54F` 的顺序执行：槽位 15 到 0，先执行该敌人的状态机，再根据精灵是否存在、场景和 `StateWidth` 判定是否调用玩家接触与玩家弹命中。碰撞方法不重复外层的准入判定。

`EnemyCollisionContext` 持有当前 `Players`、`Inputs`、`Weapons`、`Scores`、`Level`、`SceneType`、`Demo` 和 `Flash`。方法只改变这些纯游戏状态及本 `EnemySystem` 的实体，调用现有 `Player.Hit()`、`Projectile.Hit()` 和落地逻辑。`EnemyCollisionResult` 返回声音、玩家受伤、拾取、原版得分、被安排进入击毁阶段的槽位，以及弹丸接触信息；`Flash` 由 World 回写。

`ArcadeScores` 使用原版二进制计数，每单位代表显示分数的 100 分。`NewArcadeScores()` 的初值是高分 200、双方下次增命阈值 200，即显示的 20,000 分。它与平台的 `+20/−10` 合作结算无关。

## 已保留的原版语义

- 玩家接触先检查第二位角色。普通受伤、屏障撞杀和拾取处理首个符合条件的角色后返回；可站立敌人的着陆会继续检查另一位角色。
- 水中按 Down 跳过接触，基地蹲下可避开敌弹。出生保护优先于屏障撞杀；道具拾取先于这两种无敌判断。
- 平台型敌人会根据角色相对高度调整命中框标志。着陆更新角色 Y、水平增速、`OnEnemy` 和落地状态；不以通用矩形覆盖这些条件。
- 玩家弹按槽位 15 到 0 检查，每次敌人碰撞函数至多处理一次命中。室外的 `SBC` 保留原进位带来的额外减 1；部分室内路径的进位不同。
- 伤害每次扣 1 HP。HP 为 0 或不小于 `$F0` 时不扣血，但命中的弹仍进入命中特效阶段。HP 归零只安排原版击毁 routine，保留敌人的后续死亡／爆炸时序。
- 激光命中后可能消耗该组中更早的飞行弹段，而不是当前几何接触的条目。保留原版按 `Routine` 搜索的顺序。
- 拾取相同武器保留 R，不同武器清除 R；R 道具只设置增速位。屏障持续值在第七关为 `$90`，其余关为 `$80`。
- Falcon 清屏跳过枪盒、飞行补给、无精灵／不活动实体、HP 为 0 或恰为 `$F0` 的实体。它安排击毁并设置属性高位，不直接清空全部槽位、不逐个发放击杀分。后续同帧敌人循环必须读取这个已更新的状态。
- 原版普通加分遵守演示模式抑制；特殊 500,000 分路径进入独立的原版入口。分数、增命阈值和生命数量按原版边界处理，不通过外部积分规则补算。

日版音效码来自原字节：拾取 `$38`，增命 `$39`，普通命中 `$2C`、心脏命中 `$2E`、核心装甲命中 `$2A`。这些值与公开美版反汇编不同；这里只输出事件码，播放交给平台音频层。

## 原版对照范围

先记录失败场景和原版输入／输出，再写差分测试并保留 red，之后实现 Go 规则。共验证 **8,198 次真实原版函数调用**：

| 来源 | 玩家接触 | 玩家弹命中 | 范围 |
| --- | ---: | ---: | --- |
| 自然 M 拾取路线 | 814 | 513 | 纯控制器输入，含实际枪盒掉落／拾取与游戏内加分 |
| 自然受伤路线 | 2,812 | 2,616 | 纯控制器输入，双方共 5 次实际受伤 |
| 明确受控的边界输入 | 966 | 401 | 姿态、形状、坐标边界、平台、水中、无敌、HP、激光、拾取与分数 |
| 专属敌人及特殊得分输入 | 0 | 76 | 71 个关卡／专属敌人类型组合的击毁 routine 选择，另含特殊得分边界 |

每条调用比较双方玩家的 44 个字段、全部 16 个敌人槽、16 个玩家弹槽、原版分数／高分／增命阈值、Flash 和声音。受控记录只在函数入口设置所声明的输入，记录原版真实返回值后立即恢复；没有将期望结果传给实现。它们验证函数边界，不是自然通过八关的证据。

四套录制均重复执行，调用记录与完整 RAM 字节一致。自然受伤路线的 RAM 还与独立敌人状态机录制逐字节相同。标准包内的定向测试通过；并行任务的其他规则或完整游戏验收不包含在此结论中。

## 证据与复查

- [验证记录](evidence/enemies-collision-verification.json)：red／green、输入与代码摘要、观测计数、复录结果和清理范围。
- [REA 索引](evidence/enemies-collision-index.json)与 [完整 REA 证据](evidence/enemies-collision-rea.json.gz)：17 个入口及必要的共享尾部，保留地址范围和分析限制。
- [表格来源](evidence/enemies-collision-tables.json)：命中框、变量形状、命中音效、得分及击毁 routine 的原字节范围和 SHA-256。
- `games/contra/engine/testdata/original-enemy-collision-*.json.gz`：原版自然与受控夹具，范围分别记录。

在仓库根目录复查 Go 差分：

```sh
source scripts/env.sh
go test ./games/contra/engine -run '^(TestOriginalEnemyCollisions|TestArcadeScoreInitialization)' -count=1 -v
```

重新采样时使用新的运行名称，脚本拒绝覆盖既有运行；已保存的 oracle 必须与新结果一致：

```sh
.local/board-art-env/bin/python -B scripts/contra/enemies_collision_capture.py --name collisions-natural-recheck
.local/board-art-env/bin/python -B scripts/contra/enemies_collision_capture.py --name collisions-hurt-recheck --route hurt
python3 -B scripts/contra/enemies_collision_cases.py
.local/board-art-env/bin/python -B scripts/contra/enemies_collision_capture.py --name collisions-controlled-recheck --cases .local/contra-enemies-collision/cases.json
python3 -B scripts/contra/enemies_collision_cases.py --special-only
.local/board-art-env/bin/python -B scripts/contra/enemies_collision_capture.py --name collisions-special-recheck --scope controlled-special --cases .local/contra-enemies-collision/cases-special.json
```

REA 和数据表可用 `enemies_collision_prepare.py`、`enemies_collision_rea.mjs` 和 `enemies_collision_tables.py` 复查。只使用工作区内输出与现有工具；不安装全局依赖。

本轮没有完成或声称完成敌人全部 AI、Boss 对局、八关自然通关、手机操作或网络验收。没有注册游戏、提交或发布。清理仅涉及本任务的私有 REA／Swift 临时缓存、重复录制、临时 profile 和已被完整证据替代的中间分析文件；原始 ROM、其他任务文件及仍在使用的主录制保留。
