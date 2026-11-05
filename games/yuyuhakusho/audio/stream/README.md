# PCM 无损 FLAC／fMP4 编码器

本包将 44,100 Hz、双声道 signed16 PCM 编码为可供原生 MSE 消费的 FLAC／fMP4 片段。只依赖 Go 标准库，不读取 ROM，不执行 CPU／固件，不调用 FFmpeg，不创建播放器或 AudioContext。它不负责音序、DSP、游戏混音、平台生命周期或网络同步。

## 接口

```go
config := stream.DefaultConfig()
encoder, err := stream.New(config)
init := encoder.InitSegment()
segments, err := encoder.AppendPCM(interleavedPCM)
tail, err := encoder.Finish()
status := encoder.Status()
```

每个 PCM sample frame 包含左、右两个 `int16`。`Segment` 提供 `Sequence`、`StartSample`、`SampleFrames` 和独立的 `Data`；sequence 从 1、sample 从 0 开始。`InitSegment` 和输出片段由调用者持有，修改它们不影响编码器后续状态。调用同一 Encoder 须串行化。

默认 block 为 1,024 帧，fragment 为 4,096 帧，单次 Append 上限为 65,536 帧。格式严格为 44100／2／16；block 接受 16..65535，fragment 必须为 block 的正整数倍且不大于单次输入上限。单次输入上限可配置到 1<<20，避免跨 native／Wasm 的无限分配。

相同配置和 PCM 在不同 Append 分块下输出字节完全相同。改变 block／fragment 配置时，容器字节会改变，解码 PCM 保持相同。只有 Finish 可发出短于 block 的尾部，包括 1..15 帧。重复 Finish 不重发；关闭后的 Append 拒绝。

错误在提交前检查，非法 PCM 对齐、容量或计数不消耗输入、不改变 pending 和游标，也不返回部分片段。sample index 必须小于 1<<36；末端允许为 1<<36。mfhd 写入 uint32，内部 next sequence 可持有用尽后的哨兵，以提前拒绝未来无法编码的短尾。

## 状态所有权

持久状态只有 immutable config、pending interleaved PCM、已输出 sample cursor、next sequence 和 closed。`Status.BufferedFrames` 从 pending 长度／2 派生，`Status.AcceptedSamples` 从输出 cursor+pending 派生。FLAC sample number 从当前 cursor 和片段内偏移派生，没有第二套输入总数、block 序号或 FLAC 时钟。

## 格式

FLAC 使用 variable-block strategy、扩展到 36 bit 的数值 UTF-8，以及两个独立声道的 verbatim 子帧。样本按 signed16 big-endian 写入，CRC8／CRC16 多项式分别为 0x07／0x8005。STREAMINFO 的 minBlock=16、maxBlock=max(17,BlockFrames)；这是 RFC 9639 允许的预声明范围，避免 min=max 表示 fixed-block。帧字节上界、总样本和 MD5 的 0 表示未知，不在流式输出后回填。

Init 是 `ftyp+moov`，包含单一 `fLaC/dfLa` 音轨和 `mvex/trex`。每片为 `moof+mdat`，设置 default-base-is-moof、tfdt version 1、显式 trun duration／size／data offset，PTS=DTS，时间基为 44100。配置容量保证最大 box 和 signed data offset 位于可表示范围。

平台使用 `stream.MIMEType`（`audio/mp4; codecs="fLaC"`）、SourceBuffer segments 模式和 timestampOffset=0。平台管理媒体元素、MSE、手势、静音、暂停、owner、离桌和 endOfStream；本包只生成字节。

平台单片上限为 1 MiB，与编码器通用配置的上界分开。默认配置的保守片段上界为 16,584 bytes，小配置 256／1024 为 4,296 bytes，均可适配。通用配置不自动获得平台支持：适配器在消费 PCM 或打开平台流之前，必须检查 `96 + 4*FragmentFrames + 26*ceil(FragmentFrames/BlockFrames) <= 1 MiB`，否则拒绝该配置。编码器最大合法配置仍可用于独立导出。

## 延迟与范围

默认 4096／44100 约为 **92.88 ms** 的编码聚合跨度，尚未包含输入批次、MSE、设备和播放缓冲延迟。较小配置 256／1024 约为 23.22 ms；这些参数可用于后续真实延迟测量，不能据此宣称低延迟动态混音已通过。

当前验收包含 native race、Node／Wasm、独立 FFmpeg 回读，以及原生 MSE 对 Go 输出的实际播放。平台还已用 517 个小片段验证逐段供给、516 个真实边界和限定生命周期；最终绑定见 [VERIFICATION.md](VERIFICATION.md)。编码器不等于独立 DSP 或无限 BGM；Safari／手机、游戏恢复和动态混音响应仍按各自范围验收。先行失败范围见 [FAILURES.md](FAILURES.md)。
