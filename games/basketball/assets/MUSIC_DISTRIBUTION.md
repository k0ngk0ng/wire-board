# 背景音乐压缩分发

原 9 首分发批次（8 首场地背景音乐和有限结局曲 75）使用 **MP3 128 kbps、44,100 Hz、单声道** 分发。9 个 WAV 母版合计 1,343,940,008 字节，MP3 合计 243,808,096 字节，减少 **81.86%**。这是全部曲目总量；单个播放请求只选当前曲目。完整原节拍周期保持不变，长曲仍有较大文件，不能将总量减少等同于手机网络体验已验收。

音乐 76 另有独立批次：`music-076-intro-loop.mp3` 为 11,798,194 字节，母版为 65,031,540 字节。它保留原前缀和完整循环周期，使用同一压缩格式；没有改写前 9 首的历史分发清单。具体来源与核验见 [音乐 76 素材验收](../MUSIC76_ASSET_VERIFICATION.md)。

原始录音、衔接后的 WAV 母版、重复编码和研究预览都保留在本地。没有重新录音、缩短曲目或改写音符，没有上传 OSS；人工听感及发布状态仍为 `false`。

## 格式选择与身份

纽约曲 66 用于比较 MP3 64／96／128 kbps、AAC 64 kbps、Opus 48 kbps 与 FLAC。低码率更小，但失真较高；Opus 解码到 48 kHz，需要另行处理时基。FLAC 无损，样本文件为 13,254,682 字节，MP3 128 kbps 为 4,363,118 字节。本轮选择 MP3 作为统一原生媒体格式；没有添加格式自动切换，也没有依赖浏览器 Web Audio 解码。

编码使用本机已有 FFmpeg 9.0.1 的 `libmp3lame`，保留 Xing／LAME 延迟和填充信息。核心参数为：

```text
-map_metadata -1 -vn -ac 1 -ar 44100 -c:a libmp3lame -b:a 128k -write_xing 1 -id3v2_version 3 -f mp3
```

工具二进制 SHA-256 为 `9f5ddd4f8b36eb07b0c5d60ab6ea1e618bedd70bc45feab82b4dc23be1596ad1`。每首的完整命令、母版摘要、压缩摘要、重复编码摘要、解码 PCM 摘要及验证指标保存于 `.local/basketball/audio/export/music-distribution-manifest.json`。工具摘要标识本次编码程序，不保证不同版本或构建的编码字节相同。

每首独立编码两次，MP3 字节摘要相同。全部完整解码后，样本数与相应母版一致，容器时长误差小于 1 个样本；保留原有循环时基。固定质量条件包括 SI-SDR 至少 24 dB、解码后无削波、接缝跳变处于邻近样本步幅范围内、首尾局部 RMS 相对母版偏差不超过 1.5 dB。9 首全部使用 128 kbps 通过，SI-SDR 实测约 26.86～30.32 dB。这些数值不能替代人工听感验收。

## 清单与分发边界

`audio-playback.json` 的音乐条目约定如下：

| 字段 | 含义 |
| --- | --- |
| `wav`、`sha256` | 保留本地母版名称和摘要，供来源核验使用。 |
| `file`、`distribution_sha256` | 网页实际播放的压缩文件名称和摘要。 |
| `mime_type` | 当前为 `audio/mpeg`。 |
| `encoding_manifest` | 编码来源清单名称。 |
| `loop` | 59～66 为整段循环；75 为 `false`；76 为带起点的后段循环。 |
| `sample_rate`、`loop_start_sample`、`loop_end_sample`、`loop_samples`、`decoded_samples` | 76 的精确样本边界。后端、前端及打包器使用同一份清单并核对整数关系，不通过视频帧率换算媒体位置。 |

游戏适配器仅选择具备 `file` 且 MIME 为 `audio/mpeg` 的音乐条目。CDN 与本地地址必须指向同一压缩文件，使用同一 `distribution_sha256`。缺少压缩条目时该 ID 不受支持，不能偷偷回退至 WAV 母版或历史 `audio-catalog.json` 的未验收音乐。

分发程序应按清单明确选择文件，不能递归复制整个 `export/` 目录。10 首音乐只分发上述 `file`；31 个效果沿用各自已核验的短 WAV。音乐母版、`*-repeat.mp3`、NSF、逐帧记录及 76 研究预览均不属于运行时文件。服务端须提供正确的 `audio/mpeg`、Range 和长度响应；生产路由及打包由接入阶段另验。

## 原生浏览器验证

使用本地 Chromium、真实平台 `GameAudio` 和篮球适配器，共完成 10 个用例：8 首循环 MP3、有限 MP3 75，以及纽约 66 的 CDN 失败回退。循环用例将真实文件 seek 到尾部附近，以 1 倍速观察两次实际跨尾，不缩短文件，也不从头播完整首长曲。9 首均验证暂停／继续保留位置和 dispose 清空来源；75 另验原生 `ended`。所有网络记录均未请求 WAV。

CDN 用例由本地服务模拟压缩源失败，再以同一 MP3 本地来源成功播放；它没有验证真实 OSS 网络。实际浏览器时长与母版样本时长误差小于 1 个样本，结果中的 `asset_sha256` 与播放清单 `distribution_sha256` 一致。

可核验产物：

- `.local/basketball/audio/export/music-distribution-verification.json`：9 首编码和母版验证结果。
- `.local/basketball/audio/encoded-loop-browser-59/` 至 `-66/`、`-75/`：各曲 `result.json`、`verified.png` 和实际执行的 `loop.js`。
- `.local/basketball/audio/encoded-loop-browser-66-cdn-fallback/`：失败回退结果、请求路径和截图。
- `.local/basketball/audio/compression/acceptance-summary.json`：10 个结果及其文件摘要、素材摘要、bundle 摘要和范围限制。
- `.local/basketball/audio/compression/` 中 `adapter-before.log`、`no-wav-fallback-before.log`、`material-before.log`：实现前失败记录；`adapter-after.log`、`material-after.log` 和 `typecheck.log`：该轮通过记录。

这些记录绑定保存的 `loop.js` 摘要。公共播放器或适配器后续修改需要按影响范围重新验收，不能把已有记录宣称为后续实现的结果。

## 复现

仅核验已保存的压缩素材和浏览器产物：

```sh
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_music_distribution.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_music_browser.py
```

需要重新执行浏览器时，使用已安装的前端依赖和 `agent-browser`：

```sh
source scripts/env.sh
for track in 59 60 61 62 63 64 65 66 75; do
  BASKETBALL_AUDIO_TEST_TRACK="$track" PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/accept_audio_loop.py || exit 1
done
BASKETBALL_AUDIO_TEST_TRACK=66 BASKETBALL_AUDIO_CDN_FAILURE=1 PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/accept_audio_loop.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_music_browser.py
```

若母版确实重新派生，先核验母版，再执行 `encode_music_distribution.py` 更新分发清单，最后重新验证。母版派生脚本可能重建音乐条目，不应在压缩验收中无故重跑。编码器使用进程锁，并复用母版、工具和已生成文件摘要均匹配的完成记录。

真实 Safari／iOS／Android 设备、全场混音及人工试听仍未验收。76 已通过平台原生媒体的前缀／循环边界、恢复、暂停、静音、隐藏、换 owner、释放与本地 CDN 失败回退检查。当前播放器的 66 整段循环和 75 有限曲也已重新验收；其他旧浏览器记录仍只证明各自保存的 bundle。见 [音乐 76 运行时验收](../MUSIC76_RUNTIME_VERIFICATION.md)。`accepted_for_release` 和 `listening_accepted` 保持 `false`。
