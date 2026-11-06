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

当前设备只接受 `Remaining == 0` 的 DMA 环境；更完整的外部 wait／clear 域尚未接通。每次 bank／metadata／dummy 访问在自己的语义发生处调用 `phaseclock.ResolveBankAccess`。返回事件的 `Cycle` 是 handler 入口，`StartCycle` 是原子语义操作开始。bank 事件的 `Bus` 记录该次参考 effect；后续播放器时间已包含等待，调用者不要重复添加。完整 B6 byte 保留在 pan 事件中，DAC-only DSP adapter 按自身合同取高两位。

每个 frame 输入和 DMA 区间最多各 64 条。错误输入、重叠／过期窗口、资源或时刻错误均不提交部分状态。当前 opaque-resource profile 对下列元数据交叉明确拒绝：single 已锁存 header、bank 字段尚未读完时再发布描述；同槽转交已读 bank、尚未读完 size 时覆盖该槽。该限制不等于已经恢复任意窗口下原 ROM 地址的混合读取。

## 状态归属

快照包含 named audio phase、固定延迟分区中的 part 和有界循环剩余量。它们只描述本包已写出的 Go 音频语义函数；没有 CPU、任意 PC、通用寄存器、指令、固件、RAM 镜像、隐藏协程栈或历史重放。

`Frame` 是已经完成的参考 flush 数，`Time` 是播放器执行 frontier；持总线期间二者可独立变化。`Single`／两个 `Voice` 保存真正的源、预取、消费与累加状态。双槽 `Produced` 在一个部分预取阶段中尚未提交，独立于已完成的 bank 读取数量 `Fetched`。`Work` 只保存跨原子操作的必要 latch 与尚未写出的当前字节混音结果，完成对应阶段后清理。原资源数组、派生缓冲地址及原 RAM 均不保存。

`Hold` 仅表示硬件总线持有；自然游戏暂停的外部 producer 另由世界层安排。`02/03/04` 和音乐 track-end 不映射为 PCM stop。`HardReset` 保留 assert／release 两次 reset 和 typed boot；`SetRestorePan` 只改变之后自然结束的恢复 byte。

`Frame.Ledger` 按实际执行顺序返回声音／bank／YM data／reset 事件。`Frame.Events` 是该 ledger 的六类兼容临时视图。reset 的 raw cycle 可以小于前一个已完成 DAC 的 raw cycle，调用者不得排序或 clamp。默认输出视图维持 data／control／reset 合同。`Config.IncludeYMAddresses=true` 时，ledger 另包含真实 `YMAddress` port0／2 写入，data 的 `Event.Port` 为真实 port1／3。视图绑定 checkpoint identity，不能静默切换。

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

控制扩展另保存 20 组 original-only 连续向量和 2 组独立 reset 补充场景。expanded20 的 312,410 个 data／control／reset／bank 事件全部匹配；17 个非 reset 场景有 802,191 个立体声 PCM 帧及完整 DSP 状态匹配。reset raw 逆序先按执行顺序比较事件／状态，旧 DAC Queue 的单调域不被放宽。原生 race 与 Wasm 各 9 项顶层检查、50 个子场景通过。历史源在 `verification/audio-pcm-command-go/v1-source` 与 `v2-source` 保留。

## 分段总线服务

`RunUntil(at, dma)` 只推进当前参考 frame 的真实设备语义，不自动 flush；返回当前辅助 frontier、本次新 bank contention 对主时钟的 effect 及事件数量。外部 main clock 唯一拥有自己的时间，不能把 after-main 值回填为辅助预算。

`RequestBus/RequestBusWithDMA` 实现一个硬件 level：首次 assert 才推进至原 budget，返回 `at + 本次新 M68KDelay`；重复 assert 不推进、不加引用计数。at 可略早于已经完成的原子 frontier。release 直接 ceil15 对齐；未 held 的 release 不推进。`BusGranted` 读实际 level，`Mailbox` 在真实 held 下读 `Global.Busy`，没有额外 7-cycle 访问成本。

从空闲提交的顺序为 `RequestBus(0,true) → PublishAccepted(intent) → RequestBus(0,false)`。随后先真实 `RunUntil(origin)`，把 `DrainLedger()` 输出交给同一芯片 owner，再调用 YM 服务。服务每次实际 Request／RunUntil 后都应及时 Drain，不能等帧末补播旧输出。

`PendingLedger` 是唯一有界未消费队列，最多 4,096 笔；快照深拷贝 event 与 bus effect，Drain 转移并清空。它不保留已消费历史。`Held.Open` 表示外部掌握 release 时点的实际硬件持有，不用伪造无限 End。`FlushFrame()` 明确提交边界，不推进 CPU；只有已经跑到 frame 末或持有硬件时才能调用。

17 组原版非 reset 流通过这个分段接口再次执行，272,526 个原事件、原 mailbox、grant／resume 与 frame 状态相等。持有和 partial-frame 状态经 JSON 恢复保持相等。源码与结果在 `verification/audio-pcm-command-go/validation-bus.json` 绑定；真实 ServeYM 主总线／芯片连接另由 `verification/score-clock-bus` 独立验收。

## 真实 YM 端口视图

设置 `IncludeYMAddresses=true` 后，每次地址在现有固定语义 part 的实际写点输出，阶段时钟没有改变，也不在 PCM 设备保存芯片地址 shadow。single 的地址到 data 为 17 T-state，dual 为 11 T-state；启动、结束和 boot 使用各自实际间隔。完整芯片 adapter 应按 ledger 顺序直接 `WritePort(Cycle,Port,Value)`，不在 data 时刻补地址。

23 组独立 original-only 向量的 519,256 个实际 address／data／bank／reset 事件、1,274 个 frame 的原地址 latch、反馈状态和 bus 事务相等。新 Go E2E 使用实际 YM 端口实现验证 latch；reset 部分只验证原 warm-reset 保留 address 的投影，不扩大成完整 FM／FIR reset 证明。data-only 补地址方案在中断场景原 frame0 的 `2A` 与错误 `2B` 有真实 red。

新输出视图与旧 v1／v2／Bus 数据视图同时回归；冻结索引见 `verification/audio-pcm-command-go/validation-ports-v3.json`。
