# Typed 音符、音色与状态归属方案

23 个定义的 typed 音符数据已从单一初态连续匹配 127,691 个原版 note outcome。该检查逐项比较 driver step、声部、时值、base pitch、transpose 和 level。音色、音量包络与声像的实际读取另有参数级证明；这两类证明不能合并表述为完整音序状态或 DSP 已经通过。

候选数据为 [audio-score-candidate.json.gz](../../../games/yuyuhakusho/data/audio-score-candidate.json.gz)，汇总与源绑定为 [audio-score/validation.json](../../../games/yuyuhakusho/verification/audio-score/validation.json)。候选尚未由 Go 运行时加载。完整原读、原状态和来源地址保留在 worktree 的 `.local/md-yuyuhakusho/audio-score-source/`；候选不含 ROM 地址、原控制指令、任意 PC 或 RAM 镜像。候选中的 DPCM4 数据是 6 个音频 payload，后续接线须按内容 ID 与 PCM owner 统一资源归属。

## 范围与数据

音乐范围为 `82/83/84/85/86/87/88/8a/8d/95/8f`，共 99 条 header 声部；效果为 `a0..ab`，共 16 条声部。原版和观察版运行比较覆盖 122,810 个 driver tick，RAM、视频、驱动状态、PCM 摘要和样本数量一致。

| 内容 | 候选结果 | 证明边界 |
| --- | --- | --- |
| 音符 | 31,875 个定义；每轨前缀和循环，或明确结束 | 连续 note outcome 全部匹配；不是固定长度录音，也不要求长共同周期全部展开 |
| FM 音色 | 100 个去重 patch，含初始候选引用 | 113 个实际 bank/index 引用、2,525 次读取；99 个不同的实际参数 patch。额外初始候选引用不冒充原版实际加载 |
| Pitch envelope | 8 条 typed 前缀／循环 | 静态与独立合成检查；本次 23 定义没有实际读取 |
| PSG 音量 envelope | 11 条 typed 前缀／循环 | 实际到达 1、2、3、4、5、10、11，175,674 次读取／输出；6～9 仅静态与合成检查 |
| 声像 | 3 个 shared window、5 个 named mode | 79,435 次实际读取／请求；每次合并低 LFO 位的顺序保留 |
| 软件调制、gate、tie、level、detune、tempo | 按源控制语义提炼为事件 | 数据已导出；连续每 tick 状态与完整请求仍待独立验证 |
| DAC | 音符引用 descriptor ID，descriptor 引用内容 sample ID | 采样、Z80 时基、抢占与混音交由对应 owner 验收 |
| E9、FM3 与 pitch keyoff bypass | helper 保留静态语义 | 本范围没有实际到达；不虚构 catalog 中不存在的事件覆盖 |

FM operator 按硬件 register slot 归一；音量 carry 需要的 TL 高位使用 `volume_carry_bias` 表达，不能因为 YM 忽略它就丢弃。完整参数说明见 [AUDIO_SCORE_PARAMETER_TABLES.md](AUDIO_SCORE_PARAMETER_TABLES.md)。

有限循环计数和子程序在提炼时展开。原程序的循环计数字节与返回 word 使用同一对 owner，避免拆分后改变原别名行为。候选只留下有序语义事件及各轨自身的前缀／循环长度；没有运行原 bytecode 的通用解释器。

## 两处实际边界

原 PSG 索引为 `(note - 0x81 + transpose) & 0x7f`。常规表虽有 70 项，本次实际还发生 282 次索引 `121..127` 读取。它们把相邻源字节作为频率 word 使用。提炼器只保存实际可达的 7 项语义 divider：`3085/3599/4113/4627/5248/55522/60662`，不将未到达的相邻 ROM 整段装入候选。每轨循环的 transpose 净变化已确认模 256 为零，因此扫描前缀与一轮循环足以覆盖本数据的后续索引。

`8f` 的 DAC 声部在第 165 步停止。其调用层级与普通 FM／PSG 不同，原停止路径直接退出当前 driver tick，后续声部当步不推进；第 166 步又恰好是 tempo 暂停，下一音符在第 167 步出现。只用所有轨道共用的累计 quanta 会把这些音符提前两步。候选 `track-end.ends_driver_tick` 显式表达该效果，回放为每轨保留自己的推进位置；共享 tempo 相位不重置。

这两项均先保存失败场景及真实 red，再修改提炼器与回放 helper。先行范围见 [AUDIO_SCORE_FAILURES.md](AUDIO_SCORE_FAILURES.md)。既有 note 和 catalog 检查器保持冻结复跑，没有删除原 outcome 或改为容差比较。

## 后续 Go 状态归属

以下是实现边界方案，尚未创建生产 sequencer。不可变定义与演进状态分开；原 RAM 布局不能成为新的通用状态数组。

