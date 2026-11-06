# 有序 YM reset 输入域

reset 是原 `fm_reset` 调用，保持实际调用顺序。它先按原请求 cycle 同步 FM，再进行 warm YM reset 并清 busy；它不是全音频重置，不清 PSG、当前 FIR／integrator、已生成 FM 进度或已提交输出。

```go
// 正常的非降时间，包括 reset release。
err := audioDSP.ResetYM(cycle)

// 只用于 Device 实际完成 DAC 原子段后才观察到的 reset 请求。
// 两个 cycle 必须来自同一实际事务，不从任意 FM／PSG 访问拼造。
err = audioDSP.ResetYMAfterDAC(requestCycle, completedDACCycle)
```

两种方法使用与普通读写相同的 uint64 相对 master 时间线。`ResetYM` 不得早于当前 cursor。特殊方法要求 `completedDACCycle` 等于当前完成 frontier，`requestCycle` 不晚于该完成点，也不得早于已 flush 的帧。请求与完成点最多相差 `MaxResetDACBackstep=194`。调用者须从 PCM Device 的实际 DAC 原子段取得这两个时间；此方法不能用于把任意回时的 YM／PSG 操作伪装成 DAC 完成。

194 是本固件的有界 profile：single DAC 写为 13 个 Z80 T-states，即 195 个 master cycles，请求落在该原子段内时回退严格小于 195。dual 的 7T 写对应更小的 104 上限。这里没有宣称通用硬件或任意 I/O 指令的最大延迟。普通事件的 frame lookahead 上限仍为 1,008，reset 不扩大它；原生矩阵包含完成 frontier 达到帧尾＋1,008、请求回退 194、随后显式 flush 和恢复的组合。

特殊方法保留完成 frontier，把未经修改的原 request cycle 交给 FM 更新器。已经生成的采样、副效和 FIR 均不撤回；不 sort 或 clamp 原输入。后续普通读写仍不能回到 frontier 之前，旧 Queue 不增加 reset 或回时能力。

warm reset 初始化芯片参数、oscillator／envelope、LFO、定时器和 CSM，同时保留共享 address latch 和延后生效的 keyscale。后续 data-only 写仍使用该 latch；下一次实际采样刷新才推进 keyscale 阶段。busy 被清为零，PSG 和 resampler 保持原样。

reset 不持久保存原请求时间、事件历史或第二份状态。它计入本帧 4,096 次操作上限；失败不提交芯片、FIR、frontier 或计数变化，现有 `Snapshot`／`RestoreInteractive` 可直接保存完整 reset 前后半帧。

真实反例的原顺序是先完成 DAC 写 `7183080`，再调用 `fm_reset(7183006)`；chip tick `7183008` 已经被计算。正确顺序的 12 帧、8,832 个立体声采样及已记录 DSP 状态完全相同；按 raw cycle 排序的首差为 `frame=8, first_byte=56`，两个索引均从 0 计数。完整验证见 [研究记录](../../../../docs/research/md-yuyuhakusho/FMPSG_RESET.md)和 [验收索引](../../verification/fmpsg-reset/validation.json)。
