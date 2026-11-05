# Typed 音序接口

本包支持 schema 2 的 23 个已恢复音乐／primary effect 定义。`DecodeCatalog` 消费解压后的 JSON；正式资源为 `data/audio-score.json.gz`。数据、Root dispatcher、PCM feedback、chip synthesis 和媒体播放分别由对应 owner 管理。

```go
catalog, err := score.DecodeCatalog(decodedJSON)
driver, err := score.New(catalog)

output, err := driver.Advance(score.Input{
    SoundID: acceptedScoreID, // 0 means no score dispatch in this tick.
    DAC: score.DACFeedback{
        Blocked:     mailboxHighBit,
        Priority:    modePriority, // 0, 64, 128 or 192.
        PanRedirect: mailboxNonzero,
    },
})

saved := driver.Snapshot()
restored, err := score.Restore(catalog, saved)
```

`Advance` 每次推进一个实际 driver call，包含本次 Root 接受的 dispatch。它不假设固定视频帧率，也不自建 queue。Root 暂停／调度与 catalog 身份绑定由调用者管理。未知 dispatch、非法反馈、无效恢复或运行错误不提交部分 score 状态和输出。

| 输出 | 字段与消费者 |
| --- | --- |
| `ym` | `Bank` 为 0／1，`Register` 与 `Value` 为芯片有效寄存器请求。时基层赋予 cycle 后交给 YM `WriteRegister`。 |
| `psg` | `Value` 为一个有序 PSG latch／data byte。 |
| `dac` | `DescriptorID` 与 `Pan`。使用 `catalog.Descriptor(id)` 得到 sample 内容 ID、native rate、flags 与 routing，再由 PCM owner 处理发布。 |
| `dac-pan` | `Pan`。更新当前 DAC 的声像请求；不等于重新开始样本。 |
| `queue` | `Action="replace-first"`、`SoundID`。Root 覆盖 queue 第一槽。 |
| `ClearPrimaryPriority` | 任一 primary track 停止时即返回 true，也可能在音乐初始化清理旧效果时返回 true。Root 应更新其唯一 priority owner。 |

`Output.Intents` 的数组顺序就是提交顺序；`Order` 与数组下标一致。不要按寄存器、声部或类型重新分组。`Value` 的有效字段投影不包含已证明的硬件忽略位；TL 高位影响 driver carry，已在 typed patch 中单独保留。

`DACFeedback` 是外部 PCM owner 在相应读取阶段的只读 view，不进入 `State`。`Blocked` 只对应 mailbox high bit，`PanRedirect` 对应整个 mailbox 是否非零，两者不能合并。Root 需要在自己的时序层保持这两个读取阶段的实际关系。

`State` 保存 typed voice 参数、计数、envelope／pan 位置和一份语义事件位置。`Snapshot` 与 `Restore` 不共享调用者可变 slice；`View` 是派生的规范化观察，不是另一份可写状态。Root 仍需用自己的资源身份校验将快照绑定到相同 catalog。

完整原版轨迹、先行失败、可重复验收和剩余边界见 [AUDIO_SCORE_DRIVER.md](../../../../docs/research/md-yuyuhakusho/AUDIO_SCORE_DRIVER.md)。该包尚不提供精确 master-cycle、DSP、group2 或平台播放。
