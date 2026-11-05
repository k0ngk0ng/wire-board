# DAC-only 参考帧 DSP

本包只支持 `ReferenceFrameDiscreteHQ44100`：原配置的离散 YM2612（原 enum 值为 `0`）、HQ 44.1 kHz、preamp100、filter0 和 mono0。它把给定的 typed DAC／enable／pan 输入变为 PCM16，不执行 CPU、固件、样本选择、FM／PSG 或独立音频事件时钟。

```go
stream, err := dsp.New(
    dsp.Config{Profile: dsp.ReferenceFrameDiscreteHQ44100, MasterOrigin: origin},
    initialTypedState,
)
// Queue may receive several packets from the same reference frame.
err = stream.Queue(events)
interleavedPCM16, err := stream.FrameBoundary()
saved := stream.Snapshot()
```

`Event.Cycle` 相对 `MasterOrigin`。相同 cycle 保留传入顺序；`Queue` 只复制当前帧尚未消费的输入，不提前修改 DSP。`FrameBoundary` 每次提交 896,040 master cycles，输出 735／736 个立体声采样帧。输入可分批或中途保存恢复，不能把任意半帧提前当作参考 frame flush。

原参考 core 可能在一条帧内调用中把芯片算到帧尾之后，最终只 flush `chipTick<end` 的部分。这里保持原 raw event cycle，并明确丢弃 lookahead；寄存器的最新值保留给下一帧。`InputAhead` 保存独立 raw 输入游标超出已 flush 时间的量。它不是硬件最大延迟：此 profile 的有界输入合同为 1,008 master cycles、每帧最多 4,096 条 pending 输入。

`State` 只保存 Time、DAC register 语义、上一已提交 chip 输出、真正独立的 integrator／18-tap tail、raw 输入 ahead 和 pending 队列。fractional resampling offset 和下个 chip tick 由绝对时间推导；未启用的低通没有状态。快照及构造均深拷贝 pending，非法配置／参数不会部分提交。

原生与 Wasm 验收逐帧比较 9 组 native oracle、441 帧、324,556 个立体声采样帧及完整 typed DSP state，全部相等；包括相同 cycle 顺序、pan、帧尾／超帧写、半帧分批和恢复。详细边界与最初 raw-global-time 失败见 [DAC DSP 研究](../../../../docs/research/md-yuyuhakusho/AUDIO_DAC_DSP.md)；机器索引见 [validation.json](../../verification/audio-dac-dsp/validation.json)。FIR 数值 profile 来源见 [KERNEL_SOURCE.md](KERNEL_SOURCE.md)。

```sh
TEST_TELEMETRY_DIR="$PWD/.local/go-telemetry" \
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -json -count=1 ./games/yuyuhakusho/audio/dsp
```

这些测试给定原输入时刻，只证明 DSP 层。生产调用者仍需独立产生声音事件；本包不读取测试中的原 PC／时间表，也不表示完整游戏声音已接通。
