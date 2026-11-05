# 生成音轨承载连续 PCM 的原生可行性

在独立 Chrome `157.0.8103.38` 的工作树内 profile 中，`MediaStreamTrackGenerator(kind:"audio")`、`WritableStream<AudioData>` 和 `HTMLAudioElement.srcObject` 已实际承载原 `82` 的连续 PCM。两组最终 12 秒测试都接受全部 529,200 个立体声采样帧，并产生真实 `playing` 及持续推进的媒体时钟；不是仅检查函数或 canPlayType。

本研究没有修改平台生产、MSE、Go 或游戏播放器。它验证原生承载，不验证 DSP、原版任意混音、原生输出无丢样本或扬声器 bit-exact。完整报告、逐 packet／时钟／生命周期证据和源码摘要见 [总索引](../../../games/yuyuhakusho/verification/audio-pcm-native-probe/summary.json)，先行失败范围见 [FAILURES](AUDIO_PCM_NATIVE_PROBE_FAILURES.md)。平台接口建议另见 [最小合约提案](AUDIO_PCM_NATIVE_CONTRACT_PROPOSAL.md)。

## 连续供给与启动

输入来自已验收 WAV 的前 12 秒，44,100 Hz、双声道、signed16 little-endian，共 529,200 帧。前缀 PCM SHA-256 为 `7801d4c5f7ae515e7819c69637f3974ed2feedfc0362df2370ec28f5b8d55f4c`，与既有纯 Go 无损编码研究使用的输入相同。原 WAV SHA-256 为 `85b105ce9ea41d49cf62da9b0a2612bbfdb436a77a9b6e4c0d72cd8ff3167c50`，没有重新采集或修改。

| 最终配置 | packet 数 | 全部接受样本 | capture-phase 可信手势至 playing | 首 playing 时已接受 | 最大 producer ahead |
| --- | ---: | ---: | ---: | ---: | ---: |
| 1,024 帧，ahead 2 块 | 517 | 529,200 | 18.3 ms | 2,048 帧，46.44 ms | 2,043 帧，约 46.33 ms |
| 441 帧，ahead 1 块 | 1,200 | 529,200 | 11.1 ms | 882 帧，20 ms | 441 帧，10 ms |

两组都最多一个 write 在途，最大已观察 write 等待为约 0.2 ms。输入数组先从本地文件读取，但原生轨道没有预载全部 12 秒：实际最后写入发生在约 11.95／11.99 秒，每片按连续 sample cursor 定时供给。timestamp 固定由 `floor(startSample*1000000/44100)` 派生，不按每片墙钟累计。末片允许短于标准块。

每片都检查实际构造的 AudioData 的帧数、声道、采样率、格式和 timestamp，并通过 `copyTo` 对照原 packet。独立 Python harness 再按原 PCM 区间核对 copy 的校验值、开始样本、帧数及 timestamp。write 完成或失败后全部 AudioData 都 close。所有实测 Chrome 命令行均没有 `mute-audio`、假媒体设备或绕过用户手势的 autoplay 参数；没有 AudioContext、麦克风或外部捕获调用。

初始可行性测试曾得到 21.2 ms／2,048 帧 ahead，另一个 441 帧生命周期样本得到 3.8 ms／441 帧已接受。它们是个别实测，不是延迟 SLA，也不是最短理论 startup。最终表格采用同一最终源码的完整 12 秒结果。原 `82` 本身还有约 22 ms 的前导静音，因此 `playing` 延迟不等于首个非零声音或扬声器延迟。

## 抖动与生命周期

- 441 帧分片在第 61 片前主动等待 35 ms，实测延后约 35.7 ms。400 片／176,400 帧仍按原样本顺序全部接受，没有跳过源 packet。没有观察到首次 playing 之后的 waiting，但这不证明原生渲染没有补静音或丢弃过期输入。
- 暂停观察窗中，媒体 currentTime 保持 `0.516772`，接受样本保持 `22,932`。恢复后相同 epoch 的输入 timestamp 连续，4 秒全部供给完成。该结果不替代原驱动 `F807` 的语义或生产暂停竞态验收。
- mute、stop 和真实 hidden 都在正在播放时触发。mute 同时设置原生元素 muted 并释放；不是以音量标志代替停止。hidden 通过创建同一私有 Chrome 的空白目标使实际 document.hidden 为 true，没有覆盖属性。
- 所有主验收场景都观察到生成轨道 ended、writer 解锁、srcObject 清空、元素移除、MediaStream inactive、pending 为 0，并且 AudioData 创建数与 close 数一致。隐藏页结果是在原隐藏 target 中直接读取，之后才重新置前截图。

实际 write 很快，停止时的观测点没有长期阻塞的原生 write。停止期间迟到 Promise 等全部竞态仍应由下一阶段 public API 的先行单测和原生 E2E 验收，不能由本探针推定已覆盖。

## 被动 EOS 与输出计数的反证

`writer.close()` 成功后，生成轨道已 ended，MediaStream.active 为 false。但最终两组分别被动等待约 354.5／359.3 ms，HTMLAudio.ended 仍为 false，也没有 ended 事件；currentTime 继续到约 `12.428783／12.430891`。随后探针显式 pause、清空 srcObject 并移除元素。

这是有界观测，不声称更长等待也永远不触发 ended。它已经说明：当前原生路径不能把 writer.close、MediaStream inactive 或 HTMLAudio.currentTime 当作“最后一个源样本已经播放完”的精确证明。MSTG 的输入结束不得伪装为 MSE 的被动 ended／draining 语义。

本探针能证明源 PCM、构造的 AudioData 和已接受 packet 连续。它没有原生渲染／设备已播样本计数或丢样本计数，因此这些字段保持 null；当前时钟只作为媒体活动的观测值。浏览器仍可能转换采样格式、重采样、补静音或丢弃过期数据。没有扬声器输出回读，不能声明 native 或 speaker bit-exact、无间隙或输出无 drop。

## 来源版本与复跑

先保存失败清单和 harness，再取得缺 probe 实现的真实浏览器 red。首次结束记录因读取已释放 writer 的 desiredSize 失败，修正仅保护观测 getter。隐藏页首次检查跟随到了新 blank target；保留该 red 和旧 harness／helper，修正为读取完全相同的已隐藏 target，没有放宽播放、连续性或释放断言。v1／v2 探针源和全部历史结果均在索引中绑定 SHA。

已有 Chrome、agent-browser 和真实 Node 必须可用；不安装软件。在 worktree 根目录运行：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_audio_pcm_native_probe.py \
  --label smoke-1024-review --seconds 12 --chunk 1024 --ahead-chunks 2
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_audio_pcm_native_probe.py \
  --label smoke-441-review --seconds 12 --chunk 441 --ahead-chunks 1
```

生命周期使用 `--seconds 4 --chunk 441 --ahead-chunks 1`，分别指定 `--scenario jitter/pause/mute/stop/hidden` 及独立 label。脚本从旧 WAV 验证并重建本地前缀，所有 profile、缓存、临时文件和相对 socket 都位于 `.local/audio-pcm-native-probe/`；socket 命令的 cwd 也固定在该目录，避免 macOS Unix socket 路径上限。每次 finally 都关闭自己的命名 browser session，不操作其他 profile。

测试是 macOS 上的 Headless Chrome。没有 Safari、手机、其他浏览器或真实 Go DSP 到此承载的接入证据。MSE 仍保留独立实现和验收结论；承载选择由 Root 后续决定。
