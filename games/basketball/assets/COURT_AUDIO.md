# 各场地与结局音乐

场地索引是原 `$058B`，与 `$0588` 的画面布局／场景索引不同。固定 `$D23F..D245` 读取 `$D306..D30D` 后请求声音，8 个值为 `[59,66,61,64,60,62,63,65]`。目前只有 Court 1 的纽约名称已独立核实，其余保留数字，不从建筑图片猜城市。

| Court | 原音乐 ID | 原序列圈数／完整节拍周期 | 循环文件时长 |
| --- | --- | --- | --- |
| 0 | 59 | 65 圈／208,896 帧 | 3,475.876 秒 |
| 1（纽约） | 66 | 5 圈／16,384 帧 | 272.618 秒 |
| 2 | 61 | 21 圈／65,536 帧 | 1,090.471 秒 |
| 3 | 64 | 17 圈／55,296 帧 | 920.085 秒 |
| 4 | 60 | 5 圈／12,288 帧 | 204.463 秒 |
| 5 | 62 | 71 圈／245,760 帧 | 4,089.266 秒 |
| 6 | 63 | 83 圈／221,184 帧 | 3,680.339 秒 |
| 7 | 65 | 23 圈／81,920 帧 | 1,363.089 秒 |

单圈原音乐约 40～60 秒。上表较长周期来自原节拍累加器：仅回到同一序列跳转地址不代表节拍相位已重合。每项都保留完整驱动状态重合的周期，而不是把纽约曲套到其他场地，也没有将变长循环压缩成固定时长。

## 来源与素材

`court-music-source.json` 保存 ROM 表字节、请求入口、REA 证据和结果／奖励的范围。源 ROM SHA-256 是 `0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b`。来源为用户提供的《热血篮球》ROM；对外分发许可尚未建立，素材没有上传 OSS。

`.local/basketball/audio/export/court-music-manifest.json` 保存各项双录文件、完整状态和样本区间、派生方法、SHA、节拍、圈数及局部 RMS。播放清单 `audio-playback.json` 的 `wav`／`sha256` 保留独立母版身份，`file`／`distribution_sha256` 指定实际分发的 MP3 及其摘要。所有播放仍通过平台 `GameAudioPort`。

新增 7 首各自独立渲染两次，保留未加工的 PCM、APU 写入、完整声音驱动 RAM、原序列跳转、PCM 游标和调用记录。两次文件摘要全部一致。纽约 66 原录没有重录，其既有循环文件 SHA 保持不变。

每个派生文件保持原周期长度，只在尾部 1,411 样本（31.995 毫秒）将两个对应音乐位置的原 PCM 做余弦衔接。接缝和局部能量按先写的验证器检查：窗口外字节不变、完整驱动状态相等、无削波、首尾相对原相邻样本误差不超过 1、末 16 样本误差不超过 2，以及 8 毫秒窗口／2 毫秒步长的局部 RMS 偏差不超过 ±2 dB。新增曲实测最大偏差为 1.505 dB，原 DPCM 读取范围均为未改动的 `$FE80..FF80`。

这些是未发布的无损 WAV 母版；长周期会产生较大文件。8 首场地曲及有限结局曲 75 已派生为压缩分发文件，母版不变。压缩参数、原生媒体验收和分发边界见 [MUSIC_DISTRIBUTION.md](MUSIC_DISTRIBUTION.md)。`accepted_for_release` 与 `listening_accepted` 均为 `false`，人工听感和真实设备矩阵尚未核验。

## 比赛结果、奖励与战役结局

- bank 4 `$BA40` 的完整 145 次自然结果调用只出现控制 253／255 和效果 8／50，已有 27 个效果覆盖，不存在需要补一个「结果 BGM」的证据。
- bank 4 `$BD8E` 奖励入口静态直接请求效果 50，其背景加载 `$DBF8` 使用 253／255。未恢复的完整奖励玩法不能被当作已通过动态验证；也不能把后续菜单／结局音乐混称为奖励音乐。
- Scene 7 经 `$CAE3 → $FC83 → bank3:$8015/$B995` 进入战役结局。其 `$BA6C` 请求 75，`$BD68` 请求 76，另有独立 REA 证据。

