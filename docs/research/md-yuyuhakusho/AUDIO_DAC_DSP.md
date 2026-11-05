# DAC-only 参考帧 DSP 与 PCM 对照

独立 Go `audio/dsp` 已在给定 DAC／enable／pan 输入与明确 master-clock 原点时，重建固定参考 profile 的连续 PCM16。原生竞态与 Node／Wasm 各 6 项顶层检查通过；9 组场景、441 个参考音频帧、324,556 个立体声采样帧及每帧完整 typed 状态全部相同。

这项证明只覆盖 DSP 层。它不生成原事件时刻，不执行 CPU、固件、PCM 命令、FM／PSG 或世界规则，也不把原 PC／时间表带入生产。机器索引见 [audio-dac-dsp/validation.json](../../../games/yuyuhakusho/verification/audio-dac-dsp/validation.json)，先行范围见 [AUDIO_DAC_DSP_FAILURES.md](AUDIO_DAC_DSP_FAILURES.md)，API 见 [audio/dsp/README.md](../../../games/yuyuhakusho/audio/dsp/README.md)。

## 实际配置与同一初态

配置来自固定参考进程的只读 scalar inspector，不是根据注释推测。原始配置、字段归一化和 ABI 校验见 [runtime-config.json](../../../games/yuyuhakusho/verification/audio-dac-dsp/runtime-config.json)。

| 项目 | 实际值 | 本 profile |
| --- | --- | --- |
| 芯片模型 | `YM2612_DISCRETE = 0`，`ym3438 = 0` | profile ID 为独立枚举 `1`，不把原 `0` 误读为关闭 |
| HQ FM | `1` | 固定 band-limited profile |
| 采样率 | 44,100 Hz，双声道 | 交织 PCM16 |
| FM preamp | `100` | 固定原比例 |
| filter | **`0`** | 低通／EQ 均未启用 |
| lowpass range | `39321 = 0x9999` | 仅原配置留存，不参与运算、不用于拟合 |
| mono | `0` | 保持左右通道 |
| 芯片采样间隔 | 1,008 master cycles | 周期网格从原 audio-clock epoch 推导 |

第 2,520 步静音检查点的绝对 master 原点为 `2,258,020,800`。同初态是 DAC128、disabled、pan0、上一芯片输出 `[768,768]`、resampler integrator `[32704,32704]`、18-tap tail 全零。原 fractional offset 为 `4441963404461476`，与 `(factor/2 + origin*factor) mod 2^52` 精确相同；固定 factor 为 `3698956963656`。

只读 inspector 初版把 `YM2612SaveContext` 返回的 3,624 字节当作结构加 24，实际检查失败。原源代码的 `save_param` 已推进一次指针，而 24 个 DT index 又各显式推进一次，因此是 3,576-byte typed 结构加 48；旧 inspector、真实失败和修正谱系保留。生产未保存该结构、host 指针或 native context。

## 先推进芯片，再写寄存器

原数据写入口在写 YM register 之前先推进芯片：生成 `chipTick < writeCycle` 的采样，再提交新寄存器值。相同 cycle 的输入保留原顺序，没有额外采样插入；恰在 tick 的写影响尚未生成的该 tick。

DAC byte 先映射为 `((value-128)<<6)` 的有符号 14-bit 值。离散 YM2612 还在每通道上施加正负 ladder 偏移；pan 静音并不等于负 DAC 信号没有偏移。当前固定 profile 的其他 FM 声部为静音条件，但仍保留其参考基线。随后依次进行固定 FIR profile、15-bit integrator／high-pass、PCM16 clipping 和交织输出。由于实际 filter0，不运行低通或保存冗余低通状态。

Go 实现是 typed 算术与循环卷积。运行时仅含 264 个固定数学 kernel 系数；没有 Cgo、原 core、CPU 或固件。数值 profile 的来源和 LGPL notice 见 [KERNEL_SOURCE.md](../../../games/yuyuhakusho/audio/dsp/KERNEL_SOURCE.md)。

## 为什么不能直接使用全局 raw 时间排序

第一个纯算术 prototype 严格按全局原 handler 时间推进：三个场景的 58,876／66,236／66,236 个立体声采样帧全等，但 `pair-long` 有 2,686 个样本值不同，最大误差 1,266。首差位于第 84 个视频块附近。这个真实失败保留为 [global-timestamp-prototype-red.json](../../../games/yuyuhakusho/verification/audio-dac-dsp/global-timestamp-prototype-red.json)，没有裁切、改增益、移动原事件或启用低通隐藏差异。

只读 frame-flush probe 复跑同一 160 步流，RAM／视频／PCM 仍与规范原控制完全一致。第 83 帧的原 DAC 写时间是 `896070`，超过帧尾 30 周期：

| 时刻 | FM count | frame start phase | 临时缓冲立体声位置 |
| --- | ---: | ---: | ---: |
| 写之前 | 889,992 | 936 | 882 |
| 原写 handler 推进之后 | 897,048 | 936 | 889 |
| 原 frame flush 之后 | 0 | 0 | 0 |

原 handler 算出了 `chipTick=896040` 的 lookahead，但 frame flush 只提交 `chipTick<896040` 的 888 个帧内采样，随后重置计数／缓冲。最新 DAC register 值保留给下一帧，而 last-flushed output 仍为 `[-928,-928]`。原 scalar 证据见 [frame-flush-probe.json](../../../games/yuyuhakusho/verification/audio-dac-dsp/frame-flush-probe.json)。

