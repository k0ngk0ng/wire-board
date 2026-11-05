# 树（来源 7）selector／构造恢复失败清单

本清单先于 `fighter7_attacks.go` 和专属测试创建。范围仅含四个 selector 的完整返回及所达构造；命令识别、动作继续执行、子对象、碰撞、公共尾段和整局验收由其他范围承担。

## 证据与顺序

1. 以 SHA-256 `c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06` 的原 ROM 为唯一目标。现有 `fighter-action-flow.json.gz` 的 source7 selector 145 条、共享普通构造 36 条、树构造 101 条、共享重置／扣费尾 23 条已逐条校验 ROM 字节；静态结论不等于执行观察。
2. 使用主任务唯一 original-core 3602 帧 control 的输入、参考帧和 checkpoint。不得另外生成一份 original control。hook-core 重放逐帧必须与该 control 一致。
3. 保留自然完整 selector 返回、声明的脏内存调用矩阵、单初态持续调用。私有调用先运行原 source7 initializer 校验五项资源绑定；每次 restore 必须恢复 refresh_cycles。
4. 采集完成后冻结原 CPU 夹具和专属 Go 测试，以不存在的本范围 API 运行真实 red，再编写实现；不得改冻结的断言来获得 green。
5. 返回点须核对 SP=entrySP+4、A5 仍为同一对象、输入与输出全对象字节、三个子槽、能量小数、RNG、效果池、投掷缓冲。每次同时比较 CPU 64 KiB：所有改动按确切所有者解释，IRQ／外部音频消费者单独记录，不以未知大区掩码通过。

## 必须暴露的失败场景

| 类别 | 失败场景与应保留的边界 |
| --- | --- |
| 入口 | source7 身份错误、返回不完整、SP 不平衡、对象地址串用；四个 fighter 地址均覆盖 |
| 方向表 | 地面 16 种低四位方向；空中 A 的 `0/4/8→95` 与其他 `→94`；Held 高四位及扩展键不得扰乱方向索引 |
| 按钮优先级 | 普通 B 高于 A；R1 B 高于 A；R2／R3 C 高于 B 高于 A；无可用按钮必须不改状态 |
| 地面前后冲 | R8 优先且无 Pose 门槛，R7 仅 Pose!=0b；各自 VX `38000/40000`、MAIN `11/0f`；Facing 0/1/3/80 的烟雾方向；完整池、当前槽可用、下槽可用、空池、游标环回 |
| 普通攻击 | 下蹲 B/A 的 `84/86`，水平 B/A 的 `81/83`，下斜 A 的 `13/87`；Pressed、Changed、Held 三通道不得混用 |
| 投掷 | 唯一地面投掷条目，W18/H24/Y-24/OffsetX10，完整保留未写 byte；偏移非32对齐、容量不足应原子失败，不能污染调用者 slice |
| R1 | B/A 均扣16；Gravity3800、VY -20000/-30000、VX40000/60000；VX 只按 Facing bit0 镜像；Aux48 仅最高 byte 清0、低24保留；Request 清0 |
| R2 | 主 child.Size 非0时完全无变化；C/B/A 均扣16；VX 仅高word写30/70/b0，低word脏值保留；其他子槽不作门槛；MAIN3b/Posea6 |
| R3 | 地空 C/B/A 均扣16；Aux48 仅最高byte写2/1/0，低24保留；VX／Gravity 不改；MAIN3a/Posea4 |
| 弱生命 | R4/R6 共用；Health signed<=111、Held.Down、扣200；Health -32768/-1/110/111/112/32767，Changed=0仍可成功；无Down应不改；R4无玩家命令绑定，不伪称自然达到 |
| 空中请求 | R1 原样 RTS；R2 B/A 成本16/20，MAIN45/Poseae/af，VX40000/80000、VY40000/30000，Facing 从 Held 的 Right 优先于 Left；R4/6无根 |
| 能量 | 有符号高word -32768/-1/0/cost-1/cost/cost+1/32767；低word 0/1/ffff 均原样保留；拒绝时完整状态不变 |
| 重置 | 构造重置 Clock／Aux64／ActionFlags／Aux77，并 Team=BaseTeam；不扩大到其他脏 byte，不重置所有 Request（普通攻击与冲刺保留原请求） |
| 持续流 | 单初态后仅改变声明的 Held／Pressed／Changed，不回灌中间 actor／池／RNG／能量；重复扣费、请求清零、投掷追加、池环回均按原 CPU 输出推进 |
| 存储错误 | 无效 mode、无效池游标、投掷索引错误返回输入原状态；空中无需投掷／池时不读取不可用存储 |
| 证据范围 | 自然无调用必须明确为0；私有成功不能冒充自然可玩；未覆盖根不得抬高恢复比例 |

## 最小字段方案

`Fighter7AttackState` 嵌入已有 `ChuAttackState`，仅增加 `AlternateVelocity int32`，作为本次调用中 canonical `WorldObject.Aux48` 的临时视图。`Body.VelocityX`／`VerticalVelocity`／`Gravity` 各自保留唯一 full long。拒绝新增 variant、重复计数或第二份 FE 所有者。

## 当前状态

静态核对和失败清单已完成。等待主任务提供唯一 control 的输入／帧／checkpoint 后运行采集。此时尚无 source7 实现或通过声明。
