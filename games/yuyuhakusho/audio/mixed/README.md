# PCM 与 FM／PSG 组合运行层

`mixed.Runtime` 将独立 PCM 协议的实际地址／数据端口输出送入 FM／PSG 合成器，返回连续的 44,100 Hz 立体声 PCM16。它借用不可变音频资源，分别持有一份 PCM Device 状态和一份 chip／FIR 状态。

```go
config := mixed.Config{PCM: pcmConfig, Chips: chipConfig}
runtime, err := mixed.New(config, initialChipState)
frame, err := runtime.AdvanceFrame(acceptedPublicationWindows)
// frame.PCM is interleaved stereo; frame.Protocol is returned diagnostic output.
saved := runtime.Snapshot()
restored, err := mixed.Restore(config, saved)
```

PCM 配置必须启用 `IncludeYMAddresses`。组合层按 Device 输出顺序提交真实端口，不合成缺失的地址，不排序、不合并写入。原硬件复位只重置 YM 域，保留 PSG、共享地址锁存和 FIR 副效。

`RunUntil`、`RequestBus`／`RequestBusWithDMA`、`BusGranted`、`Mailbox`、`PublishAccepted` 与 `Status` 对接实际 PCM Device；`WritePort`、`WritePSG` 与 `ReadStatus` 对接真实 chip。Runtime 可直接作为 `scoreclock.ServeYM` 的 Bus 和 Chip。辅助设备产生的物理输出在对应 bus 操作返回前提交，Busy 与 mailbox 继续由各自 owner 现场读取。

上述带时刻参数的接口使用 PCM Config 的坐标，包含其 `Origin`。组合层减去该不可变原点后交给 chip；chip 的 `MasterOrigin` 保留原采样网格相位。Runtime 不另外保存主时钟。

`FrameBoundary` 只 flush 两个 owner。调用前必须显式将 PCM 推进到当前参考帧末，或保持实际硬件 hold。`AdvanceFrame` 则使用 caller 提供的 typed 输入窗口推进 PCM 后 flush。两者均不为世界对象或音乐 driver 推造触发时刻。

所有可能修改状态的公开操作先建立工作分支，成功后整体提交。任一 owner 拒绝输入时，丢弃整个分支并返回零值输出。分支通过 `pcmcommands.Device.Fork` 借用已认证资源，避免每次 bus 访问重新读取、散列全部样本。

快照保留两个 owner 的 canonical 动态状态、不可变 chip 配置绑定和一个 `LastPCMWasDAC` 标志。该标志仅记录当前 chip Cursor 的因果来源，用于已证明的 DAC 后有界复位；复位时刻仍读取唯一 chip Cursor。普通 chip 访问会清除该标志，不能借此放宽普通写入的单调性。快照不保存已消费 ledger 或历史音频块；恢复时校验配置、资源身份、参考帧和各 owner 的有效状态。

原版夹具、先行失败和重复验证记录位于 [audio-mixed-runtime](../../verification/audio-mixed-runtime/)。复跑入口为 `scripts/yuyuhakusho/verify_mixed_runtime.py --mode native|wasm|vet --out <仓库内独立目录>`；离线审计使用 `--audit`。已有结果不可覆盖。

本层提供生产可用的 PCM→chip 组合和恢复接口。完整音序成本、世界／IRQ 排程、音乐控制协调及平台 `openPCMStream` 接线仍需后续集成；本层通过不代表完整对局音频已验收。
