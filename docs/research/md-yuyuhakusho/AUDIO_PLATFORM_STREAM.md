# 平台连续原生无损流验收

平台 `openStream` 已在真实 Chromium 上接受纯 Go 生成的原 `82` 连续 PCM 分片，完成 12 秒、517 个小片段和 516 次实际跨界。载体保持同一个 HTMLAudio／MSE 来源，逐片追加、边播放边回收旧缓冲，正常播放后没有 waiting、stalled、seeking 或 error。

本验收证明所测浏览器中的平台流承载和生命周期，不证明独立合成器、原版任意混音、扬声器逐样本等价或最终交互声音延迟。原 `82` 的有限 PCM 是已知参考输入，不能把它当作已经解决未知未来效果声的动态混音。

## Public 合约

纯类型位于 [audio-stream.ts](../../../web/src/game-api/audio-stream.ts)。平台管理原生 HTMLAudio、MediaSource、SourceBuffer、ObjectURL 和宿主清理；游戏不创建自己的播放器或 AudioContext。

`openStream({ playbackID, mimeType, sampleRate, initSegment, maxForwardSamples?, retainBackSamples? })` 同步返回 owner／epoch 绑定的 handle，或在格式／状态不支持时返回 undefined。同 owner 和实例的归一化配置、init 完全相同，返回同一 handle；冲突拒绝，自然结束后相同实例不重播。新实例只替换旧 stream，legacy music／有限声道保持既有语义，模式切换由宿主明确停止旧适配器。

| 操作 | 成功含义 | 有界行为 |
| --- | --- | --- |
| `ready` | sourceopen、SourceBuffer 创建及 init updateend 完成 | 不等待原生 play，也不需要先有媒体样本 |
| `append` | 本片 updateend 后提交接收游标 | 一次只接收一个 in-flight；相同 pending 片返回同 Promise；不同并发片 false，不排队 |
| `end` | 当前片提交后 EOS 成功 | 锁住新片，重复 end 复用 Promise；EOS 后仍是 draining，播到尾才 ended |
| `stop` | 取消本 handle 的资源 | 旧 handle 不能停止继任 epoch；pending 统一 false，ObjectURL 撤销一次 |
| `status` | 当前只读快照 | 唯一 nextSeq／acceptedEndSample、当前 pending bytes、原生时钟与实际缓冲 |

媒体从 sequence 1、sample 0 开始。最后已接受片的元数据和 payload 精确重复返回成功但不再次追加；更早历史不保存，无界重放不可用。gap、倒序、相同序号不同 payload、非安全整数及溢出在写入前拒绝。caller 的 init 和 pending 字节在异步使用前复制。

容量为 init 最多 64 KiB、单片最多 1 MiB，默认 forward 66,150 samples（1.5 秒）、retainBack 88,200 samples（2 秒）。forward 可收紧但不超过 441,000；back 在 11,025～441,000 内。false／busy／backpressure 不预留序号，producer 重试同片而不推进游标。清理只移除已经过去的音频，不能移除全部当前缓冲。没有公开 seek 入口。

元数据校验不能证明 fMP4 内部真实性。编码器身份、容器样本时间戳和全部 PCM 值由 Go 编码器及清单另验；平台只消费已经验证的编码输出。

## 先行失败与实现时序

[失败清单](AUDIO_PLATFORM_STREAM_FAILURES.md)、public 类型和完整真实浏览器脚本先于平台 runtime。原生浏览器首先因缺少 `openStream` 退出 1；Root 的 8 项 FakeMSE 生命周期／容量测试也先取得缺实现 red。Root 随后实现 `web/src/audio-stream.ts`，并由 `GameAudio` 接入 owner、pause／resume、mute、stop 和 dispose。

原始 native missing red、冻结源码和 SHA 位于 `verification/audio-platform-stream/test-first.json` 及 `missing-final-red/`。原生测试首个已接入口运行通过 opening、字节所有权、序列和 single-flight，但在暂停 fixture 的固定 8 片启动处超时；该失败没有覆盖或删除。

## 启动阈值与固定 8 片的修正

