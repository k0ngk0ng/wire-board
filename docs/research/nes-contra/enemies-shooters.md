# 日版敌方射击、枪兵与炮台

本轮已实现日版原版的敌弹创建、12／24 方向瞄准、枪兵、旋转炮台、红炮台、第一关 Boss 炮塔及跑兵开火阶段。输入绑定 ROM SHA-256 `6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe`。实现使用命名游戏状态、原版数据表及图形请求接口，没有 CPU 指令解释器或通用 RAM 读写层。

## 先测后写与验证范围

先列出方向与取整、攻击开关、满槽、出生时序、炮台状态、图形队列拒绝、姿态和别名目标等失败场景，再保存原版函数输入／输出和 red 测试，随后实现独立规则。[验证记录](evidence/enemies-shooters-verification.json)保留源码摘要、完整原版记录引用、red／green 与限定 race 结果。

| 验证来源 | 范围 | 结果 |
| --- | --- | --- |
| 原有自然轨迹 | 788 次枪兵分派 | 完整 16 槽一致 |
| 新的连续自然轨迹 | 枪兵 1,051 次、旋转炮台 806 次、红炮台 764 次，共 2,621 次 | 16 槽、声音、图形请求、真实队列字节及游标一致 |
| 受控原版几何／发射 | 2,786 个请求，各记录象限、绝对角度与发射结果 | 三项全部一致，含关闭攻击、满槽与速度上限 |
| 受控枪兵／跑兵 | 198 次状态调用 | 枪兵九阶段、目标状态及跑兵开火一致 |
| 受控炮台／Boss 炮塔 | 352 次状态调用，167 次实际图形请求 | 各阶段、拒绝与接受、声音及队列一致 |
| 受控跑兵进入开火阶段 | 48 次调用 | 计时、攻击开关、武器强度和随机位组合一致；实现由敌人主任务接入 |
| 检查 | 6 项限定 race 测试 | 全部通过 |

自然轨迹从正常双人初态开始，前 350 帧重放已验证的纯按键 M 武器拾取输入，后续使用可重复的控制器遍历输入。最终到达第一关屏幕索引 10，之后玩家耗尽生命，没有自然 Boss 战或通关证据。实际交给模拟器的 [输入记录](evidence/enemies-shooters-natural-inputs.csv.gz)已保留；该录制脚本不属于最终平台电脑策略验收。

自然轨迹、几何受控轨迹和炮台受控轨迹都重复执行过；原版完整 RAM、输入和对应调用记录逐字节一致。受控用例只在原版函数边界设置明确输入，原版例程完整运行并记录结果，然后恢复用于后续采样的状态；不把这些设置后的场景称作自然经过。

## 日版代码与数据

完整函数 Evidence 见 [REA 记录集](evidence/enemies-shooters-rea.json.gz)，不是只保留分析摘要。数据表由 `enemies_shooters_tables.py` 从本地日版字节导出，范围和 SHA-256 见 [数据表清单](evidence/enemies-shooters-tables.json)。美版反汇编只用于寻找候选，不决定日版地址或常量。

| 入口 | 日版作用 |
| --- | --- |
| `$F225` | 创建敌弹；按最高空槽分配，普通弹要求攻击开启，类型 `$20` 的 Boss 炮弹例外 |
| `$F29A`／`$F30B` | 按原版方向表、速度分支和整数取整生成有符号速度；创建入口将速度代码限制到 7 |
| `$F492`／`$F4C4` | 根据玩家有效状态、场景及原版象限表选择目标方向 |
| `$F3B3`／`$F3B7` | 12／24 方向的绝对角度与旋转方向 |
| `$8953` | 枪兵状态表基址减 2，包含九阶段 |
| `$8386` | 旋转炮台状态表基址减 2，包含十阶段 |
| `$84C5` | 红炮台状态表基址减 2，包含九阶段 |
| `$8B56` | 第一关 Boss 炮塔状态表基址减 2，包含六阶段 |

`$F30B` 调用跳转表分派器后紧跟数据，不能把 Ghidra 自动列出的表字节当成实际执行指令。本实现核对了各速度分支的真实入口及原版受控结果。原版速度使用逐次整数移位和加法，不能换成浮点倍率再统一四舍五入。

枪兵受击共用 `$88DA` 的冲量写入为 `$FC80`，有符号 8.8 值是 −896，即 −3.5 像素／tick。参考注释中的 −4.5 不准确，受控原版调用也返回 −896。

## 图形更新与红炮台目标别名

炮台通过 `EnemyStepContext.ChangeTerrain` 接入真实 `GraphicsQueue`。当前游标 `$21 ≥ $40` 才拒绝请求，不能按预计追加后的长度提前拒绝。`$EB10` 的拒绝另把动画计时设为 1；`$EB1B` 的坐标减 12 借位路径成功返回但没有有效图块地址。状态测试直接执行独立图形队列并比较原版字节，没有把 `accepted=true` 写死。

红炮台还保留一个实际影响开火的原版别名行为：`$F3B3` 先令 `$0F=0`；如果本帧转向并成功提交图块，图形请求会将它改为替换调色属性表的高字节。第一关该值为 `$86`。随后 `$85B7/$85B9` 用此值读取目标坐标作开火范围判断。

本轮原版调用实际观察到索引 0、1、134。`$86` 对应的命名投影为：

- X：`WeaponSystem.Projectiles[2].Y`，原位置 `$03BA`。
- Y：`WeaponSystem.Projectiles[8].YFraction`，原位置 `$03A0`。

`JungleShooterTarget` 只实现已确认的第一关投影；未知索引返回错误，不能默认改成第一位玩家、最近玩家或零坐标。原版受控用例明确给这两个字段设置非零值，验证接受／拒绝图块时的差异。通用图形请求的 `$80` 无调色更新结果已有独立证据，但本轮没有观察红炮台消费它，因此没有泛化未知投影。

