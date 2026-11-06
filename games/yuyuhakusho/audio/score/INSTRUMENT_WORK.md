# 乐器内容与查找工作的统一引用

`BindInstrumentWork` 将经过认证的、与原 score JSON 精确绑定的不可变乐器元数据应用为新目录视图。它不修改旧目录，不复制音色 payload，也不新增动态音色／原 instrument index／RAM 状态。`VoiceState.Tone` 仍是当前选择的唯一 owner；在 bound 目录中保存逻辑乐器引用，音色内容 ID 由目录派生。

```go
bound, err := score.BindInstrumentWork(catalog, metadataReader)
driver, err := score.New(bound)
work, ok := bound.Instrument(driver.Snapshot().Voices[slot].Tone)
// work.PatchID identifies shared sound data; work.RecordsSkipped is immutable seek work.
```

资源包含 106 个 sound／work 定义、68 个初始选择和 892 个控制选择。逻辑 selector 使用 score／voice／initial|prefix|cycle／事件索引；不含源 bank、PC、地址或原返回栈。内容相同、查找工作不同的条目拥有不同引用；音色和工作完全相同的条目可以共享。

定义引用由音色 ID 与有界 `records_skipped` 的内容摘要生成。该工作量是不可变资源属性，不作为原索引再存进 VoiceState。目录绑定逐项验证原音色、逻辑覆盖、唯一性、fallback 与 score 内容 SHA，失败不返回部分目录。仅保持原始 JSON 身份并不能认证下载，外部加载层仍须验证可信清单。

native `View` 派生原音色 ID，因此原轨迹与声音结果不变。bound Snapshot 保留具体选择引用；恢复到旧目录或拿旧 Tone 直接恢复到 bound 目录会拒绝，避免猜测别名。跨目录完整恢复仍由外层资源身份负责。

统一加载使用 `resourcebridge.DecodeWithInstrumentWork(scoreJSON, pcmJSON, parserJSON, instrumentJSON)`，第四份资源参与整体身份和 PCM checkpoint 身份。三参数旧入口继续提供原目录，不静默改变旧存档含义。新资源使用 gzip 无损封装，原 score／PCM／parser 字节不改写。

此项补齐乐器选择身份和查找工作信息；`OpResourceUnavailable` 在完整参数／传送时序尚未组合前继续保留。它不宣称整场音频已可播放。原版轨迹与可重复验证见 [audio-instrument-work](../../verification/audio-instrument-work/)。
