# PCM 命令语义设备

本包从 empty/idle 状态接收上层已接受的普通或音乐 DAC intent，以及外部明确给定的 bus hold／DMA 窗口，独立推进启动、预装、描述转交、播放、替换和自然结束。

```go
config := pcmcommands.Config{
    Resources: pcmcommands.Resources{
        Identity: catalogIdentity,
        Length: payloadLength,
        ByteAt: payloadByte,
    },
    Descriptors: descriptors,
}
device, err := pcmcommands.New(config)
frame, err := device.AdvanceFrame(pcmcommands.FramePlan{Inputs: inputs})
checkpoint := device.Snapshot()
restored, err := pcmcommands.Restore(config, checkpoint)
```

`Resources` 借用不可变目录；设备只保存 descriptor／payload ID，没有 encoded byte 副本。创建时校验资源完整性和 ID。普通目标及 `StartDual` 来自既有仲裁器的 sampled decision；音乐 intent 已经过上层接受判断。设备不重新选择优先级或目标。`Status` 提供同一 canonical 状态的临时视图，`Blocked/Priority/PanRedirect` 供 score 使用。

## 时间与输入

`AdvanceFrame` 每次推进一个 896,040 master-cycle 的固定参考 frame。输入 `Begin/End` 是同一绝对执行时轴上的 bus request／release。请求先让正在进行的原子语义操作完成，release 直接对齐至 `ceil(End/15)*15`；跨 frame 持有保留原阶段。

每次 bank／metadata／dummy 访问在自己的语义发生处调用 `phaseclock.ResolveBankAccess`。返回事件的 `Cycle` 是 handler 入口，`StartCycle` 是原子语义操作开始。bank 事件的 `Bus` 记录该次参考 effect；后续播放器时间已包含等待，调用者不要重复添加。完整 B6 byte 保留在 pan 事件中，DAC-only DSP adapter 按自身合同取高两位。

每个 frame 输入和 DMA 区间最多各 64 条。错误输入、重叠／过期窗口、资源或时刻错误均不提交部分状态。当前 opaque-resource profile 对下列元数据交叉明确拒绝：single 已锁存 header、bank 字段尚未读完时再发布描述；同槽转交已读 bank、尚未读完 size 时覆盖该槽。该限制不等于已经恢复任意窗口下原 ROM 地址的混合读取。

## 状态归属

快照包含 named audio phase、固定延迟分区中的 part 和有界循环剩余量。它们只描述本包已写出的 Go 音频语义函数；没有 CPU、任意 PC、通用寄存器、指令、固件、RAM 镜像、隐藏协程栈或历史重放。

`Frame` 是已经完成的参考 flush 数，`Time` 是播放器执行 frontier；持总线期间二者可独立变化。`Single`／两个 `Voice` 保存真正的源、预取、消费与累加状态。双槽 `Produced` 在一个部分预取阶段中尚未提交，独立于已完成的 bank 读取数量 `Fetched`。`Work` 只保存跨原子操作的必要 latch 与尚未写出的当前字节混音结果，完成对应阶段后清理。原资源数组、派生缓冲地址及原 RAM 均不保存。

暂停只持有总线。`02/03/04` 和音乐 track-end 不映射为 PCM stop。硬复位与更完整控制 ledger 正在独立扩展，原固定 12 组向量保持不动。

## 验证

初始 12 组 original-only 连续向量覆盖 idle 启动、两槽方向、追加、替换、两向模式切换、音乐协议和跨 3 frame 的暂停。每个 frame 都执行 JSON `Snapshot→Restore`，并比较原声音／总线事件、反馈状态、grant／resume、实际 PCM 和完整 DAC DSP 状态。

测试先于实现取得缺失 API 的实际编译 red。移植期间漏计双槽 Control64 路径的 10 T-state 跳转，坏版源和原失败保留。边界复核的拒绝测试也先于对应修复，原向量未放宽。

```sh
TEST_TELEMETRY_DIR="$PWD/.local/go-telemetry" \
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -json -count=1 ./games/yuyuhakusho/audio/pcmcommands
```

给定的 typed 命令与外部总线窗口是本组输入。世界 producer、IRQ、完整 FM／PSG、平台播放和真实扬声器仍有独立验收边界。
