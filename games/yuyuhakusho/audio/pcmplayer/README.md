# 已初始化 PCM 播放器

本包从明确的 typed 初态、自身上一步状态和不可变 DPCM 资源，独立生成 DAC、bank 预取、dummy 读取、恢复声像和关闭 DAC 的有序事件。生产仅有 Go 算术与语义阶段；资源使用 encoded SHA-256 身份，没有 CPU、固件、任意 PC、原时间表或 RAM 镜像。

```go
player, err := pcmplayer.New(pcmplayer.Config{
    Mode: pcmplayer.Dual,
    Samples: [2]*pcmplayer.Sample{sampleA, sampleB},
    Origin: 0,
    RestorePan: 0,
})
for !player.Stopped() {
    events, err := player.Next()
    // Consume ordered events; a semantic byte stage returns at most six.
}
saved := player.Snapshot()
resumed, err := pcmplayer.Restore(config, saved)
```

## 初态合同

- `Dual` 从两个空缓冲、已有描述已转交、活动槽尚未启用解码的初态开始。至少有一个资源；单槽长度为 69～32,768 字节。另一个槽可以为空。
- `Single` 从原 128 字节预装完成的初态开始，只使用 slot 0，长度为 129～7,788 字节。该 profile 的 dummy 游标从 bank 窗口起点开始；超出本合同的长度明确拒绝。`Rate == 0` 使用 256 次延迟循环。
- `Origin` 是调用者明确给定的参考 master-clock 起点。`RestorePan` 支持 `00/40/80/C0`。调用者同时提供已启用 DAC 的芯片初态。
- 该有限 profile 没有外部总线暂停或 DMA。每次 bank／dummy 读取都包含固定参考核的 45 master-cycle 等待。命令启动、描述发布、抢占、暂停及环境握手仍需独立实现。

`Next` 原子地提交一个字节的高／低输出阶段，或最后的高半段及恢复／关闭序列。它允许在字节边界保存恢复；没有提供字节中间插入命令的接口。非法构造、恢复和停止后的推进均不提交部分状态。

## 事件和时钟

`Cycle` 是参考 handler 的调用时刻，`StartCycle` 是产生这次访问的语义内存操作起点。两者都使用同一 master-cycle 轴。按参考 frame 准入时使用 `StartCycle`；一次在 frame 末前开始的操作可以在 frame 末之后才写 DAC。

- `DAC`：`Value` 为滤波前 8-bit DAC 值。
- `Bank`：`Slot/Offset` 指出样本资源的顺序读取。尾部预取可超过有效 payload 最多 3 字节；这些超前读取只影响总线时序，不从资源外取值或进入解码。
- `Dummy`：`Offset` 为本单槽 profile 的 dummy 游标。dummy 返回字节不影响输出，只产生总线读取。
- `Pan/Enable`：结束时依原顺序恢复声像、关闭 DAC。

`Bank/Dummy` 后续的 45-cycle 等待已经计入播放器时间。下游可以据事件计算并行 68k 的参考等待，不能再次给播放器重复加 45。外部 DMA 必须在未来的可中断语义阶段内部生效，不能把整个字节统一平移后当作已恢复。

## 状态归属

`State.Time` 是已提交语义阶段末尾的唯一时钟。`Single` 与 `Dual` 两种状态互斥。双槽保留源预取数量、已解码数量、累加器、解码开关和活动生命周期；单槽保留预取／消费数量、累加器与有独立后续作用的 dummy 游标。缓冲低字节游标、结束指针和单槽尾部阶段由这些量派生。

资源只读复制一次。没有持久化环形缓冲副本、原 RAM、寄存器、历史输出或声音时间表。`Snapshot` 与构造／恢复均隔离调用者可变对象。

## 验收

原生竞态与 Node／Wasm 各 6 项顶层检查、37 个子场景通过。12 组私有原生播放器的 70,856 个有序声音／总线事件相等；自主播放器接独立 DAC DSP 后，302 个参考帧、222,258 个立体声 PCM 帧及每帧完整 typed DSP 状态相等。

这些原生输入明确从初始化后开始，并关闭世界／68k 执行。完整游戏的启动、替换、DMA、总线握手、世界 IRQ 和 FM／PSG 混合仍在后续范围。证明边界和复跑说明见 [研究报告](../../../../docs/research/md-yuyuhakusho/AUDIO_PCM_PLAYER.md)；机器索引见 [validation.json](../../verification/audio-pcm-player/validation.json)。

```sh
TEST_TELEMETRY_DIR="$PWD/.local/go-telemetry" \
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -json -count=1 ./games/yuyuhakusho/audio/pcmplayer
```
