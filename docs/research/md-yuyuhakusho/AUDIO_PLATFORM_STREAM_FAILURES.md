# 连续原生无损流的先行验收范围

本清单、public 类型和真实浏览器验收先于共享流播放器实现。仅以现有原 `82` 连续 PCM 的有限片段证明承载；不把它称为独立 DSP、无限 BGM、原混音或扬声器样本等价。

## 格式、序列和容量

- 缺少 `openStream`，或以游戏私有 HTMLAudio／MediaSource／AudioContext 替代平台入口。
- MIME／采样率／初始化片段不支持；字段非安全整数、越界、空字节、init 超过 64 KiB、媒体片超过 1 MiB。
- 同 owner、同实例的归一化配置和 init 相同却重建媒体；同实例配置或 init 冲突却打断有效流；自然结束后同实例重播。
- 第一个媒体片不是 sequence 1／startSample 0；后续 gap、倒序、溢出、重复序号不同内容进入 SourceBuffer。
- 最后已接受片精确重复被追加两次；较早历史片无限缓存；接收游标提前、双份游标相互不同步。
- append 中又接收不同片形成无界队列；同一 pending 片精确重复不复用 Promise；拒绝后 producer 错误推进游标而丢段。
- caller 修改 init／pending 媒体字节改变已提交内容；闭流后仍保留大块旧数据。
- 默认 ahead 66,150 samples、back 88,200 samples 没有落实；自定义 ahead 超过 441,000、back 小于 11,025 或超过 441,000 被接受。
- SourceBuffer／native buffered 持续增长，或为清理直接移除全部当前缓冲，导致待播放音频丢失。
- 仅验证外部元数据，却声称已证明 fMP4 内部时间戳／编码真实性；这需要编码器及清单的独立证明。

## 就绪、结束和生命周期

- `ready` 等待尚无媒体样本的 `play()`；或 append 等待 `play()`，使串行小片 producer 因播放器还需要更多数据而死锁。
- `ready` 在 sourceopen、SourceBuffer 和 init 的 updateend 之前成功；append 在 updateend 前提交接收游标。
- end 直接停止、移除当前缓冲，未播到尾；EOS 提交成功被写成播放已经结束。
- end 与当前 append 竞态丢段；重复 end 创建多任务；请求 end 后仍接受新片。
- 替换 opening 流、append 更新中 stop、owner pause／resume、mute／hidden／离桌／dispose 后，迟到 sourceopen／error／play Promise 复活旧流或停止新 epoch。
- 旧 handle 的 append／end／stop 操作影响新 owner／实例；ObjectURL 不撤销或多次释放产生错误。
- 隐藏页开流；测试页没有实际置前便将 available=false 误写成播放器失败。必须记录真实 visible 与可信手势，不覆盖 document.hidden。
- 新 stream 暗中改变 legacy play／music／finite 行为；模式切换由宿主明确停止旧适配器，stream 只替换旧 stream。
- 暂无依据的 seek／未来时间定位入口进入 public API，或把媒体时钟等同于原 CPU 音频时钟。

## 真实浏览器与证据层次

- 只用 FakeMSE／能力声明，不实际创建平台管理的原生 HTMLAudio 和 SourceBuffer。
- 一次性预载 12 秒后播放，被误称实时低缓冲追加；最终 Go 小片验收必须持续增量供给、记录首次 playing 的实际缓冲和后续样本边界。
- 首片播放后未继续追加；存在 waiting／stalled／error、src 重建或 seeking；所有片已接受却只播放首片。
- 原始 PCM、无损封装重组再解码和浏览器时间轴混成一项通过；Opus 捕获误作原 44.1 kHz 样本证明。
- 只检查最后 currentTime，不证明至少两个实际片段边界；从未触发的迟到回调竞态被当作覆盖。

所有状态和队列必须有界。`append` 成功只表示本片 updateend 后提交，不能承诺已听到；`end` 成功只表示 EOS 已提交，phase 在播放到尾之前为 draining。一个 in-flight append，最多等待该片的一个 end；拒绝不影响有效流。原生 Chromium 的有限载体验收与未来低延迟、Safari／手机和真正原版合成分别报告。
