# 帧内即时读写接口

`Interactive` 让声音时基调用者在同一帧读取 YM status，依据刚返回的 busy bit 决定下一次轮询或写入。它与旧 `Stream.Queue`／`FrameBoundary` 共用一个帧事务；旧公开接口、原生 PCM 和已提交 `State` 保持兼容。

```go
audioDSP, err := chips.NewInteractive(config, committedState)
status, err := audioDSP.ReadStatus(cycle, 0)
// 调用者按自己的已验证语义成本移动 cycle，再读取或写入。
err = audioDSP.WritePort(cycle, 0, register)
err = audioDSP.WritePort(cycle, 1, value)
err = audioDSP.WritePSG(cycle, data)

saved := audioDSP.Snapshot() // 完整半帧状态，不是旧 State 的截断形式
audioDSP, err = chips.RestoreInteractive(config, saved)
frame, err := audioDSP.FrameBoundary()
committedState, err = audioDSP.CommittedState()
```

`Cycle` 相对 `Config.MasterOrigin`，使用与旧接口相同的 uint64 master 时间线。所有端口输入共用非降 raw cursor；同 cycle 依调用顺序执行。每帧最多 4,096 次读写，raw overrun 上限仍为 1,008 master cycles。调用者显式 flush 视频帧；API 不在 busy 循环中自动 flush，不把 cycle 改成帧尾，也不预看未来 deadline。

`ReadStatus(cycle, 0)` 先生成严格早于该 cycle 的 YM 采样，再返回芯片 status 和此刻 busy。port1／2／3 按固定离散参考模型返回零且不推进 FM。地址写只更新共享 latch，数据写先同步再写入；数据端口本身不选择 bank。忙标志仍由唯一芯片 owner 维护，调用者使用返回值，不保存另一份 deadline 或 status。

交互实例只拥有一个 YM 状态、一个 PSG 状态、raw cursor、当帧 FM 采样进度、busy 和一个当前 FIR buffer。FIR buffer 包含本帧尚未读取的 delta 及 18 个尾单元，取代旧 `State.Tail`；实例不持久保存 tail、旧帧初态、输入历史或已返回读值。`Snapshot` 和 restore 都深拷贝 FIR，普通读写不复制整块 FIR：先完成所有可能失败的候选芯片计算和边沿检查，再原子提交。

`CommittedState` 仅在尚未进行新帧第一次读写时可用。半帧调用会明确失败，避免把当前 FIR 截成 18 项而丢失声音。`NewInteractive` 拒绝带未消费 `Pending` 的旧 `State`；旧队列先按原接口 flush，再于帧边界切换。已返回的 status 不进入 `Frame.Reads`；旧队列模式仍按原顺序在 `Frame.Reads` 返回其排队读值。

FM raw lookahead 已推进的 oscillator／envelope／timer 保留，超前输出仍从参考 FM 帧输出中丢弃。PSG ahead 边沿留在 FIR 尾部。flush 后可从先前 raw cursor 的同一 cycle 或更晚 cycle 继续，不能回到已提交超前输入之前。

实例不增加一个任意的 `AdvanceTo`。时基调用者只移动自己的语义成本游标，在实际端口读写或显式帧尾推进 DSP；额外 YM update 分块可能改变参考 timer-B 的可观察边界。

验收覆盖 5 组原生场景、1,109 次交互、13 帧，以及每步完整半帧快照恢复。Native／race／Wasm／vet 通过，旧 raw YM、混合 PCM、PSG 与 DAC oracle 全部回归。详见 [研究记录](../../../../docs/research/md-yuyuhakusho/FMPSG_INTERACTIVE.md)及 [验证索引](../../verification/fmpsg-interactive/validation.json)。本接口没有恢复原 driver 的 CPU 语义成本，也没有独立决定世界声音事件时刻。
