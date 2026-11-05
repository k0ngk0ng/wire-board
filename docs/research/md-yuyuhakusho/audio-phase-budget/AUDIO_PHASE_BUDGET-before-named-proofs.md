# 声音 phase 预算与可交换偏序研究

已证明现存两条自然流可以把 driver tick 的计算归约到当前对象 callback 前／后，而保持全部声音字段写入、状态和分派码序不变。这个归约存在性证明覆盖所有内部消费与空 tick，但 callback 前／后的选择仍取自原观察顺序，不能作为独立运行时的 IRQ 预测算法。

局部语义成本模型也取得可验证结果：默认场地七个 row／wave／copy 块在私有原调用中与闭式成本完全相等。不过同一模型在自然场景有 6,710 个超界，不能宣称全局预算已解决。固定参考内核的并行 Z80 ROM 读取会偷走 68k 总线周期；driver 自身、DMA／FIFO 和其他规则阶段仍是下一步依赖。

本阶段仅新增研究脚本、观察器和报告，没有修改任何生产代码、已冻结 helper、旧 producer 或测试。完整索引见 [validation.json](audio-phase-budget/validation.json)。

## 目标需要保留的因果

原声音请求不一定都可听。需要保留的是：每次消费看到的四槽内容、优先级与暂停状态；特殊请求的保留、普通请求的覆盖／丢弃；消费后真正分派的命令；主／次效果组的停止、抢占和优先级释放；音序内部后续请求；独立播放实例和恢复／静音后的事件游标。相同编号的新请求不能全局去重。

视频音频时钟与暂停的逻辑时钟不同。空队列 tick 仍推进音序并决定稍后请求属于当前还是下一次消费，不能因没有分派就丢弃。独立 host CPU 的快慢不应改变虚拟时序；若主动改变虚拟 68k 速度，必须重新证明声音有效操作的偏序不变，不能以运行更快视为自动等价。

CPU 时刻可以在没有声音读写、暂停／优先级变化的计算区间内移动，只要不跨过声音有效操作的边界。这个性质可保留选择与抢占因果，但不单独证明 PCM 起播样本、合成相位或混音波形精确相同。精确到单 cycle 不是所有选择决策的必要条件；有界区间若跨过一个 request 实际写点，就必须继续细化或建立显式语义 barrier。

## 完整声音字段写入审计

新 C 观察器只记录原内核执行，不解码或执行指令。监视字段为当前优先级 `F800`、暂停 `F807`、选中命令 `F809` 和四槽 `F80A..D`。它记录每次实际写入，包括写相同值，并在每个事件前与每个视频步骤末尾重建这七个字段。两流完整 RAM、视频及 PCM 均与先前未观察控制组逐步相等。

| 项目 | 既有输入流 | 合法暂停流 |
| --- | ---: | ---: |
| 视频步骤 | 5,500 | 3,602 |
| 原始观察事件 | 2,401,755 | 1,544,325 |
| 保留事件 | 267,672 | 171,044 |
| 全部声音字段写入 | 5,953 | 3,802 |
| 闭合世界 callback | 59,788 | 38,119 |
| request 调用 | 109 | 76 |
| driver tick | 5,195 | 3,298 |
| 非 `FF` 分派 | 107 | 76 |
| callback 内非空消费 | 83 | 62 |

精简只移除无积压时 `21a06` 的空轮询返回 marker，保留全部写入、全部其他事件和原序号。所有声音写入都有明确来源：请求器、优先级消费器、分派清空、原初始化、driver reset、主效果停止、有限效果优先级释放、暂停确认／恢复，以及音序内部请求；未知写入为 0。

优先级消费的每一笔物理写入都用原不可变优先级表独立计算后核对，不只比较最终四槽。分派 `37cb4` 读取的命令与随后清为 `FF` 的写入也完整核对。

两个容易遗漏的来源已经确认：

- 原启动 `37e` 把 `04` 放入 `F80A`，不是直接写入 `F809`。早期研究模型对字段的误认首先失败，原失败与修正前模型保留，并与已有 `ghidra-entry.txt` 核对。
- 音序命令 `EB` 在 `38f28` 执行 `MOVE.B (A4)+,F80A`，无条件覆盖 slot 0，不走 `21960` 的特殊保留规则。两流第 556 步都直接写入 `93`，第 557 步分派 `93`。它属于原冷菜单流，完整对照没有将它裁掉。

