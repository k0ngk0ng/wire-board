# 随机工作字与联机确定性适配

本轮核验的是固定日版 ROM（SHA-256：`6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe`）的 `$34`。结论是：它不是每个游戏 tick 调用一次的独立 PRNG；空闲循环、帧计数、CPU Carry、中断位置、敌人内部写入和初始化都会影响它。

用户随后明确允许独立联机版的随机序列与 FC 不同，优先原版玩法与联机，同时保留原版随机运算、敌人决策条件和可恢复的确定性。因此本轮分别交付原版算术 `RandomState` 与明确标记的平台适配器 `AdaptedRandom`；没有修改 `World`、敌人策略或原版 ROM。

## 原版观察

### `$F880` 是持续运行的空闲累加循环

```text
F880  LDA $1A
F882  ADC $34
F884  STA $34
F886  JMP $F880
```

这四条指令的标称耗时合计 12 个 CPU cycle：两条零页读取、一条零页写入和绝对跳转各 3 cycle，不包含中断或 DMA 等额外等待。公式为 `sum = 已加载的 A + $34 + CarryIn`，写回 `sum & 255`，CarryOut 为 `sum > 255`。循环没有 `CLC`，也没有外部 PRNG 调用；上一轮 Carry 会进入下一轮。

`$1A` 在 `$C001` 进入游戏主分派时增加。NMI `$F889` 保存并恢复 A／寄存器／状态；真实轨迹中的 5,096 对 NMI 进入与 RTI 观察均保留 A 和 Carry。NMI 本身可能改变 `$1A` 与共享 RAM，但不能把其内部算术 Carry 当作空闲循环恢复后的 Carry。

中断可落在 `LDA` 与 `ADC` 之间。自然样本确实出现 A 仍为 111，而 `$1A` 已变为 112；这次 ADC 必须使用 111。只保留「当前帧号＋随机字节」不足以逐 cycle 复现原版。若中断落在 ADC 与 STA 之间，还要保存尚未提交的累加结果，不能在中断后重新读取 `$34` 重算。

初始 120 帧单人样本有 194,379 次完整 idle ADC，而各视频帧的执行次数不同；后续各场景进一步覆盖从没有空闲机会到上千次累加的情况。没有证据支持给每个游戏 tick 固定一个 idle 次数。

### 同 tick 的 `$34` 是可变共享状态

以下地址由固定 ROM、REA 汇编和实际执行分别绑定：

| 位置 | 行为 | 本轮证据 |
| --- | --- | --- |
| 固定 bank `$F884` | 空闲循环将 ADC 结果写回 `$34` | 自然完整指令写回检查与 3,584 组受控原 CPU 运算 |
| bank 0 `$9FA5..$9FA9` | `LDA $34; ADC $1A; STA $34` | 原始敌人分派的 180 组受控执行 |
| 固定 `$C2A8..$C2AA` | 同时将 `$1A` 和 `$34` 清零 | REA 静态汇编；本轮自然录制未执行该直接写点 |
| 固定 `$C30B` | 从 `$28` 到 `$EF` 的 RAM 清零循环，包含 `$34` | REA 与启动／流程切换的原始写回 |
| 初始化 `$F95B` 的间接写入 | 初始化覆盖 `$34` | 启动时实际内存写回观察；回调 PC 为 `$F95D` |

`$9FA5` 不是通用 `rand()` 的调用约定，而是敌人行为中的内联更新。前面 `$9F9E` 读取 `$2F`（武器强度），连续三次 `ASL A`，把八倍强度写入 `$08`。因此进入 ADC 的 Carry 来自强度的 bit 5。随后累加值写回全局 `$34`，右移并取低两位选择间隔，再减去八倍强度。后续同 tick 的其他读取会看到新 `$34`。