| Owner | 持久内容 | 约束 |
| --- | --- | --- |
| 根声音时间线 | dispatch、四槽 queue、暂停和推进边界 | score 只输出 replace-first 等意图，不再持有第二份 queue 或 pause |
| Music clock | tempo period 与 remaining | 效果不套用 music tempo；变速按同 tick 的原顺序生效 |
| 每轨事件位置 | 前缀／循环阶段、事件序号、到下一事件的剩余推进量 | 不再同时持久保存等价的绝对 quanta；DAC 停止只让尚未访问的轨道延后 |
| 每轨发声参数 | transpose、base level、当前解析后的 base pitch、tone 选择、pitch control | transpose 只作用于新音符；已发声音符不由当前 transpose 追溯重算。pitch control 在运行时使用拆分后的 envelope 选择与 modulation enable，不能再维护 packed 镜像 |
| Gate | 配置 tick 与 remaining | 两者分别是定义值和当前倒数；tie／note reset 顺序须按原事件验收 |
| Pitch／volume envelope | typed 前缀／循环位置、必要的 scale 累积 | pitch hold 与 volume 交替无写不同；不能合并为一个猜测的 hold 机制 |
| 软件调制 | 当前定义 ID、delay/rate/step remaining、当前方向、16 位累积 offset | 定义不可变；首半步、翻向、零值环绕和 tie 不重置均须单独验证，不同时保留方向和等价的 signed delta 两份 owner |
| 声像自动化 | 当前定义 ID、step、elapsed | note 边界与持续 driver tick 分开调用；不能把 automation 计数换成音乐 quanta |
| 声部仲裁 | music/effect claim、覆盖和恢复 | 隐藏音乐继续推进；恢复以当前音乐状态为准。选择的 tone 与实际已加载硬件状态可能在被抑制期间不同，需混合 oracle 决定最小必要状态，不能先假定总能合并 |
| FM3／全局寄存器意图 | 明确的 music/effect domain 与有序全局事件 | 不把两个 FM3 域互相覆盖；E9 的全局 LFO 不能挪进单轨私有 DSP |
| DSP | 硬件有效寄存器、振荡器、包络、噪声、重采样及输出阶段 | score 不再保存一份 DSP 寄存器镜像；DAC-only 已有结果不能外推为完整 FM／PSG |

研究 helper 为对应原探针而保留的旧 duration 等诊断量，不自动成为生产 owner。生产数据已解析的时值不需要再保存原 multiplier 或复用时值 token；原程序早期与最终两个 cleanup outcome 的诊断顺序仍由研究回放证明。

## 验收缺口

下一层应从同一个静音初态连续执行 typed gate、tie、modulation、pan 和 envelope 状态，逐 driver tick 比较状态及有序 YM／PSG／DAC 请求。不能在每 tick 注入原 RAM 或从原状态补回漏算字段。

之后单列音乐与 FM／PSG 效果的 claim、覆盖和恢复流，确认初始未加载音色、隐藏期间选音色、释放时恢复及 `ab` 的 replace-first `7d` 意图。`ac..ef` 等后续效果域、PCM 总线时基、完整 FM／PSG DSP、总混音与平台接线均不包含在本次 note 投影通过范围内。

静态路线使用固定 ROM 与保存的 direct Ghidra 68000 指令列表。当前 REA raw 架构接口没有 68000／Z80，本结果没有把 ROM 冒充成其他 CPU 导入，也没有把已有 REA 模拟器包清单作为音序反编译证据。

## 复跑

以下命令在 `worktrees/yuyuhakusho` 根目录运行，输出留在工作区。原读 fixture 已保存时，前三步不需要重跑原游戏：

```sh
python3 -B scripts/yuyuhakusho/validate_audio_score_reads.py \
  --capture .local/md-yuyuhakusho/audio-score-source/full/index.json

python3 -B scripts/yuyuhakusho/export_audio_score.py \
  --out .local/md-yuyuhakusho/audio-score-review/typed

python3 -B scripts/yuyuhakusho/check_audio_score_notes.py \
  --catalog .local/md-yuyuhakusho/audio-score-review/typed/catalog.json.gz \
  --index .local/md-yuyuhakusho/audio-score-source/full/index.json \
  --provenance .local/md-yuyuhakusho/audio-score-review/typed/provenance.json \
  --out .local/md-yuyuhakusho/audio-score-review/typed/note-proof.json

python3 -B scripts/yuyuhakusho/check_audio_score_boundaries.py \
  --catalog .local/md-yuyuhakusho/audio-score-review/typed/catalog.json.gz \
  --index .local/md-yuyuhakusho/audio-score-source/full/index.json \
  --out .local/md-yuyuhakusho/audio-score-review/typed/boundary-proof.json

python3 -B scripts/yuyuhakusho/validate_audio_score_catalog.py \
  --catalog .local/md-yuyuhakusho/audio-score-review/typed/catalog.json.gz \
  --proof .local/md-yuyuhakusho/audio-score-review/typed/note-proof.json

python3 -B scripts/yuyuhakusho/check_audio_score_parameters.py \
  --index .local/md-yuyuhakusho/audio-score-source/full/index.json \
  --probe .local/md-yuyuhakusho/audio-score-source/probe-patch-reads.json \
  --out .local/md-yuyuhakusho/audio-score-parameters/review-proof
```

原读 fixture 可由 `capture_audio_score_reads.py --out <新的 worktree 内目录>` 重新采集；不覆盖已有控制、失败或参数检查来源。参数检查器从已冻结的私有 checker 提升到 `scripts/` 时只调整了一行工作区根定位，全部断言保持不变，谱系由验证索引绑定。

本轮只清理了自行生成、逐 byte 确认重复的第二次 catalog／provenance 导出，共 5,320,912 bytes。原始捕获、真实失败、源快照及规范 provenance 均保留，清单见 [audio-score/cleanup.json](../../../games/yuyuhakusho/verification/audio-score/cleanup.json)。
