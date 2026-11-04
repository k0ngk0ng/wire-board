# 来源 1「藏马」攻击选择：实现前失败场景

本清单先于 `fighter1_attacks.go` 和对应测试建立。工作基线为 `b5723d24`；本轮仅恢复四个选择器及其立即初始化，不据此认定完整招式或对局已完成。静态依据为现有 Ghidra 指令清单；REA 尚不支持本目标的 raw 68000 路由，不将清单称为 REA 反编译结果。

## 入口与状态边界

| 入口 | 模式 | 主要失败场景 |
| --- | --- | --- |
| `163fe` | 地面普通攻击 | Request 8 优先；Request 7 仅在 Pose 不为 11 时启动；16 种方向；B 优先于 A；无攻击键；投技与普通攻击分支；Facing 任意非零；重置范围与速度正负号 |
| `16540` | 空中普通攻击 | 方向表实际落点；B 优先；A 的两种 Pose；无键直接返回；保留未写字段 |
| `165b6` | 地面特殊攻击 | 未知 Request；1～4 的 ChildSize 非零门槛；Changed 优先级；有符号 Energy 与 Health；成功与失败的字段保留 |
| `1668c` | 空中特殊攻击 | 原函数仅 RTS；所有字段保持不变，包括未读取且对其他分支可能无效的 Pool、Throws、ChildSize、Power |

API 使用 `Fighter1AttackState{ChuAttackState; Power int16}` 和 `SelectFighter1Attack`。Power 对应既有对象 `+7c` word；不建立第二份 canonical 状态。

## 九个特殊构造

| 根 | 条件与成本 | 失败场景 |
| --- | --- | --- |
| `18b7a` | Request 3；ChildSize=0；16 | 不要求 Changed 攻击键；成功清 Request；不应清 Power |
| `18ba4` | Request 1；ChildSize=0；Changed B；16 | B 与 A 同时变化时优先 B；成功清 Power word，失败保留 |
| `18bbc` | Request 1；ChildSize=0；Changed A；24 | 完整 Power word 被清；不影响其他对象字段 |
| `18bea` | Request 2；ChildSize=0；Changed B；16 | 完整写 VelocityX=`0x40000`，不擅改朝向 |
| `18c04` | Request 2；ChildSize=0；Changed A；16 | 完整写 VelocityX=`0x60000`；B 同时变化时仍选前一根 |
| `18c36` | Request 4；ChildSize=0；Changed C；24 | C 优先；仅将 VelocityX 高 word 写为 `0x30`，保留 low16 |
| `18c4e` | Request 4；ChildSize=0；Changed B；24 | B 优先于 A；VelocityX 高 word=`0x70`，保留 low16 |
| `18c66` | Request 4；ChildSize=0；Changed A；24 | VelocityX 高 word=`0xb0`，保留 low16 |
| `18c96` | Request 6；Health≤111；Changed A；200 | Health 为 signed word；不受 ChildSize 门槛限制；成本失败不清 Request |

成本覆盖 `-32768/-1/0/cost-1/cost/cost+1/32767`；Health 覆盖 `-32768/110/111/112/32767`；ChildSize 覆盖 `0/1/0x100/0xffff`。检查全部 Changed byte 及按钮组合、方向高位、Facing `0/1/3/128`、Power `0/1/-1/-32768/32767`、非零速度 low16 与 Energy fraction。

## 共享写入、完整性和错误处理

- 四个对象地址分别执行原 CPU 完整入口至调用者返回，核对 `exit_sp=entry_sp+4`；不能在构造入口截断。
- 快移效果覆盖空池、当前槽可用、扫描后槽可用、满池、游标回绕和高位游标；核对分配指针、全 16 槽字节、位置小数、Layer 回绕、Attributes 掩码。
- 投技覆盖全部四种横向方向、Facing 任意非零、坐标 word 回绕、16 个缓冲槽与尾游标；核对全 512 字节和游标。无存储、非对齐游标、超界游标必须报错且返回原状态，不修改共享 slice。
- 成功、拒绝和无操作均核对完整 actor、三个 child、其他 world 对象、完整 FX 池、throw 缓冲、两个游标、Energy fraction 和共享 RNG；不对差异做掩码。
- 不支持模式及实际访问到的无效 EffectPool／Throws 存储须原子失败；未访问的存储不应触发额外校验。
- 连续流只允许改变声明的输入按钮，从单一初态累积执行，不注入原版中间状态、RNG 或每步预期值。
- 原 core 帧基线使用唯一的合法菜单 1 ordinary／special 物理输入；被动观察必须逐帧保持 RAM／video 与原基线一致。私有边界上下文明确标识，不充当自然游玩证明。恢复均使用 `restore_refresh_cycles=True`。

先保存原 CPU fixtures，再冻结完整字节比较测试并实际运行缺失符号 red；随后才新增独立生产文件。保留初始失败与最终 native race／Wasm 结果，不修改旧 Go、bundle、PROGRESS 或既有证据。
