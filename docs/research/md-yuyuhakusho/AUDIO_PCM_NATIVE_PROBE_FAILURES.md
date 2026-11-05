# 原生生成音轨：先行可行性验收

本探针只研究 `MediaStreamTrackGenerator(kind:"audio")`、`WritableStream<AudioData>` 和原生 `HTMLAudioElement.srcObject` 能否承载已验原 `82` 的连续 PCM。使用工作树内独立 profile／缓存／临时目录，不修改 MSE 验收、共享 web 生产代码或其他会话。禁止创建 AudioContext、调用麦克风／屏幕捕获、捕获外部音频、安装工具或发布资源。

先保存失败范围和浏览器验收 harness，在缺少生成音轨实现时保存真实失败，再加入研究页的探针实现。先证明写入成功且原生媒体确实产生 `playing` 与持续 currentTime，之后才比较低缓冲启动。函数存在、构造成功或 write 已接受均不能单独证明播放。

## 来源与连续输入

- 原 WAV 或前 12 秒 PCM 摘要不符；声道、采样率、16 位小端布局或帧数错误。
- AudioData 不支持 `s16`／双声道／44,100 Hz；构造后 copyTo 不能复原提交源；先 close 再 write，或写入完成后未 close。
- 1024／441 个立体声采样帧的 packet 出现 gap、重复、倒序、末片越界；timestamp 来自墙钟而不是唯一 sample cursor。
- producer 跳过迟到输入、advance cursor 早于写入完成、同时多次 write、无界排队、写入或 ready Promise 永不完成。
- 首先一次性写完 12 秒再播放，误称低缓冲；只证明短头部播放，未跨过多个真实 packet 边界。

## 原生播放与生命周期

- 未记录真实可见页和可信手势，或在同一手势里 await play 后才开始供给，造成输入／播放互等。
- 只有 play Promise／write Promise 成功，没有 `playing`、推进的 currentTime 或非零输入实际提交。
- `waiting/stalled/error` 被忽略；媒体时钟倒退；用接受样本游标替代实际媒体时钟。
- 最小 startup 尚未测量就声称低延迟；timestamp 精确被误称 speaker PCM bit-exact 或无丢帧。
- 迟到／抖动没有记录；源未丢包被误称原生渲染无丢样本。原生输出若无样本计数接口，应保留不可观察边界。
- pause 后 producer 继续累积，resume 重新从头或修改 sample timestamp；close／stop／mute／真实 hidden 后迟到 write 复活已关闭资源。
- writer 未释放锁、生成轨道未结束、srcObject 未清空、原生元素未移除、AudioData 未 close、定时器或大块 source 长期保留。
- Headless Chrome 的原生事件被误称已听到扬声器输出；Chrome 私有／旧实验接口结果外推到 Safari、手机或生产可用。

当前验收只声明输入 PCM／构造的 AudioData／已接受 packet 连续。原生媒体输出可能转换采样格式、重采样或在实时过期时丢弃数据；没有扬声器／输出采样回读时，不声明样本等价。抖动、pause、mute、stop、close 和 hidden 分别记录，不为替代承载而修改已有 MSE 断言。
