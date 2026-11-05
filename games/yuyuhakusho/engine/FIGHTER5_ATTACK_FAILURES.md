# 来源 5「仙水」攻击选择：实现前失败场景

本清单先于本轮测试与生产代码建立，基线为 `b832d149`。范围为四个选择器 `1705c/1719e/17214/172c4`、`19216..19473` 中可达构造，以及这些构造的完整共享返回。静态依据复用直接 Ghidra 清单；REA 不支持 raw 68000，不将静态推断记为执行观察。

最小状态拟为 `Fighter5AttackState{ChuAttackState; AlternateVelocity int32}`。`AlternateVelocity` 是 `+48` full long 的唯一存储，`Body.Gravity` 已是 `+4c` full long。高 word、最高 byte 仅为临时视图，不增加重复状态。API 为 `SelectFighter5Attack`、`FinishFighter5Combination`、`BeginFighter5FinalProjectile`；后两者只表达原构造，不承担后续 motion。

| 入口／根 | 失败场景 |
| --- | --- |
| 地面普通 `1705c` | Request 8 优先，`11/0c`、VX=`38000`；Request 7 且 Pose≠11 才 `0f/0b`、VX=`40000`；方向 16 项、B 优先、无键、横向 A 投技、全部 Facing byte 类与完整 FX 分配副作用 |
| 地面投技 `1710c` | 横向 4/5/8/9，Y 高 word−`28` 回绕，W=`18`/H=`28`、Source=`b8`/Action3、Target=`40`/Action3、TargetFacing1、OffsetX=`18`；不改 VX，不因 Facing 调整零 X 偏移 |
| 空中普通 `1719e` | B→`90`；A 按方向 0/2/4/6/8/10→`95`，其余→`94`；B 优先，无额外空中投技，Stages=`10` |
| `19216` | Ground R3，Changed B 或 A 任一；成本 8→`2d/a8`；C 单键不触发 |
| `19240/19284` | Ground R1，无 Changed 门槛；Held.bit1 选择 `19284`，其余 `19240`；成本 16→`2e/a6` 或 `2e/a4`；失败保留 Request 和全部脏存储 |
| `192c8/192f6/19324` | Ground R4，主 child.Size=0；Changed A 优先于 B，B 优先于 C；成本 4→Action=`2f`，Pose=`a0/a1/a2`、VX/VY=`0/48000`、`28000/0`、`10000/10000`；`+4c` 仅写最高 byte 0/1/2 |
| `19364` | Air 仅 R4；主 child.Size=0，signed wrap(Ground.high−128)≤Y.high；Changed A/B/C 任一；成本 4→`30/ac`、VX/VY=`10000/10000`，`+4c` 仅写最高 byte 3 |
| `193a4/193bc/193d4` | Ground R6，signed Health≤111、主 child.Size=0；Changed A>B>C；成本 50→`31/a3`，`+4c` 仅写最高 byte 0/1/2，`CLR.W +48` 只清高 word，保留低 word |
| `1926a/192ae` | 无成本组合后续→`03/a5` 或 `03/a7`，清 Request/Stages 并 Reset；不改 Energy、Facing、速度、Aux48、Gravity |
| `19408/19426/19444` | 无成本最终子弹→Action=`32`；按 variant 0/1/2 写与 Ground R4 相同的 VX/VY/Pose/Gravity 最高 byte；清 Request/Stages 并 Reset；不扣 Energy，不清 Aux48 |

- 有成本根覆盖 signed Energy `-32768/-1/0/cost−1/cost/cost+1/32767`、fraction `0/1/ffff`；R6 的 Health 覆盖 `-32768/110/111/112/32767`；主 child.Size 覆盖 `0/1/100/ffff`。
- 全部输入 byte 独立扫描；Changed 的 A+B、A+C、B+C、A+B+C 必须保持原优先级。Extended 输入、RawHeld、RetainTicks 不参与错误分支。
- `+48/+4c` 使用非零脏低 word 与低 24 位，覆盖 0、全 1、正负边界；禁止把 byte 写成 full long 或把 `CLR.W` 写成 full long 清零。
- 空中高度覆盖 Ground 高 word 的 `-32768/-1/0/127/128/32767` 与阈值上下各一；保留位置小数。
- 投技覆盖全部 16 个游标槽和完整 512 字节；坐标高 word 回绕。无效 buffer 应错误且原子返回，不改输入 slice；未访问分支不检查无关 buffer。
- FX 覆盖空池、当前槽可用、后槽可用、满池、高位游标及回绕。无效 pool 只在实际分配时返回错误，不泄漏 actor 变化。
- 四个对象地址均执行完整入口到实际调用者返回，核对 `exit_sp=entry_sp+4`；不在特殊根或 Reset 中途截断。
- 记录完整 64 KB RAM 差异与前后 SHA、栈写域、IRQ 分步差异；不得吞掉外部变化。语义测试比较完整 actor、三个 child、其他 world、FX、throw、游标、Energy fraction 与共享 RNG。
- 单初态流仅变更声明的按钮；状态累积，不逐步注入预期中间值或 RNG。所有 restore 使用 `restore_refresh_cycles=True`。
- 唯一未观察原 core 3602 control 由 Root 生成；本模块仅复放其固定物理输入并逐帧比较 RAM/video。私有脏数据与后续构造不证明自然角色资源初始化。
- 先采原 CPU、冻结测试、保存真实 own-symbol red，再新增生产。测试断言与先行夹具不得为实现改动；最终保存 native race、Wasm 和 SHA 绑定产物。
