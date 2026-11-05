# 原生 PCM 公开接口：最小合约提案

本页供 Root／媒体 API owner 设计下一阶段生产接口，尚未实现 public 类型或播放器。依据为 [原生探针](AUDIO_PCM_NATIVE_PROBE.md)：Chrome 已有低缓冲连续播放实证，但 writer 接受、媒体 playing、输入结束与扬声器已播样本不是同一个状态。新接口必须保留这些边界，不改写既有 MSE 的结束语义。

## 建议 public 形状

由现有 `BoardProps.audio` owner 提供 `openPCM`，游戏不创建 Audio、AudioContext、生成轨道或静音开关。建议独立于 MSE 的配置和 handle，名称由 Root 定稿：

```ts
type PCMConfig = {
  playbackID: string;             // owner 内稳定 epoch
  sampleRate: 44100;
  channels: 2;
  format: "s16le";
  maxAheadSamples?: number;       // admission 预算，不是 native buffered
};
type PCMPacket = {
  sequence: number;               // 首片 1
  startSample: number;            // 此 epoch 首片 0
  sampleFrames: number;           // 正整数，v1 上限建议 1024
  bytes: Uint8Array;              // 恰好 sampleFrames * 4
};
type PCMHandle = {
  ready: Promise<boolean>;
  released: Promise<void>;        // 资源清理完成，不是声音已播完
  append(packet: PCMPacket): Promise<boolean>;
  endInput(): Promise<boolean>;   // 封闭输入／关闭 writer，不叫 playedToEnd
  stop(): void;
  status(): PCMStatus;
};
```

`PCMStatus` 至少区分 `opening/ready/feeding/paused/input-ended/stopped/failed`，并提供 `acceptedSample`、`lastSequence`、`pendingSequence`、`nativePlaying`、`nativeTrackState`、`nativeCurrentTimeSeconds`、源迟到量和最近拒绝原因。`nativePlayedSamples` 与 `nativeDroppedSamples` 保持 null，不能填入接受游标或推算值。计数器、来源配置及状态快照返回值应不可被调用方修改。

## owner、epoch 与输入事务

- 一个 owner／epoch 同时最多一条 PCM 输出。相同 owner、playbackID 和归一化配置返回同一 handle；同 ID 配置冲突不得破坏当前有效流。新 epoch 明确替换旧 epoch，旧 handle 不能停止或恢复新流。
- `ready` 只证明生成轨道、writer 和平台原生元素已建立并可供给。不得等待尚无输入的 play Promise，否则会重现 producer 与播放互等。`nativePlaying` 独立观测，能力存在不等于 ready／playing。
- 元数据必须为有界安全整数，字节数与帧数准确相符。第一片从 sample 0／sequence 1 开始，后续必须精确接续。timestamp 由 transport 根据唯一 sample cursor 派生；不接受调用方墙钟 timestamp、seek 或 future-start 参数。
- 校验后、任何 await 之前 clone PCM 输入。每次最多一个 pending，包括等待 ahead 预算的片。不同新片在 busy 时拒绝，游标不变；pending 片的精确重复复用同一 Promise。仅保留最后已接受片的有界指纹或字节用于幂等，不能累计历史。
- writer.write 成功且 epoch 仍有效后才提交接受游标。AudioData 在成功、失败、取消路径均 close。停止时迟到的写入完成不得复活旧流；取消返回 false 也不承诺原生曾经完全未见过该输入，调用方不得把旧 epoch 的失败片补播到新 epoch。

仅限制一个 pending 不足以实现低缓冲，因为实际 write 很快。必须另有 producer／admission 时钟和有界 ahead 预算；建议初版默认不高于已测的 2,048 帧。该时钟是供给准入，不是已播放样本。源帧计数必须由 DSP／controller 的唯一 cursor 维护，不能用 native currentTime 给源游标授信。

## pause、迟到、取消与结束

平台 owner pause 冻结媒体及 admission 时钟，保留至多一片 pending；resume 继续同一 sample cursor，不从头播放，也不补写暂停期间的墙钟欠账。原驱动 `F807` 的模型行为仍由 DSP／规则恢复，不能把 transport 的 owner pause 当成驱动暂停实现。

对迟到仅记录源迟到量，不能声称测到了 native drop。v1 不应静默跳过样本、压缩时间或推进 cursor 来追赶；超过 Root 约定的可恢复迟到预算时，明确失败／停止并由 host 建立新的当前 epoch。35 ms 抖动的 probe 只证明源输入仍连续，未证明原生输出没有补静音或丢弃。预算及重同步策略应在生产测试前由 Root 固定。

mute、真实 hidden、离桌、模式切换和 dispose 立即失效旧 epoch，暂停并清空 srcObject，停止生成轨道，abort writer，释放锁与临时 AudioData／clone。`released` 等待这些清理完成。取消后不补播；恢复可见或取消静音时必须由 host 以当前模型状态显式恢复，不靠旧回调继续。

`endInput` 先封闭后续输入，等待至多一片 pending，并在明确的 source/admission 结束预算后 close writer；重复调用幂等。它的成功只表示输入关闭，phase 为 `input-ended`。不能等待或伪造 HTMLAudio ended，更不能宣称 speaker 已 drain。此后必须由 host 的显式 stop／有界清理策略释放原生元素；新 API 文档应规定该义务，不能让一个 inactive MediaStream 的 currentTime 无限推进而被解释为仍有源样本。若 Root 选择自动尾部等待后清理，应把等待值标为策略并单独验收，不将 probe 的 80 ms 等待当作精确 drain 保证。

## Phase 3 先行验收

实现 public 代码前先冻结上述类型和失败清单。单测覆盖 owner／epoch、字段、clone、一个 pending、完全重复／冲突重复、游标原子性、停流后的迟到 resolve／reject、pause／resume、endInput／stop 竞态，以及所有路径的 AudioData／writer／track 释放。Native currentTime 与源游标必须分开测试，禁止把 null 输出计数补成 0。

真实浏览器 E2E 至少复跑原 12 秒 1,024／441 帧输入、原生 playing 与媒体时钟、多片跨界、真实 hidden 和资源释放；再接入实际 Go DSP 的连续输出、735／736 帧时基和 UI 负载／迟到情形。原版 PCM 数学等价、承载可播放和扬声器结果分别报告。MSE 继续按自己的 public 合约与测试通过，不通过弱化 MSE 断言为新承载背书。