## 请求必须在实际写点线性化

既有流第 3612 步的 `7d` 请求在 `21960` 入口时队列为空，入口时间为 160,679 master cycles。IRQ 保存的被中断 PC 是 `21964`，即比较和分支期间。当前 audio tick 仍无声音分派，实际 `2196c` 写入发生在 285,006 master cycles；请求随后返回。

因此，把 request 原子地线性化到函数入口会让这个声音早一个 tick。所有 109／76 次请求都可在实际 store 点用原队列算法核对，但函数最终队列相同不代表起播时序也相同。

两流还有 23／15 次 tick 发生在同一 callback 的稍后 request 之前，以及 5／3 次 tick 在 request 之后；既有流另有上述 1 次 tick 位于 request 函数内部。空 tick 的相对位置是可听因果的一部分。

## 已证明的 callback barrier 归约

把每个 driver tick 视为有序的 driver 操作组，保留其内部请求、消费、分派和优先级／暂停写入。对每个 callback，若 tick 前没有世界声音写入，可把其计算移到 callback 前；若 tick 后没有世界声音写入，可移到 callback 后。driver tick 的原先后顺序保持不变；原音频时间戳另外保留。

| 归约位置 | 既有输入流 | 暂停输入流 |
| --- | ---: | ---: |
| callback 前 | 3,158 | 2,039 |
| callback 后 | 5 | 3 |
| 不在闭合 callback 中，保持原位 | 2,032 | 1,256 |
| 必须在 callback 内另设 barrier 的已观察 tick | 0 | 0 |

归约后完整 9,755 笔声音字段写序逐项相等，每笔前后状态与全部 183 个非 `FF` 分派码序也相等。145 次 callback 内消费全部保留；没有只挑 arena 或忽略 actor 的反例。

这只证明两个观察流存在安全归约。选择哪一个 barrier 使用了原 producer 与 tick 的已观察偏序，不是对任意新输入的独立预测。未恢复的 driver／音序执行及优先级释放仍是原观察器输入，本研究不把它们伪装成独立 DSP。

帧尾归约有更直接的反证：全流从第 375 步开始分歧，原 selected 为 `FF`，帧尾放置会选到 `73`，此前 69 个正常分派检查（包括 `FF`）完全相同。为避免仅由原菜单证明，另保持 1800 步前的原顺序，只对战斗阶段应用帧尾假设；第 1813 步仍首次分歧，原 selected=`FF`，假设 selected=`A3`。在此前 1,508 个分派检查相同的前缀内即可证明提前一 tick，无需伪造后续 DSP 结果。

## 默认场地的局部成本原型

模型只按已恢复的语义循环结构计算基础成本，不解释寄存器、不执行 ROM，不构建 CPU／RAM VM。PC 仅用于标记原 oracle 中相同语义块的边界。

| 语义块 | 固定结构 | 基础 68k cycles |
| --- | --- | ---: |
| A 主行 | 16 轮，每轮四笔 long store | 972 |
| A 尾行 | 4 轮，每轮四笔 long store | 268 |
| B 前缀行 | 8 轮，每轮四笔 long store | 508 |
| 第一组波形 | 16 轮，indexed byte 与右移 4 | 1,068 |
| 第二组波形 | 16 轮，indexed byte 与右移 3 | 1,036 |
| B 平行行 | 9 轮，每轮四笔 long store | 574 |
| 视差尾表 | 7 轮，每轮四笔 long copy | 726 |
| 合计 | 不含 gate、初始化、camera、feedback | 5,152 |

来源为 `20918` 静态清单的循环与分支、固定 core 的基础时序和 DBF／shift 处理。正常 taken DBF 为 10 cycles，最后一次为 14；`MUL=7` 将 68k 周期转为参考 core 的 master cycles。

在 192 个原私有配置中，336 个实际经过的块全部满足：

```text
observed_master = semantic_base_cycles × 7 + observed_refresh_count × 14
```

独立区间预测不读取原观测的 refresh 次数，而按每次 deadline 896、插入 14 的保守上界计算，也全部包住私有结果。参考 CPU 的普通 refresh、长 DIVU 的额外处理和中断入口不能统一算成这个公式；camera 的循环、clamp 和数据相关 DIVU 成本尚未纳入此最小原型。

