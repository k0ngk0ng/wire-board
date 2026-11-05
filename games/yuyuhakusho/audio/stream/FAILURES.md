# 纯 Go 无损音频分片编码器的失败场景

本清单先于本包测试和实现。输入为 44,100 Hz、双声道、16-bit 的 interleaved PCM；输出为 FLAC verbatim frame 和 FLAC／fMP4 片段。编码器不是合成器，不执行原版 CPU、ROM 或固件，不依赖运行时 FFmpeg，也不创建播放器或 AudioContext。

## 格式与字节

- 不支持的采样率、声道或位深被默许，左右声道交换，负数或最值在端序转换时损坏。
- STREAMINFO 最小／最大 block 非法；variable-block 策略却声明 min=max；未知样本数或 MD5 被填成错误的已知值。
- variable-block header 使用普通 Unicode UTF-8，导致 21～36 bit sample number 截断；符号、保留位、block-size hint、CRC8／CRC16 范围或多项式错误。
- verbatim 子帧误用 mid-side、wasted bits 或压缩器分支，原 PCM 不再逐样本相等。
- 1～15 sample 的短块在流中间发出，或末尾短块被补零、丢弃、重复。65535 合法上限与 65536 禁止值混淆。

## 连续计数与 owner

- 不同 Append 分块导致丢样、重样、改序、不同字节输出，或 `Finish` 重复发出尾部。
- 原输入、InitSegment 返回值或已返回片段被调用者修改后，影响编码器内部状态或后续输出。
- 持久状态同时保存输入总数、pending frame 数、block 序号或 FLAC sample number，形成重复 owner；这些数必须由唯一输出 sample cursor、pending PCM 长度与 next sequence 派生。
- 非法 PCM 长度、单次输入容量、sample number 上界或 fragment sequence 溢出，在已经消费输入、改状态或返回部分片段之后才报错。
- 36-bit FLAC 起点、64-bit tfdt、uint32 fragment sequence、box 长度、sample count、signed data offset 或 Go/Wasm 的 int 发生截断／回绕。
- 关闭后的 Append 仍推进；空输入意外发包或改变游标；重复 Finish 不幂等。

## ISO BMFF 与实际承载

- Init 缺少 ftyp／moov／mvex、fLaC／dfLa 或必要空 sample table；时间基不为 44100。
- moof 缺少 tfdt v1，trun 缺少相对 data offset，mdat 未包含全部 FLAC packet，或时间戳存在缝隙、重叠和倒退。
- tfdt 时间与 FLAC sample number、trun duration、片段语义字段或真实 PCM 长度不一致。
- 依赖主机时间、指针、平台端序、随机 ID、外部编码器或 cgo，使 native／Wasm 字节不同。
- 只由自写解析器通过，未用独立 FFmpeg 回读和真实原生 MSE 播放；仅 MIME 声明可用就宣布承载完成。
- 为通过测试改成 AAC／Opus、交叉淡化、重建 audio 元素或重置 DSP 相位；把有限编码器／载体验收等同无限 BGM 和低延迟动态混音完成。

同一 Encoder 由一个调用方串行独占；平台 owner 队列负责串行化调用，本包不引入并发重排。非法调用必须原子拒绝。配置中的容量是显式边界，不能以无限增长的 pending 缓冲代替流式推进。