受控运行通过真实 `$E54F/$E58A` 分派：关卡索引 1、该关敌人表 `$E5FF`、类型 `$1C`、状态 3、槽位 15；设置世界／实体输入后执行原版路径，并在分派返回时恢复。它证明该路径能实际写入随机工作字，但不表示已经自然打到该敌人，也不构成这一敌人的完整规则验收。

其他调用点可能仅采样。例如跑兵生成路径 `$AB00` 做 `ADC $34` 后取位，没有在该处将结果写回 `$34`；`$8774`、`$87EF`、`$8B93` 等敌人分支直接读取 `$34` 的低位。不能把这些读取统一替换成每次都会推进的新随机数调用。

本轮做了完整 PRG 的直接零页读写指令字节候选扫描，也记录了所运行场景的实际访问。扫描可能命中数据，不能证明所有候选都是代码；索引／间接写入也不由直接指令扫描穷尽。因而这里给出已确认写入点和有界观察，不声称排除了整个游戏中所有其他别名写入。

### lag 帧不等于随机字节冻结

NMI 检查 `$1B`。上一轮主逻辑尚未结束时走忙碌分支，继续必要的显示／音频工作，但不再次调用 `$C001`。因此显示帧、游戏主分派次数和 `$1A` 的变化不能混为一谈。

启动实测例子：第 5 个已观察视频帧结束时 `$1A=1`、`$34=0`、主逻辑仍忙；第 6 帧被 FCEUX 标记为 lag，没有新的游戏主分派，旧分派完成后却运行了 300 次 idle ADC，最终 `$1A` 仍为 1，`$34` 变为 45。另一个 lag 帧记录了 1,767 次 idle ADC。没有空闲执行的时段也存在，初始化写入还可能单独改变 `$34`。

所以不能使用「每个视频帧固定抽样」「只有游戏 tick 才改变随机字节」或「lag 帧完全不动随机状态」作为原版事实。

## 独立联机版采用的适配

`random_adapted.go` 使用标准 SplitMix64 产生帧熵，但将这一部分明确划为平台适配层，**不称作原版算法**：

1. 开局时由服务器生成并传入一个 64 位 seed。
2. 每个权威模拟帧开始、原版规则阶段执行前，只调用一次 `AdvanceFrame(nextFrame, originalCounter)`。
3. SplitMix64 推进一次，取输出高 8 位，与共享 `Work.Value` 做异或。这一步是适配层注入。
4. 调用原版 `Work.Accumulate(originalCounter, IdleCarry)`，保存返回的 Carry。
5. 此帧所有规则读取同一个 `Work.Value`。原版会修改 `$34` 的敌人分支仍调用该工作字的原版累加操作；其他读取不额外抽样。

只用固定 idle 次数会保留原累加器的模 255 短周期／固定点风险，特别是计数 0／255 与特定 Carry 组合。帧熵注入避免依赖这类次数选择。测试中的频数上下界只用于检测冻结、短周期和严重偏斜，不证明与 FC 的统计分布完全相同，也不提出密码学质量承诺。

`AdvanceFrame` 要求帧号连续，从 1 开始。重复、跳号或损坏的适配状态都会在消费熵前返回错误。网络重传、快照读取、浏览器刷新、画面插值、托管切换和服务端批处理次数均不得调用它；批处理中的每个实际模拟帧仍按相同顺序调用一次。

## 接入与保存接口

```go
random := contra.NewAdaptedRandom(serverSeed)
err := random.AdvanceFrame(nextAuthoritativeFrame, originalFrameCounter)
if err != nil {
    // 交给现有原子世界步进返回错误，不提交半帧。
}

// 普通规则读取，不推进：
choice := random.Work.Value & 3

// 仅在原版 $9FA5 分支已满足时执行：
enemyCarry := random.Work.Accumulate(originalFrameCounter, weaponStrength&0x20 != 0)
_ = enemyCarry // 可供该原版分支继续运算，不能写入 random.IdleCarry。
```