因此固定参考 profile 必须表达 frame 事务，不可将原 handler cycle 当作无帧边界的物理 pin-edge 时间。修订 prototype 保持所有 raw cycle，按原 update／write／frame-flush 顺序提交，并显式丢弃 lookahead，四条原流共 309,100 个立体声采样帧全部归零差异，见 [reference-frame-prototype.json](../../../games/yuyuhakusho/verification/audio-dac-dsp/reference-frame-prototype.json)。这是参考 core 的行为，不自动等同真实硬件。

## Queue、FrameBoundary 与状态 owner

- `New(Config, InitialState)` 只接受 `ReferenceFrameDiscreteHQ44100`。未知 profile 明确拒绝。
- `Queue([]Event)` 复制当前参考帧未消费输入，可以分批接收；不会提前改变 DAC、上一芯片输出、integrator 或 tail。
- `FrameBoundary()` 每次提交固定 896,040 周期，只输出帧内芯片采样。原 raw 时间超过帧尾的寄存器写仍提交，不 clamp 原 cycle；输出 lookahead 按参考行为丢弃。
- `Snapshot()` 以及 `New` 都深拷贝 pending，可在半帧输入分批之间保存恢复。

`State.Time` 是最后输出 flush 边界。`Pending` 是尚未消费的输入 owner；`InputAhead` 是已提交 raw 输入超过该边界的独立游标量，不能由 Time／DAC 值恢复。offset 和 next chipTick 由绝对时间推导，不另存；本 profile 未启用低通，所以没有低通状态。上一已提交芯片输出与当前 DAC register 是不同流水阶段：例如帧尾 spill 后当前 register 已为新值，last-flushed output 仍是旧采样值，两者均有真实独立语义。

当前有界合同为每帧最多 4,096 条 pending 输入、最多 1,008 master cycles raw lookahead。私有原 DSP 输入验证了 195／1,008、下一帧恰等于前游标、相同 cycle、end 处写及 pan00／40／80／C0。它不是物理硬件最大合法 overrun 的宣称。正常本固件的 DAC／enable／pan store 为 7T／13T，FM handler 不再增加 Z80 cycle，因此从 `while(cycles<target)` 启动的这一类 store 本身 overrun 小于 195；ROM bank 等待在另一类读指令中发生，不能混入本次写的周期。

所有参数／队列校验在临时值上完成，非法输入不清空 pending、不推进 Time／InputAhead，也不提交部分 PCM。`origin+time`、raw cursor 及帧尾加法都有显式检查。offset 使用 `Mul64/Add64` 取模低位：`2^52` 整除 `2^64`，因此丢弃乘积高位与加法 carry 后仍得到精确模余数；固定一帧的乘积／输出规模另有上界。复查时发现 raw cursor 还需检查 `origin+Time+InputAhead`，先加入真实失败的独立 regression test，再修校验；原五项冻结 native 测试未改。

## 原生与 Go／Wasm 验收

原 oracle 包含五组直接私有芯片边界输入，以及四条已采实际 CPU DAC 事件的直接原生 DSP 重放。九组共 441 帧／端，原 core 和观察 core 的 PCM、归一化 typed state、未改变的 guest RAM／视频一致。四条实际 DAC 事件重放的波形仍逐 byte 等于之前 CPU 执行原流，不只等于新 prototype。

Go 与 Wasm 均逐帧比较 PCM16、DAC state、last output、integrator、18-tap tail、InputAhead、空 pending，以及原 offset／next tick 的派生关系。另覆盖半帧输入分批、自己的 snapshot 恢复、深拷贝保护、同 cycle 顺序反例、跨帧旧输入、未知 profile／event、非法 pan／enable、overflow 与资源 cap 原子拒绝。总计 324,556 个立体声采样帧零差异。

给定原输入时刻的 DSP 证明，不是独立 cadence 或真实扬声器录音证明。完整 FM／PSG 以后必须保留被丢弃 lookahead 采样已经推进的 oscillator／envelope 侧效；当前 DAC-only 映射是无记忆的，不能将这一优化照搬到完整芯片。

## 复跑与后续

Go／Wasm 命令见 [API README](../../../games/yuyuhakusho/audio/dsp/README.md)及 `verification/audio-dac-dsp/wasm-command.json`。重新读取原运行配置或重复 native oracle：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/inspect_dac_dsp_state.py \
  --out .local/md-yuyuhakusho/dac-dsp-config-review
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_dac_dsp_boundaries.py \
  --out .local/md-yuyuhakusho/dac-dsp-native-review
```

新输出仅在 worktree。规范原 DAC 事件和其原 PCM 来自 `.local/md-yuyuhakusho/audio-pcm-model-first/`；不要删除或覆盖它们，也不要用 prototype 输出充当新 original。

仍需独立 PCM player、bank／总线／IRQ 时基、完整声音调度、FM／PSG 合成、与世界规则和平台流式播放接线。这个 profile 没有加入声音目录、仲裁、scheduler、世界或媒体状态的第二份 owner，也没有发布、上传或部署。

本分工清理 10 个私有重复／临时路径，共 7,075,992 bytes。九组完整原生 PCM／state 和 portable 夹具再次采集／导出逐 byte 相同；已被九组规范 oracle 覆盖的五组初版数据在删除前逐 byte 核对。规范 native 输入、状态、PCM、实际失败、最终日志与来源保留，未动其他 scope 或工作区外文件。
