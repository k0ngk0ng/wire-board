# 日版室外玩家常规步进

本文保留首轮室外及生命周期恢复范围。基地、基地 Boss 与瀑布玩家分支现已接入，新增 9,776 次原版验证见 [多场景玩家恢复](player-modes-recovery.md)；不能继续把下文首轮的场景缺口当作当前实现状态。

实现与证据绑定原版日版 ROM。`Player.StepOutdoor` 对应正常状态 `$D45E` 的一次完整调用，包含瞄准、边缘、射击、动作、地形、镜头约束、动画物理副作用、显示字段和越界死亡。`Player.Step` 进一步分派入场、正常、死亡和等待借命状态。它不执行 ROM，也不保存通用 CPU／RAM 状态。公共双人帧调度由世界帧模块处理。

测试先于实现：`player_test.go` 最初因缺少 Player API 失败；完整武器集成测试先因缺少原版前置弹丸池夹具失败，之后才补充自然原版采样。原版结果从未作为游戏实现输入或射击回调返回值。

当前通过范围：

| 验收 | 结果 |
| --- | --- |
| 不射击的完整常规调用 | 1,019 次，44 个玩家字段及同伴／世界字段全部一致 |
| 玩家与真实 WeaponSystem 集成 | 3,006 次，44 个玩家字段、同伴／世界和全部 16 槽弹丸池一致 |
| 重复原版采样 | v6/v7 使用同初态、同按键，全部常规调用及碰撞窗口一致 |
| 水中阶段 | 自然进入、稳定水中、Down 冻结、A 不跳、自然出水均有记录 |
| REA | 固定区 30 个步进／调度入口，另复用精灵 agent 的 `$A4F8/$A677` 证据 |
| 生命周期自然调用 | 4 次入场、297 次死亡推进、3 次命中、4,427 次借命检查，与原版一致 |
| 借命受控边界 | 102 个明确分阶段输入，10 次成功转移，全部字段一致；不冒充自然通关 |
| 世界帧集成 | `TestOriginalCombatFramePhase` 的 1,220 个完整原版 `$D296` 阶段通过，含自然入场、死亡与双人镜头；证据由世界模块维护 |

最新完整原版夹具为 [static-playerstep-complete-oracle.json.gz](evidence/static-playerstep-complete-oracle.json.gz)，SHA-256 `cfb545aa19f5e4b4e7a94cd963a1111eb4a766aee93fde097946ced3a0ade916`。游戏测试副本位于 `games/contra/engine/testdata/original-player-complete.json.gz`。较早的 42 字段及 44 字段记录保留了恢复过程，最新夹具增加了完整前置／后置弹丸池和出水场景。

## 完整顺序

原版 `$D45E` 调用 `$D495`，后者依次执行以下步骤：

1. `$D848` 按当前方向键及前帧朝向选择瞄准方向。
2. `$D8B0` 检查脚下；可能进入踏空或水中状态。
3. `$FA12` 切换到 bank 12/13 的 `$8001` 执行真实武器逻辑。射击使用更新后的瞄准方向和移动前锚点。
4. `$D518` 清理本帧 X 速度，执行 `$D595` 动作优先级，再经过 `$D525` 的侧面碰撞、边缘和镜头约束应用 X 位移。
5. `$D4A1` 应用关卡自动滚屏带来的玩家位置变化。

随后 `$D45E` 调用 `$FA4A`，切换 bank 4/5 到 `$A4F8`，执行角色动画与显示字段更新。这一步包含水中 Y、X 速度及计时器变化，属于确定性规则。最后保存前帧瞄准方向；若 Y 高字节为零且 Y ≥ `$E8`，进入死亡状态。死亡不会在这一步立即扣命。

## 地面、跳跃与踏空

地面输入按右、左、下、上依次检查，左右同时按时向右。空中方向代码先检查右，再由左覆盖；左右同时按时向左。空中释放左右不会清除方向代码，已形成的水平惯性会继续。

跳跃只消费 A 新按边沿。`$D5FD` 在已有跳跃标志时直接返回；开始跳跃仅初始化方向和速度、清部分动画计时，不移动 X/Y，也不重置 Y 小数累计。自然记录中 A+左右首帧保持 `(48,100)`，下一帧左右冲突开始向左移动。

下降时，整数 Y 速度为零的跳跃顶点阶段不立即检测落地。速度 ≥ 4 时检测；速度为 1–3 时还受 `(Y+scrollY)&15 < 8` 限制。脚底非空即吸附并落地。向上时检查 `(X,Y−10)` 的实体碰撞，撞头先清 Y 速度，再在同帧应用重力。

脚下实际查询点是 `(X,Y+16)`；Y 加 16 溢出则为空。落地的室外吸附保留 Y 小数与高字节，只更新像素字节：`base=(scrollY&15)|0xF0`，`Y=(((Y+base)&0xF0)-base+4)`，各步保持原版字节运算。

落地清跳跃／踏空／动作序列和 Y 速度；若此前在空中，另清动画帧、动画计时、踏空 X 锁定高度并产生落地音效。跳跃落地分支先按空中方向更新 X，再清跳跃，因此落地帧仍可水平移动。踏空落地分支没有该更新，本帧 X 在前面已被清零。

Down+A 的方向判断是 `held & 7 == 4`；只关心下、左、右，上不会改变这一判断。必须确认下方存在可落地的非空格且起始格不是实体，才设穿平台状态 `$81`。踏空初段锁定水平方向，直到 Y 达到 `min(255,fallStartY+20)` 后才接受新的空中方向。

## 地形与镜头