`RandomState.Accumulate(loadedOperand, carryIn)` 仅恢复二进制 ADC／写回算术，不包含任何时钟、随机种子或自动推进。`AdaptedRandom.Work` 是共享入口，不能给每个敌人复制一份整帧开始时的随机值，再丢弃敌人写入。

完整存档必须保存 `Version`、初始 `Seed`、当前 `Entropy`、累计 `Frames`、`Work.Value` 与 `IdleCarry`。三个 64 位字段用十进制 JSON 字符串编码，避免经过 JavaScript 时被舍入。`Validate` 检查版本与 seed／步数对应的生成器状态。恢复存档、重连、取消托管或切换关卡不能重新创建 seed；新的对局才创建新的适配状态。

暂停和场景切换是否仍属于一个权威模拟帧，由 `World` 已恢复的规则阶段定义；适配器不根据墙钟、网络延迟、浏览器动画帧或真实设备快慢自行推进。本轮未把适配器接入 `World`，也未替代敌人原版的射击、间隔、跳跃条件。

如果将来需要硬件时序精确的研究模式，仍须恢复各阶段 CPU 耗时、PPU／DMC／DMA 对时间的影响、中断阶段及已加载 A／Carry／待提交结果。当前在线适配不假装解决这些边界，也不从 oracle 的 after 值回填世界结果。

## 证据与复查

入口为 [random-verification.json](evidence/random-verification.json)。它链接完整 REA bundle、观察轨迹、原始 ROM／模拟器／初态／场景摘要，以及实现前 red 和实现后 race 记录。

- 自然输入或既有受控关卡初态的只读观察：5,108 次 `frameadvance` 采样；两次冷启动的首个调用不产生 `registerafter` 回调，实际保留 5,106 个视频帧回调。各运行的全部 RAM 样本与相同输入的无钩子运行逐字节一致。
- 在线检查了 7,916,951 次完整 idle ADC 写回，保留 11,425 个代表性输入／输出样本，包括加载计数跨 NMI 的情况；不是把全部七百多万组样本都保存为 JSON。
- 3,584 组原 CPU 受控运算覆盖全部 256 个工作字、两种 Carry 与 7 个边界加数；另有 180 组真实敌人分派的共享写回。
- 7 项定向 race 测试通过：原版算术、自然中断输入、敌人写回、适配器已知序列、4,096 帧保存恢复、非法步进原子拒绝及 4 组各 65,536 帧的防退化检查。

采样时实际内存写回的回调 PC 已在操作数之后，例如 `$F884` 的 STA 回调显示 `$F886`。本轮特意使用写回回调提供的值作为结果；若在下一条指令执行钩子才读取 RAM，中间可能已经执行一次 NMI 并清零 `$34`，那就不是该次 STA 的结果。记录中的 `writePC` 保留此偏移，不冒充指令起点。

```sh
source scripts/env.sh
python3 -B scripts/contra/random_prepare.py
node scripts/contra/static_rea.mjs .local/contra-random/random-spec.json
.local/board-art-env/bin/python -B scripts/contra/random_capture.py --mode natural --label <新标签>
.local/board-art-env/bin/python -B scripts/contra/random_capture.py --mode boot --label <新标签>
.local/board-art-env/bin/python -B scripts/contra/random_capture.py --mode controlled --label <新标签>
.local/board-art-env/bin/python -B scripts/contra/random_mutator_capture.py <新标签>
go test -race ./games/contra/engine -run '^Test(RandomOriginal|AdaptedRandom)' -count=1 -json
```

REA 使用私有 stdio 会话，没有切换共享 MCP 目标；FCEUX 在只允许工作区写入的 sandbox 内运行。八个关卡初态沿用现有明确标注的受控初始化，不表示本轮自然通过八关。清理仅涉及 `.local/contra-random/` 与 `dist/cr-tmp/` 中的本次临时 ROM、重复初态、缓存、失败运行及中间分析；保留文档中的可重复证据，不改写用户 ROM、其他游戏工具或原始初态。
