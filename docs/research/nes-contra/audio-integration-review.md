# 魂斗罗音频与逐帧呈现接入审查

本轮检查 `RuntimeClient` 与 `RuntimePresentationQueue` 从每 50 ms 批次改为 60 Hz 权威快照队列后的音频边界。检查和修改限于音频适配、音频验收及说明；公共 `restoreTrack`／`rate` 与运行客户端的接线由平台任务维护。

## 发现与修复

| 问题 | 影响 | 处理 |
| --- | --- | --- |
| 媒体首次加载较慢，适配器只记启动事件的位置 | 声音加载完成后持续落后当前已绘制快照 | 每次有效音频快照保留当前 voice 位置；成功的迟到播放 Promise 只校准仍属同 epoch、同 lane、同 playbackID 的活跃媒体 |
| 暂停、恢复与 pulse1 重新显声没有位置校准 | 媒体墙钟与权威位置的差异继续保留 | 暂停后校准，恢复／取消声道静音前校准；普通连续帧不反复 seek |
| 60 Hz 权威时钟与约 60.098 Hz 原录不同 | 背景音乐逐渐领先画面及逐 tick 打击乐 | `rate = tickSeconds × 60`；位置仍为原媒体秒。循环使用 `periodSamples / sampleRate / periodTicks`，有限声使用 NSF 头的 16,639 µs 更新间隔 |
| 后台隐藏时停止媒体，返回后同 ID 被 tombstone 拒绝 | 同一首背景音乐不能继续 | `suspend()` 与永久 `stop()` 分开；仅下一次显式恢复可经平台 `restoreTrack` 恢复当前持续音乐，复用原 ID、资源、循环和 rate |
| 终局修订可能合并数个音频 tick | 仍存活的声音从过早的事件位置启动 | 以最终显示快照中同 ID voice 的年龄启动 |
| 耗尽继续次数后立即 finalized，音频不再推进 | `$6E` 的三条旋律能播完，后续打击乐却没有请求 | 仅首次连续、实时的 finalized 边界，且只有同龄的三条 `$6E` 旋律时，停止分轨并按当前偏移续播已验证完整原版有限曲 |

`rate` 的例子：丛林曲为 `0.9983560090702946`，有限效果为 `0.99834`。它修正长期速率差异，不表示多个原生媒体通道已经达到采样级同步。

后台恢复不制造新的 `playbackID`，不恢复枪声、爆炸、暂停提示或其他有限效果。全局取消静音本身不触发恢复。显式停止、离房、旧 epoch、乱序快照和迟到异步回调均不能恢复旧声音。正常逐帧更新不重新启动已有轨道。

## 终局完整曲的来源与边界

完整文件为 `.local/contra-audio/export/cue-6e.wav`，SHA-256：`c5c16a2b174c64c4a2d2a2fef2ba5507311c8db66f09efd01ed62240731f3fe8`。它与两份已留存的完整原录及原请求到 PCM 游标的裁剪范围一致，尾部完整。

原 `calls.csv` 与独立 Go 控制数据的 slot 3 请求逐项相等：共 **39 次内部请求，27 次 noise、12 次 DPCM**，从 age 7 到 age 287。`FlushTerminal` 最多推进 5 次声音更新，随后停止计时，因此不能依靠后续平台 tick 补齐这些请求。来源核验见 [终局曲证据](evidence/audio-terminal-cue-verification.json)，可用 `python3 -B scripts/contra/audio_terminal_verify.py` 复查。

前端没有创造新的权威 tick。专用 `contra/terminal-cue` 媒体 lane 复用 slot 0 原生 `playbackID` 和当前偏移，只在本次实时终局边界播放一次。普通非终局 `$6E` 继续走原逐 slot 控制。初次进入、重连、历史缺口、后台返回、重复 finalized、全局静音和离房都不会补播旧终局曲。完整曲接入由 `resolveTerminalCue` 单独解析，不泛化为有限效果恢复入口。

## 验证方式

先记录失败场景，再运行失败测试，再修改实现。针对性测试覆盖真实呈现队列到音频适配的调用边界、延迟媒体 Promise、暂停修订、跨槽位抢占、换关、恢复与终局。原生浏览器夹具使用生产 Go `AudioState` 快照、生产适配器和 `GameAudio`，音频仍由真实 `HTMLAudioElement` 播放；包含 350 ms 的真实 HTTP 音频延迟、原生标签页隐藏和完整终局曲自然结束。

原生位置测试的服务器必须支持 HTTP Range。首次使用仅返回 200 的 WAV 夹具时，Chromium 没有按目标位置恢复；加入标准 206／Content-Range 后，通过了实际 `currentTime` 检查。因此保留 Range 是媒体分发与本地夹具的接入条件，不能只检查 seek API 返回 true。

最终结果、源码绑定、PC／320px 截图和复验命令见 [本轮验收](evidence/audio-integration-review-verification.json)。这些结果只证明所测浏览器中的控制和媒体位置行为，不替代实际设备听感，不证明 NES 非线性混音或多轨采样级同步。

平台任务已修正另外两项接线风险：队列超过 250 ms 没有实际绘制后，下一次有效状态以 resync 跳到最新位置；隐藏页面不执行绘制／音频回调，初始化等待页面恢复可见。已回读对应实现，并通过包含这些回归的 22 项音频与呈现测试。原生媒体另通过 17 项检查；保留 PC 和 320px 截图。