75 的原音乐声道在采样步 8,470 自然停止，完整有限乐句导出为 `music-075.wav`，分发文件为 `music-075.mp3`，两者时长均为 141.204 秒，播放清单使用 `loop:false`。浏览器已验证正常播放、暂停／继续和实际到尾停止。该用例暴露并修复了公共播放器在 Task 结束后丢失媒体归属的清理问题；增加 4 项失败回归后，仅补充已加载媒体的 owner，保证其他 owner 不被误清理。75 完整生命周期重验通过，到尾后 dispose 的来源已清空。

76 含一次性的节拍 129 前奏，之后是节拍 140 的循环段。原 `music-076.wav` 的 332.768 秒时间轴继续保留为研究预览。运行时改用独立核验的 `music-076-intro-loop.mp3`：前缀为 5,465,255 个样本，随后循环 27,050,493 个样本，采样率均为 44,100 Hz。前缀保留原前奏及循环段进入第三个 D0 边界之前的全部声音；循环段保留 36,864 个原更新帧、35 圈音序，不把单圈音序当作完整节拍周期。平台通过 `loopStartSeconds` 在同一原生媒体资源中重复后段。源码锚点、被拒绝的候选、双录和 MP3 检查见 [音乐 76 素材验收](../MUSIC76_ASSET_VERIFICATION.md)。本地 Chromium 的两次 `ended` 到 `playing` 事件间隔为 4.4 毫秒和 1.2 毫秒；这不是扬声器输出无缝的证明，设备与听感验收仍未完成。

## 新规则的效果缺项

本轮恢复中的代码新增了 13／14／46（`court_animation.go`）和 30（`special_ball.go`）四项请求，已有 27 个效果没有覆盖。这四项现已分别双录、核对请求和 PCM／APU 记录、完整尾部与 SHA，合计覆盖 **31 个当前实现中的效果 ID**。9、19、21 等场地请求仍复用其相同原 ID 的既有素材，不重新录制。

新增效果放在 `additional-effects-manifest.json`，并通过播放清单的 `effects` 字段接入。原 `audio-catalog.json` 保留首批 27 项及其来源证据，避免变更历史批次摘要；游戏适配器合并两个清单。`verify_additional_effects.py` 和针对性适配测试均可重复执行。

## 验证与恢复执行

先检查存活进程及 `court-capture-journal.json`。采样器使用仓库内进程锁，完成目录带 `capture-info.json` 和内容摘要；参数相同的已完成录音直接复用，不能因工具观察超时而重新录制。

```sh
source scripts/env.sh
# 新场地初始探测；已有参数匹配的完成记录会复用。
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/capture_court_music.py 20000 59 60 61 62 63 64 65
# 完整周期使用各自已保留的 213000/20000/69000/250000/224000/59000/86000 帧记录。
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/derive_court_music.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_court_music.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/export_ending_music.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_ending_music.py
# 仅在重新生成母版后需要再执行编码；它复用身份一致的已完成编码。
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/encode_music_distribution.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_music_distribution.py
node --experimental-strip-types --test games/basketball/tests/audio.test.mjs
# 真实平台原生媒体测试，可分别指定 59～66；75 测有限曲到尾停止。
BASKETBALL_AUDIO_TEST_TRACK=59 PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/accept_audio_loop.py
```

早期 WAV 验收记录保留于 `.local/basketball/audio/loop-browser-NNN/`（纽约为 `loop-browser/`）。当前 8 首压缩场地曲均通过两次真实原生媒体跨尾循环、同一来源保持、253／255 不重置位置及 dispose 清理；75 的 MP3 另验到尾结束，66 另验压缩 CDN 失败后使用本地 MP3。压缩结果位于 `encoded-loop-browser-NNN/`，汇总见 `compression/acceptance-summary.json`。测试 seek 到尾部附近，不缩短或加速原文件，不代表人工听感或整场游戏验收。
