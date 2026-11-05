# 平台原生 PCM 流接口

平台现已通过 `BoardProps.audio.openPCMStream` 管理生成音轨、AudioData 和原生 HTMLAudio。两组真实 Chromium 157 的 12 秒测试分别按 1,024／441 帧供给全部 529,200 个原 PCM sample frames；源连续、实际 playing、暂停、取消和资源释放均有独立记录。没有导入研究 probe 的私有播放器，没有 AudioContext、麦克风或外部声音输入。

这证明生产承载能力，不证明独立 DSP、原版任意混音、设备已播样本或扬声器 bit-exact。`nativePlayedSamples` 和 `nativeDroppedSamples` 始终为 null，不能以接收游标填充。

## Public 合约与时间来源

公开类型位于 [audio-pcm-stream.ts](../../../web/src/game-api/audio-pcm-stream.ts)，实现由 [audio-pcm-stream.ts](../../../web/src/audio-pcm-stream.ts) 和 `GameAudio` 统一管理。

配置固定为 44,100 Hz、双声道、s16le。owner 内 `playbackID` 标识 epoch；默认及最大 `maxAheadSamples` 为 2,048，可收紧。packet 从 sequence 1／sample 0 开始，最多 1,024 帧，字节数必须恰为 `sampleFrames * 4`。所有计数、元数据和派生微秒 timestamp 都必须安全；timestamp 从唯一源 sample cursor 用整数商和余数计算，不按墙钟累加。

第一片合法 append 才建立独立 `performance.now()` 源时钟。非法、busy 和重复请求不得重设原点；native currentTime 不参与供给授信。每次仅一片 pending，包含等待准入预算和 native write 的阶段；不同并发请求拒绝，精确 pending 重复返回同一 Promise。校验后在任何 await 前复制元数据和 PCM，write 成功且 epoch 仍有效才提交唯一 `acceptedEndSample/nextSeq`。只保留最后一片去重，不累积历史。

owner pause 冻结源时钟和媒体，保留至多一片 pending；resume 继续相同源游标，不把暂停墙钟转换为供给欠账。固定源迟到预算为 4,410 samples（100 ms）。未声明输入结束的供给缺口或 pending 超时会失败并释放，要求宿主从当前模型建立新 epoch；不静默跳过 packet，不伪造输出 drop。该预算是明确策略，不是设备丢样测量。

## ready、输入结束与释放

| 状态／操作 | 实际含义 |
| --- | --- |
| `ready` | 轨道、writer 和平台原生元素已建立，可供给；不等待首包或 play Promise |
| `append` 成功 | 原生 write 已接受，接收游标提交；不表示设备已播 |
| `endInput` | 立即锁住新输入，等待最多一片 pending及其名义源结束时刻，再 close writer |
| `input-ended` | 输入关闭；不是 MSE draining／ended，也不是 speaker drain |
| `released` | 显式 stop、mute、离桌或 dispose 的资源清理完成；不是播放结束 |

`endInput` 后停止供给缺口检查，但 pending 仍受迟到预算约束。没有自动猜测尾部 drain 时间。宿主必须显式 stop：暂停并清空 srcObject、停止轨道、abort 或关闭 writer、释放锁、关闭所有临时 AudioData并移除元素。取消后迟到 write／play 回调不提交旧游标、不复活旧流，也不影响继任 epoch。

## 与既有音频的关系

PCM 与 MSE 是两个互斥的 mixed 输出模式。新资源 normalize／prepare 成功后才替换旧 mixed 输出，非法参数、不支持能力或准备失败不能截断当前有效通路。旧 handle／owner 只清理自身资源。legacy finite、music 和 loop 保持独立语义，统一 mute／音量 0／hidden／离桌仍取消全部当前声音，不在解除静音后补播历史。

真实 contract 场景验证了无效 PCM 保留 MSE、成功 PCM 停旧 MSE、无效 MSE 保留 PCM、成功新 owner MSE 释放旧 PCM，以及旧 handle stop 不影响新输出。原 MSE 517 片／516 跨界验收、8 项 finite 原生及 10 项 legacy 原生验收再次通过。

## 先行测试与实际结果

先冻结[失败谱](AUDIO_PLATFORM_PCM_FAILURES.md)、public 类型、完整真实平台 E2E 和 16 项受控时钟／事务／释放竞态，再保存缺入口的实际 red，随后实现生产代码。首个实现曾使用 Node strip-only 不支持的 TypeScript parameter property；保留该失败后只改显式字段声明，不改测试。原 16 新测试和 83 既有平台测试保持通过。后续专属回归另有 6 项，当前合计 105 项通过，全项目 TypeScript 检查通过。

