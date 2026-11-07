# 独立原生音频通道验收

`BoardProps.audio.track(key, sources, options)` 提供当前 owner 内独立的原生媒体通道。`sources` 是主地址和回退地址；默认播放一次，可设置 `loop`、`startOffset`、`loopStartSeconds`、`playbackID` 和 `rate`。游戏仍负责声部优先级、事件接受／丢弃、覆盖时刻及资源选择。

- 同 key 的新事件替换旧媒体，不与旧声音重叠；不同 key 可并发，不同 owner 的同名 key 相互隔离。
- `muteTrack(key, true)` 仅静音，媒体时间继续推进；取消通道静音不重播。
- `pauseTrack`／`resumeTrack` 冻结／恢复该通道位置。暂停原有通道后，可在另一 key 播放暂停提示；全局静音和 owner 暂停仍禁止新声音。
- `stopTrack` 取消该通道；公共 `stop`、音量归零、全局静音和离桌清理覆盖所有通道。普通 `track`、`seekTrack` 和取消静音不会恢复已取消的旧声音。
- `seekTrack(key, offsetSeconds): boolean` 只校准当前 owner 仍存在的活跃通道，不调用 `play`／`resume`，不替换播放器、声源或 `playbackID`。暂停和通道静音状态保持不变；已结束、已停止、已卸载或全局静音／隐藏不可用的通道返回 `false`，不会重建。负值、非有限值及已知超出一次性声音长度的位置也返回 `false`。
- seek 使用已播放总秒数：普通循环按总长度取模；带前缀的循环先保留前缀，再在后缀区间归一化。元数据未到时，`true` 表示接受待应用的定位，只有最新请求生效；元数据加载失败仍按公共媒体失败规则清理。原生 seek 完成是异步的，媒体服务器须支持实际寻址。该接口用于显式重连校准，不能用新 `playbackID` 绕过旧声音的停止记录。
- `restoreTrack(key, playbackID, { startOffset, muted?, paused? })` 用于当前对局持续音乐的显式权威恢复。只接受当前 owner 保留的非空相同 ID，且原通道必须为循环；复用原声源、前缀／循环设置和速率，不接受替换资源。一次性音效、旧 ID、其他 owner、已卸载通道、页面隐藏、全局静音或音量为零均不能恢复。已有活跃通道只校准位置；已停止通道重新准备媒体。`paused: true` 在首次原生 `play()` 前生效，直到 `resumeTrack` 才开始加载／播放；期间可用 `seekTrack` 更新待播放位置。调用方须区分后台恢复与永久停止，离桌后不得继续调用恢复接口。
- `rate` 默认为 `1`，只接受 `0.25`～`4` 的有限值，在首次播放和回退前设置原生媒体速率。`startOffset`、`seekTrack` 和 `restoreTrack` 的位置始终使用媒体秒数，不按速率再次换算。该能力可对齐音频素材时钟与游戏权威时钟，不能代替权威事件顺序。
- `playbackID` 应包含房间／对局 epoch 与已接受事件身份。同 key 的相同 ID 不重启，也不能改变声源、循环方式或速率；正常结束、停止及全局静音后仍保留该 ID，阻止重复快照重播。每个 key 只保留最近一次 ID，新的事件 ID 可再次播放相同 URL；乱序包和重连时更早的历史事件仍由游戏权威序列过滤。
- key 必须是稳定通道名，不能每个事件生成一个新 key。播放器最多保留 64 个通道身份；到达上限时只回收其他 owner 已停止的通道，保留当前 owner 的去重边界。

失败场景和先行测试见 `web/tests/platform-audio-tracks.test.mjs`、`platform-audio-seek.test.mjs` 和 `platform-audio-restore.test.mjs`。重新验证：

```sh
source scripts/env.sh
node --experimental-strip-types --test web/tests/platform-audio*.test.mjs
python3 -B scripts/sound/accept_keyed_audio.py --out .local/keyed-audio/recheck
python3 -B scripts/sound/accept_keyed_seek.py --out .local/keyed-seek/recheck
```

浏览器脚本要求输出目录尚不存在。它使用安装好的 agent-browser 和工作区 profile，运行生产 `GameAudio`、原生 WAV 解码及 HTMLAudioElement 播放；保存 JSON、PC 和 320 px 截图，结束后删除自有 profile、音频夹具、临时服务文件和 socket。偏好／隐藏事件由夹具显式接入，跨标签测试产生真实 storage／visibility 事件。该证据验证公共音频能力，不代表任何游戏的原版混音、整局、实体 iOS 设备或扬声器输出已经验收。

此前基础通道浏览器运行使用的脚本 SHA-256 为 `4d8b4f5d1a526a1daefb73ebae34d81feb5f904721aaa78055b987d83e0e1e41`；那次运行后仅将第 102、110 行的两个等待选择器从字符串拼接改为三引号字符串，当前基础脚本 SHA-256 为 `a61af8ca55a9aae906daccfbb48fc1d860f77161eb713a9b9df4b7f97bdacb61`。这两处传给浏览器的 JavaScript 字符串逐字相同；当时没有改动清理逻辑、断言、场景、浏览器动作或生产音频实现。

已反向还原这两处字面量改写，将与运行记录 SHA-256 完全一致的脚本字节保存在 `.local/keyed-audio/browser-native/harness-at-run.py`。该文件是证据快照；重新执行仍使用上方仓库脚本入口。同目录的 `harness-after-run.diff` 保留两行差异，`harness-binding.json` 记录两个摘要及静态解析后的等待表达式一致性；`.local/keyed-audio/verification.json` 引用此绑定。该次摘要绑定补充只核对证据，没有重跑浏览器或改动原版游戏声音验收结论。

后续新增 `seekTrack` 使用独立的 `accept_keyed_seek.py` 验收并保存运行时脚本原件及生产来源摘要。新证据在 `.local/keyed-seek/`：实际 `Range` 响应支持原生定位，测试等待 seek 完成并核对位置、播放器身份、原生 `play()` 调用次数和暂停／静音状态；另覆盖延迟元数据、已取消请求和一次性声音结束。旧通道浏览器脚本也针对增加 seek 后的生产代码回归；旧 SHA 的历史结果不被当作新实现验收。