Boss 炮塔 `$8BA5/$8BA8` 也使用普通超图块请求。属性为 1 时，请求期间临时把 X 减 8，随后恢复；请求的 Carry 决定是否继续，不能无条件推进炮击或死亡。它不直接决定平台胜负或整场通关。

## 接口与复查

规则位于 `games/contra/engine/enemies_shooters*.go`：

- `CreateEnemyShot`、`EnemyShotVelocity`、`EnemyAimQuadrant`、`EnemyAimAngle`。
- `StepSniper`、`StepTurret`、`StepRedTurret`、`StepBossCannon`、`StepRunnerFire`。
- `JungleShooterTarget` 提供已确认的命名投影。

共同爆炸调用敌人层的 `StepExplosion`；音效代码追加到 `EnemySystem.PendingSounds`，由世界阶段统一消费。碰撞、规则内得分、玩家死亡和平台积分由相应阶段处理，本层没有越过这些接口直接写胜负。

复查当前实现：

```sh
source scripts/env.sh
go test -race ./games/contra/engine -run '^(TestOriginalEnemyShotAndAimBoundaries|TestOriginalSniperSteps|TestOriginalControlledShooterStates|TestOriginalTurretStatesWithGraphicsQueue|TestContinuousNaturalShooterStates|TestOriginalRunnerFireTransition)$' -count=1 -v
```

重录时使用新的运行名称，脚本会拒绝覆盖已有模拟器输出，并检查与保留夹具的结果是否一致：

```sh
.local/board-art-env/bin/python -B scripts/contra/enemies_shooters_capture.py recheck
.local/board-art-env/bin/python -B scripts/contra/enemies_shooters_capture_states.py recheck
.local/board-art-env/bin/python -B scripts/contra/enemies_shooters_capture_turrets.py recheck
.local/board-art-env/bin/python -B scripts/contra/enemies_shooters_capture_natural.py shooters-natural-recheck
```

需要先具备由 `capture_bootstrap.py` 生成的正常双人初态，以及已经保留的自然 M 拾取时间线。研究工具与临时输出限当前工作区。后续仍需连续独立世界、自然 Boss 战、其余关卡以及正式电脑和联机验收；本轮数据不代替这些要求。

## Boss 门、警灯与通道延续

随后完成第一关 Type 17 的七阶段门体。102 个原版受控调用逐项核对了完整实体池、进度字段、音频 reset／play 顺序、图形队列和碰撞窗口；其中 16 次真实通道碰撞写入单独验证。再以一次明确初始化为起点，让原版自行执行完整门体延续：击破至移除 115 次调用；存活武器箱造成真实队列竞争的通道延续 65 次调用，其中一次拒绝后成功重试。

连续验证保留独立 Go 门体和碰撞状态，不在每次调用前用原版结果重置它们；其他实体及当前帧图形工作由对应原版边界提供。两条轨迹与独立原版重复录制字节一致。合并射击规则与门体后，9 项限定 race 检查全部通过，记录见 [Boss 门验证](evidence/enemies-shooters-bossdoor-verification.json)及 [完整函数证据](evidence/enemies-shooters-bossdoor-rea.json.gz)。这仍是受控起点的延续验证，不是自然 Boss 战或整局通关。

日版步骤为：

1. `$8BE6` 播放警笛 `$33` 并进入等待，不能采用美版注释的 `$1B`。
2. 受毁后 `$E671` 先调用 `$F9D4` 重置当前游戏音频，再播放 `$77`；设置 `BossDefeated=1`、`DelayLow=$FF`，保留原 `DelayHigh`。
3. `$E949` 将仍可毁的敌人转入各自销毁阶段并设置属性 bit 7；跳过未激活、无精灵、武器箱、飞行补给、HP 0 和 HP `$F0`。它不直接清空全部槽，也不发放击杀分。
4. 门体完成共同爆炸后隐藏精灵，按 8 tick 的节奏依次画出八块通道。只有图形请求接受后，才修改对应碰撞并推进通道索引。模式 0 清空四格；模式 4 清空上排、将下排两格设为地板。
5. 每块通道尝试创建一个原版 Type 2／Routine 6 爆炸效果实体；槽满时仅跳过该效果，通道仍推进。最终设置 `DelayLow=$30`、`DelayHigh=0` 并移除门体。

`StepBossDoor` 只返回 `BossDoorNoSignal`、`BossDoorDefeated` 或 `BossDoorTunnelFinished`。敌人总循环即时消费信号，将进度写到已有 `OutdoorWorld`／`Campaign`；它不创建重复 Boss 状态，不直接判定整场胜利。发生击破 reset 时，本方法清理此前敌人声音，总循环还须清理此前槽位及玩家阶段的本 tick 声音，再播放 `$77` 和后续音效。

音频 reset 的真实跨 bank 目标是物理 bank 2/3 的 `$8719`，其 Evidence `ev_e08468d95ffdf4089bc624a0f88f5e2d3dfb5b3d5c14a12676026f0450989006` 已连同正确音频 profile 保存。敌人窗口的静态分析不能替代这个跨 bank 语义。

门体复查命令：

```sh
source scripts/env.sh
go test -race ./games/contra/engine -run '^(TestOriginalBossDoorStates|TestOriginalBossDoorCollisionPatterns|TestContinuousOriginalBossDoor)$' -count=1 -v
.local/board-art-env/bin/python -B scripts/contra/enemies_shooters_capture_bossdoor.py recheck
.local/board-art-env/bin/python -B scripts/contra/enemies_shooters_capture_bossdoor_sequence.py recheck
```