## 自然超界与并行硬件依赖

同一冻结模型用于 3,223 个自然 arena callback，得到 21,963 个没有额外 IRQ／自身硬件访问的块，以及单列保留的 38 个 IRQ／硬件区间。前者仍有 **6,710 个超界**。原失败完整保留，没有扩大固定 arena 常数来改成通过。

扣除基础与已观察 refresh 后，残差为 `70/77/140/147/154/210/217/224/287/294` master cycles，全部能分解为若干 `70/77`。固定 core 的 `memz80.c:z80_request_68k_bus_access` 在 Z80 bank 读取 68k ROM 时，会按边沿把 `m68k.cycles` 推进 70 或 77，并把 Z80 推进 45；源码还明确说明这是经验近似的硬件 wait-state 模型。

这使得只访问 ROM／RAM 的 arena 也会受并行 PCM／DAC 的总线访问影响。私有 `m68k_run` 不推进 Z80，才得到纯基础成本完全相等。当前证据是残差算术与已固定源实现的强相关，尚未逐次直接记录每笔 Z80 bank read，不冒称独立总线模型已恢复。

其他必须区分的成本包括：

- 68k IRQ acknowledge 依 E-clock 对齐为 50～59 cycles，不能固定成 44。
- VDP DMA 可在 scanline 推进时直接延后 68k，FIFO 满也会等待；callback 自己没有 VDP 访问不代表没有外部占用。
- Z80 总线读写、bus request 与原 `1f2a` 等忙标记存在握手等待。
- driver 即使本次 selected=`FF`，仍推进音乐／效果声和寄存器写入。在既有流中这类 tick 的原持续时间为 5,327～594,118 master cycles，包含并行等待与中断，不能全算作 driver 自身的算术成本；暂停确认／保持／恢复路径另有不同成本。不能用一个空队列 tick 常数替代。

报告同时保留了 37 个「给定原 IRQ 接受窗口与原 row 起点」的条件性语义块定位。它们没有定位错误，但输入本身来自原 oracle，且自然无外部总线的前提已被超界反例否定，因此不能计作独立 IRQ 预测通过。

## 可继续实施的路径

局部纯计算成本可以闭式表达；同一个 inert block 内没有必要逐指令插入声音操作。下一步需要独立 PCM／DSP 的时间与总线语义、driver／score 操作成本，以及高层规则 producer 阶段协同。将这些成本保持为有界区间；若区间不会跨声音有效操作，可安全归约；若会跨 request 实际写点、priority release 或暂停边界，就继续细化语义阶段。

当前不能由 `StepChuSessionFrame` 的帧号或 callback 次数独立推出全部声音偏序，不能将原 PC 时间表作为运行时数据，也不能回退到帧尾单次消费。研究索引保留 `global_independent_schedule_solved: false` 与 `production_modified: false`。

## 复跑

规范输入、完整精简 trace、局部成本原数据和真实失败都保存在 `audio-phase-budget/`。纯因果研究无需执行内核：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/analyze_audio_phase_budget.py \
  --capture-dir docs/research/md-yuyuhakusho/audio-phase-budget/compact \
  --out .local/md-yuyuhakusho/audio-phase-budget/causality-review.json

PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/arena_phase_cost_model.py \
  --capture docs/research/md-yuyuhakusho/audio-phase-budget/arena-private.json.gz \
  --out .local/md-yuyuhakusho/audio-phase-budget/private-cost-review.json
```

`check_arena_phase_budget.py` 对自然 trace 的原冻结模型应返回非零，复现全部 6,710 个超界。重新取得被动原数据使用 `capture_audio_phase_budget.py --mode all --out <工作区内新目录>`；该路径需已固定 ROM、hook core、既有原控制组和本地 clang。新观察器自身再次逐步核对 RAM／视频／PCM，并核对所有声明声音字段的逐写重建。

本分工清理 22 个编译／重复临时路径，共 24,484,997 bytes；完整原观察、全部声音写入和 6,710 个超界保留。三种离线复跑的结果逐 byte 相同，分别保持因果通过、私有成本通过和自然成本拒绝。
