# 原始 DPCM 资源目录

`pcmassets` 从原样普通资源 JSON 和现有 score JSON 建立一个不可变的 DPCM byte owner。普通文件只保存 123 个 payload；音乐的 6 个 payload 继续只存于已冻结的 score 资源，解码时借读并合并。库不保存源 ROM 地址、固件、CPU、原始 RAM、播放游标或 DSP 状态，不主动下载资源。

```go
catalog, err := pcmassets.Decode(bytes.NewReader(ordinaryJSON), scoreJSON)
descriptor, ok := catalog.Ordinary(code)
payload, ok := catalog.Payload(descriptor.SampleID)
length := payload.Len()
value, known := payload.ByteAt(offset)
```

调用方先用可信构建清单核对两份资源身份。`Decode` 校验普通文件的闭合注册表及 score 原样 JSON 摘要；score 自身的非 PCM 规则仍交给 `score.DecodeCatalog`。两次 decoder 可以读取同一份临时 score JSON，完成后释放；score 包不持久保存 encoded bytes，`pcmassets.Catalog` 是这 129 个样本的唯一运行时 byte owner。

| API | 合同 |
| --- | --- |
| `Ordinary(code)` | 仅接受 `06..7e` 和已证明的 `f0` 扩展映射，共 122 个命令。 |
| `Diagnostic(0x7f)` | 独立的单字节 InternalSilent 入口，不加入有限 WAV 播放目录。 |
| `Music(descriptorID)` | 使用 score 原有的 6 个 descriptor ID；不重新计算另一套音序 ID。 |
| `Descriptor(id)` | 返回值副本，字段为 ID、SampleID、NativeRate、Flags、Protocol、RoutingOrPan。 |
| `Payload(sampleSHA)` | 返回私有数组的只读 handle，提供 `ID`、`Len` 和 `ByteAt`；`Bytes` 仅返回显式副本。 |
| `PayloadLength` / `PayloadByte` | 为命令设备提供无状态函数适配；设备只保存 descriptor／sample ID。 |
| `Counts` | 返回 ordinary、music、diagnostic、unique payload 数量。 |

普通 `Protocol=OrdinaryPCM` 的 `RoutingOrPan` 是八字节 producer 的第七字节。音乐 `Protocol=TrackPanPCM` 的同字段为 0；实际声像来自当次 track intent，不能用这个 0 把声道静音或当作 ordinary mode。

`NativeRate` 是原播放器的延迟计数字节，0 对应 256 次循环，由命令时基层解释；它不是 Hz 或 WAV 采样率。

`ByteAt` 越界返回 `known=false`。返回 byte 的零值不代表原 ROM 读到了零。`7f` 声明长度为 1，但原启动仍可能进行 128-byte 预取；命令设备保留这些总线操作，目录不盲目复制相邻 ROM，也不为未消费的预取 byte 造零填充。

完整资源与原 codec 母表的 437,940 个编码字节／875,880 个解码值已对照。Native race 和 Wasm 的证明摘要相同；该结果不包含全部样本的新原生播放、命令时钟、混音、扬声器或平台声音接线。证据与复跑见 [AUDIO_PCM_PAYLOADS.md](../../../../docs/research/md-yuyuhakusho/AUDIO_PCM_PAYLOADS.md)。
