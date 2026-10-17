# 联机音乐状态与恢复

已实现服务端音乐时间线、带起始偏移的平台播放接口，以及房间初次连接、画面重新同步时的音乐恢复。只恢复当前背景音乐，不通过重放历史 `SoundIDs` 补播效果声。

## 原命令与权威状态

`server/music.go` 只消费原引擎实际产生的声音命令。0 清除音乐和暂停状态；正音乐 ID 创建新的音乐代次，包括重复请求同一 ID；253 暂停当前时间线；255 从暂停位置继续。游戏的 START 暂停不等于音乐暂停，不能从 `Match.Phase` 或场地名称推断这些命令。

正音乐 ID 会清除已有暂停，不需要先收到 255。原代码 `$80E7` 的 `$80F5..80FC` 对 `$07E5` 执行 `(value | $02) & $BF`；0 经 `$8046 → $8166` 清除 `$0700..$07E6`。依据为驱动 REA 记录 `ev_9925b47bdf83028e38be3e23572c67d65c54c6cacd64b34f9b239f465eadaa73`，见声音研究记录中的 `audio/driver-rea/`。适配器已修正“暂停后新音乐仍被拒绝”的差异；同一曲目被原程序再次请求时也会重新开始。

快照字段 `music` 包含：

| 字段 | 含义 |
| --- | --- |
| `epoch` | 原新曲请求／清除形成的新代次 |
| `trackID` | 原音乐编号；0 表示无当前音乐 |
| `assetSHA256` | 该曲分发文件的摘要，与前端同一份播放清单绑定 |
| `controlSequence` | 最后改变音乐状态的声音事件序号 |
| `positionMS`、`at` | 音乐在服务器时间 `at` 时的位置，单位均为毫秒 |
| `paused` | 由原 253／255／新曲／清除命令决定的暂停状态 |

音乐以分发媒体文件的 1 倍速墙钟时间推进。声音事件的服务器执行时刻建立起点，暂停段不计入位置；不使用 `VideoFrames / 60` 猜测录音的 PCM 位置。服务器恢复后保留该时间线，计入离线墙钟时长，循环素材在实际媒体时长内取模。比赛世界的离线追赶限制不等同于背景音乐停止。

旧开发存档没有音乐字段时不猜测当前曲目，等待下一条明确音乐命令。原 76 保留为已知音乐状态，但当前没有可发布的前奏＋循环分发映射，不能套用其他曲目。

## 播放与生命周期

平台 `GameAudioPort.music` 新增可选 `startOffset`（秒）和 `playbackID`。播放器在元数据可用时按真实媒体时长定位，循环曲取模，有限曲已经结束则保持静音。相同播放代次的重复调用不重新起播。旧请求的元数据不能定位新来源；取消、静音和 owner 清理会移除对应回调。

`BasketballAudio.restoreMusic` 先核对资产摘要与声音水位，再恢复当前曲目。运行中的曲目根据平台提供的服务器时钟估计当前偏移，暂停曲保持原位置。快照恢复仍清理旧效果；音乐恢复失败不会使游戏操作抛异常，也不改变平台静音设置。

浏览器拒绝自动播放后，房间组件仅在后续真实键盘／指针手势中尝试恢复最新音乐状态；它不重试旧效果队列。只有最近 1 秒内收到的权威音乐状态可用于该手势恢复。旧异步加载实例如果尚未获得声音所有权，销毁时不会停止新实例的音乐。

## 验收与限制

```sh
source scripts/env.sh
node --experimental-strip-types --test web/tests/*audio*.test.mjs games/basketball/tests/audio.test.mjs
go test -race ./games/basketball/server -count=1 -v
python3 scripts/basketball/accept_audio_recovery.py
python3 scripts/basketball/accept_network_scene.py
BASKETBALL_BROWSER_E2E=1 go test ./games/basketball/server -run '^TestBasketballBrowserNetworkBoard$' -count=1 -v
```

真实 MP3 浏览器验收覆盖服务端 checkpoint 的位置恢复、原暂停、新曲清除暂停、循环取模、暂停快照恢复、继续、有限曲结束和平台静音，共 9 项；未请求旧效果或 WAV 母版。记录为 `.local/basketball/audio-recovery-browser/result.json`。

真实 HTTP／房间页面另验证向平台端口传递当前音乐位置，以及首次播放被拒绝后的原生手势恢复。此房间用例的音频端口是明确的测试替身；真实媒体证据来自上一项，不能把两者表述为已经完成跨浏览器整局混音验收。记录为 `.local/basketball/network-board/result.json`。

服务端验证原命令、重复请求、效果不改变音乐、暂停／继续、新曲和清除、游戏暂停期间 BGM 继续，以及持久化恢复。记录为 `.local/basketball/engine/music-state-race.log`。实现前失败记录保留为 `audio-offset-before.log`、`music-state-before.log`、`audio-recovery-before.log`、`audio-recovery-error-before.log` 和 `music-scene-before.log`。

当前偏移在请求播放时计算；真实远端加载延迟和跨设备漂移仍需进一步测量，不能把本地媒体检查当作样本级同步证明。真实 Safari／手机、服务器重启后跨页面的媒体恢复、人工听感和完整对局混音仍未完成。素材发布标记保持 `false`，未上传 OSS。
