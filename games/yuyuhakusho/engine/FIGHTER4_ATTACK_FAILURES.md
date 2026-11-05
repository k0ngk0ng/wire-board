# 来源 4「飞影」攻击选择：实现前失败场景

本清单先于本轮生产及测试建立，基线为 `605efb55`。范围为四个选择器 `16d86/16ec8/16f3e/16fcc`、九个特殊构造及 motion 调用的 `19092/191ba`。复用既有直接 Ghidra 清单；REA raw 68000 不支持，不冒称 REA 反编译。

最小状态为既有 `ChuAttackState`。选择 API 为 `SelectFighter4Attack`；后续无成本构造分别由 `FinishFighter4Rush` 与 `BeginFighter4DragonFollowup` 表达，不新增 canonical 字段。MovementSource 的 motion 写入与后续参数消费不属于选择器。

| 入口／根 | 失败场景 |
| --- | --- |
| 地面普通 `16d86` | Request8 优先，Action12/Pose0c/VX38000；Request7且Pose≠11才Action0e/Pose0b/VX3c000；方向16项、B优先、无键、横向A投技、全部Facing byte类及完整FX分配副作用 |
| 空中普通 `16ec8` | 方向表与B优先；不额外投技；保留未写存储与Extended输入 |
| `1904e` | R1，无Changed门槛，成本8→25/a0；VX按Facing.bit0给全long ±28000，不用Facing任意非零判断 |
| `190a8` | R2，无Changed门槛，成本32→03/a2 |
| `190d2/190f0` | Ground R3，A优先于B；A24→27/a5，B16→26/a4；不能沿用其他来源的B优先 |
| `1911a/1913c` | Ground R4，A优先于B，均24→2c/a8；VY=-68000，Gravity A=3200/B=5000，VX按Facing.bit0为±18000 |
| `19190` | Ground R6，signed Health≤111且主child.Size=0；Changed B或A任一，200→29/a6；child=1/100/ffff必须阻断；成本失败不清Request |
| `191d4/191ec` | Air仅R3，A优先；A24→28/ad，B16→28/ac；不套Ground R3的Action |
| `19092` | 无成本后续03/a1/Stages0/Reset，保留Request、Energy和未写字段 |
| `191ba` | 无成本后续2a/a7/Request0/Stages0/Reset；不擅写MovementSource或GravityParameter |

- 各有成本根覆盖 signed Energy -32768/-1/0/cost−1/cost/cost+1/32767、fraction 0/1/ffff；Health -32768/110/111/112/32767；Facing 0/1/2/3/128/255；全部Changed byte与A+B/C组合。
- 投技覆盖四横向方向、坐标word回绕、游标所有16槽、完整512字节；Y−1c、W16/H1c、Source b8/action3、Target40/action3、TargetFacing1、OffsetX18。无效存储须错误且原子返回，不改输入slice。
- FX覆盖空池、当前槽可用、后槽可用、满池、高位游标与回绕；不屏蔽全16槽或游标变化。只在实际访问的分支校验存储，无关的无效Pool/Throws保持原样。
- 四个对象地址执行完整入口至实际调用者返回，exit_sp=entry_sp+4；直接后续构造也用原CPU完整返回，不在中间constructor截断。
- 对比完整actor、三个child、其他world、FX、throw、两个游标、位置小数、Energy fraction、共享RNG。私有合成上下文不证明自然游玩或角色资源绑定。
- 单初态流只变声明输入，累积状态，不注入每步预期中间值或RNG。恢复显式使用restore_refresh_cycles=True。
- ordinary/special各唯一未观察原core3602，3600入口+2排空；hook观察逐帧RAM/video相同。先CPU夹具、冻结测试与实际缺符号red，再新增生产；不改旧Go/bundle/PROGRESS，不提交或开新工作者。