侧面探测使用移动方向的 X±8。站立时身体顶部偏移 11，趴下精灵 `$17` 为零；顶部出界或太靠上时采用 Y=10。扫描范围还取决于脚底状态及跳跃低位。对地板格会额外检查其下方实体格，不能只查询单个锚点。

室外正常 X 边界为右侧 `X>=230` 阻挡、左侧 `X<26` 阻挡，因此最左可以停在 25。Boss 完成自动滚屏后还使用按关卡选择的右边界；Boss 后的退出行走有独立放宽分支。

玩家引发水平滚屏的阈值是单活玩家 128、双活玩家 168。双人时，另一名玩家 X<33 会把推进者本帧 X 速度清零，保持镜头。自然 `camera-peer` 场景记录了 127 次这种阻挡。

发生玩家滚屏时，速度转移到世界 `FrameScroll`，本人 X 不再增加；随后公共双人帧还需校正未引发滚屏的另一名玩家。`$D356` 的横向校正只减 1，不按 `FrameScroll` 减；这是原版行为，交由世界帧调度保持。

## 水中物理必须在规则侧推进

首次入水动画使 Y 像素增加 16，计时设 16，并设置水中高位移动锁；同帧即减为 15。过渡期间 X 速度为零。稳定水中清移动锁，Down 禁止水平移动，A 不触发跳跃；方向与后坐力选择水中姿态。

稳定水中按角色当前锚点查询水；离开水格后设出水状态与 12 帧计时。出水过渡会给 X 速度字段写 6，但水中高位锁阻止其应用；计时结束清水中／动画帧并使 Y 减 16。不能把这个 6 当成一次实际冲刺。

保留日版 `$A6B6` 的字节语义：它加载精灵码 `$73` 后执行 `CMP #$08`，不是比较水动画计时，因此后续 `$18` 赋值不可达。本实现没有按注释推测去“修正”原版。

## API 与证据结构

`Player` 使用命名字段和已有 `PlayerMotion`，`OutdoorWorld` 保存共享镜头／关卡上下文。`PlayerStepContext.Fire` 必须绑定真实 `WeaponSystem.Fire`；即使不按 B，原版也可能更新机枪计时低半字节，因此没有伪造不射击回调。

`Player.Step` 对状态 0 执行入场、状态 1 执行常规操作、状态 2 执行死亡、状态 3 执行共享借命。`PeerInput` 提供同伴当前输入，`BorrowLife(*[2]Player, [2]PlayerInput, *OutdoorWorld)` 供公共帧末尾复用。`Hit()` 只让正常角色进入死亡，不立即扣命，重复命中不会把死亡状态再次推进。敌人方仍负责检查命中资格和无敌条件。

入场先重置角色属性，但保留原版没有清除的特殊动画计时和 X 速度增量。原版 `$DA0B` 也保留状态、武器、余命及 Game Over；只有 `$DA07` 额外清武器。起始位置不安全时，从 X=16 起按 16 像素扫描可落脚点，全部失败则使用原版 X=48 回退。入场给 128 tick 的出生无敌；帧外层随后按原版递减。

死亡先按朝向反向倒退，使用原版重力与地形。落到非水地面或 Y 高字节成为 1 后，进入 64 tick 等待；计时结束才清武器／属性，余命大于零时减一并进入重生，否则设置 Game Over。死亡动画每 8 tick 前进，最终停在第 5 格。

借命按第二名角色优先扫描 Game Over；找到该角色后，即使没有 A 新按也立即结束检查，不改去扫描另一名角色。供命者必须有余命；只有一条余命时必须处于正常状态，且始终不能处于室内推进。成功时供命者减一，接受者进入入场状态，接受者的额外余命仍为零。自然本轮没有出现成功借命，成功及拒绝边界由 [明确受控的原版调用](evidence/static-lifecycle-borrow-controlled.json.gz) 覆盖；原版调用返回后完整恢复 RAM，未用阶段输入冒充自然对局。

生命周期证据：[自然调用](evidence/static-lifecycle-natural.json.gz)、[完整 REA](evidence/static-lifecycle-rea.json.gz)。自然与受控两组都独立重复录制并比较进入／返回和碰撞窗口。

原版夹具的 `player_fields/world_fields` 给出数组字段顺序；每个 `steps` 条目保留 `before/after`、`beforePool/afterPool`、实际子调用进入／退出顺序和各子调用状态。游戏测试只使用 before 状态初始化 Player、同伴、世界、碰撞窗口和弹丸池，after 只用于比较。

源码入口：`games/contra/engine/player.go`、`player_animation.go`；差分与集成入口：`player_test.go`、`player_integration_test.go`。水中精灵表与原始语义依据见 [精灵恢复](sprites.md)；完整固定区证据见 [static-playerstep-rea.json.gz](evidence/static-playerstep-rea.json.gz)。

```sh
python3 -B scripts/contra/static_prepare.py
python3 -B scripts/contra/static_prepare_playerstep.py
node scripts/contra/static_rea.mjs .local/contra-static/playerstep-spec.json
node scripts/contra/static_rea.mjs .local/contra-static/lifecycle-spec.json
.local/board-art-env/bin/python -B scripts/contra/static_capture_playerstep.py --label <新的录制标签>
.local/board-art-env/bin/python -B scripts/contra/static_capture_lifecycle.py --label <新的自然录制标签>
.local/board-art-env/bin/python -B scripts/contra/static_capture_lifecycle.py --label <新的边界录制标签> --borrow-cases
source scripts/env.sh
go test ./games/contra/engine -run '^TestOriginalOutdoor' -count=1 -v
```

首轮通过证明第一关玩家阶段及对应世界帧的一致性，不证明完整对局。当前基地／纵向玩家实现见上方后续记录；所有实体撞头／侧面边界、自然成功借命、服务器恢复与网络时序仍按各自证据验收。