独立 startup probe 比较 Go small256-block／1024-fragment 与 default1024-block／4096-fragment，并各测 `preload=none` 和 `auto`。两个 preload 设置的结果相同：

| 分片配置 | 尚未起播的缓冲 | 首次实际 playing 的缓冲 |
| --- | ---: | ---: |
| small，256／1024 | 8,192 samples，约 0.185758 秒 | 第 9 片后 9,216 samples，约 0.208978 秒 |
| default，1024／4096 | 8,192 samples，约 0.185758 秒 | 第 3 片后 12,288 samples，约 0.278638 秒 |

尚未起播时 `readyState=1`、native play Promise pending、时钟为 0，而 append 已正常提交。对 small 的 8 片前缀调用 EOS，会开始播放并走到 0.185758 秒尾部，进一步区分了有限输入完成与开放流启动门槛。small codec 本身另由大缓冲独立原型证明可播放。

启动后以 0.26 秒目标继续增量供给，独立 probe 的四秒实际前瞻约为 0.224～0.247 秒，没有后续 waiting／stalled／error。因此首个失败是生命周期 fixture 无依据地停止供给在 8 片；不是低前瞻主测试已经失败，也不是需要把主测试改成多秒预载。

只修正该暂停 fixture：在原 0.26 秒界内逐片供给，直到实际时钟前进。主 12 秒供给目标 0.26 秒、配置上限 0.5 秒、尾部和清理断言都没有放宽。原旧 harness、red、startup 结果及新 SHA 由 `bootstrap-lineage.json` 绑定。probe 自身曾有局部计时变量遮蔽函数造成的 TDZ，源码和终态失败另存，重命名仅涉及观察器。

## 当前完整原生结果

输入是 Go `small-bulk` 的 529,200 个 44.1 kHz 双声道 sample frames：516 个完整 1,024-frame 片和 816-frame 尾片。manifest、init 和每片 SHA、序号、样本范围在提供给浏览器前核对。Go 生成物的独立解码逐 byte 原 PCM 相等，证明层次与原生播放分别保存。

完整验收覆盖：opening 被替换、旧 handle／owner、init 和 pending caller mutation、配置冲突、无效元数据、同 pending Promise、最后片去重、更早历史拒绝、gap／不同 payload、pause／resume、更新中 stop、mute、真实 tab 隐藏、ObjectURL 和 dispose。主流在实际播放中继续提交全部片段，逐个记录 updateend 接收、原生时钟、实际缓冲和跨界。

`restored-root2` 首次完整通过时，首 playing 缓冲为 0.255418 秒，实际最大前瞻约 0.258454 秒；实际最长历史约 0.502118 秒（包括最后一片之后的尾部排空），496 次 remove 都在当前时钟之前。EOS 提交时位于约 11.743 秒，随后走到约 11.999999 秒的唯一最终 ended；接收总数 529,200、nextSeq 518。固定实例未重播，来源未更换。

最终带完整 controller 源码绑定及隐藏／dispose 后资源记录的复跑，见机器索引 [validation.json](../../../games/yuyuhakusho/verification/audio-platform-stream/validation.json)。Root 的 8 项 FakeMSE 测试及其先行 SHA 仍是低层异常分支证据，不能取代上述真实浏览器结果。

## 复跑与限制

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_audio_stream.py \
  --fixture .local/md-yuyuhakusho/audio-encoder-plan/e2e/native/original12/small-bulk \
  --label stream-review
```

该脚本使用独立 `audio-platform-stream` profile／session，串行运行；原生激活精确验收页并等待实际 visible 后才发送可信手势。startup probe 是单独的私有诊断工具，不改变生产代码或正式流前瞻界限。

当前仍未证明：未知未来效果声进入同一 mixed timeline 的响应延迟、已缓冲未来数据修订、独立 FM／PSG／DAC 合成、原驱动混音、无限 BGM、Safari／手机及扬声器样本时钟。原生 MSE 的启动门槛可以在开局准备时测量；互动声音的稳态延迟还需另测，不能因这段已知 PCM 连续播放成功而自动通过。