真实平台矩阵包含两组 12 秒、contract、35 ms 抖动、暂停、mute、stop、真实 owned-target hidden，以及主动延迟 220 ms 的失败恢复边界。每次构造的原生 AudioData 都检查 format、采样率、声道、帧数、timestamp 和 copyTo 数据 FNV；Python 再按原 PCM 区间核对。每个对象恰 close 一次，取消路径仍保持输出计数为 null。

| 最终完整配置 | 源帧／packet | 首次合法 append 至 playing | 冷启动 click handler 至 playing | 最大已观察 ahead |
| --- | ---: | ---: | ---: | ---: |
| 1,024 帧、预算 2,048 | 529,200／517 | 约 3.7 ms | 约 123.7 ms | 2,038 samples，约 46.21 ms |
| 441 帧、预算 441 | 529,200／1,200 | 约 1.0 ms | 约 77.1 ms | 424 samples，约 9.61 ms |

冷启动包含平台 `unlock()` 的静音原生首播。append 起点由第一条调用的记录时间派生；它与 playing 事件差值不是首个非零声音或扬声器延迟。原 `82` 本身还有前导静音，不应从这些事件数字推算输出 SLA。

输入关闭后有实际反证：两组完整测试的 `nativeCurrentTimeSeconds` 仍继续至约 12.08 秒，轨道已 ended，但 phase 只标 input-ended。随后宿主显式 stop 才让 released 成功、所有媒体移除和 writer 解锁。主动 220 ms 源停顿在约 100 ms 预算边缘进入 `source-too-late`，本轮最终源迟到 4,515 samples；已接受游标停在 26,460，不跳过源 packet，不填 native drop。

两个 native 覆盖点在实现后另行加强且保留 lineage：contract 最初 441 帧处于原前导静音，改为含非零声音的 1,024 帧并直接核对实际 AudioData FNV；endInput 后的探测包改为明确有效的 4 bytes，确保拒绝来自 input-closed 而非空字节。没有放宽生产或既有断言；此前 9 场原结果和旧 harness 均保存，新后缀 `-final` 的 9 场重新通过。

## 后续复核的明确历史

复核发现同一原生元素上，pause／resume 后旧 play Promise 的迟到成功可能暂停当前 generation，以及一次清理异常可能跳过其余资源。最早复核修复曾先写代码、只跑旧测试，缺少专属先行证明；这段历史和当时源码／结果明确保留，不能称为完整 test-first。

随后精确恢复修复前源码（SHA 与此前原生结果的坏实现相同），不改原 16 项，先新增独立 6 项回归，真实得到 6 项全 red，再重施最终修复并得到 6 项全绿。原生矩阵及 MSE／finite／legacy 都在最终重施后以新 label 复跑。清理各步骤逐项尝试；如确有清理异常，released 拒绝且 status 标 failed。内部已经 attach handled catch，未 await released 的宿主 stop 不造成 unhandled rejection，而随后 await released 仍观察到原错误。

完整 rewind、旧源码、测试 SHA、red 和最终重施由 `review-fix-rewind.json`、`review-test-first.json` 和 `review-final-reapplication.json` 绑定。这里只承诺最终修复循环在新增回归 red 之后，不改写最早复核编辑的时序。

## 证据与复跑

机器总索引为 [validation.json](../../../games/yuyuhakusho/verification/audio-platform-pcm/validation.json)，其中分别绑定原 red、105 项测试、最终 9 场、源 PCM、当前生产源码、旧接口回归与清理。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_audio_pcm_platform.py --suffix review

# 单场示例；每场使用本工作区内的独立命名浏览器/profile。
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_audio_pcm_platform.py \
  --label pcm-review --seconds 12 --chunk 1024 --ahead 2048 --scenario smoke
```

浏览器必须已有可用的生成音轨和 AudioData 能力；不安装软件。脚本显式使用已有 Chrome、工作树内 profile／cache／TMP／相对 socket，真实激活精确页面后再发送可信手势。多个 PCM 矩阵场景必须串行，不能并发争用该 profile。

当前没有 Safari／手机、原生设备样本回读、已播／drop 数量、DSP 接线或原混音等价证据。承载与原版合成、调度、游戏同步继续分别验收；不把接收全部 PCM 写成设备已播放全部 PCM。
