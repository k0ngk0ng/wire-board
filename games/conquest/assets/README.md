# 征服时代音效

本目录提供 3 个项目原创的短音效，使用 Python 标准库离线合成。未使用外部采样或《Age of Conquest》的商业音频。作者为 wire-board contributors，供 wire-board 项目使用。

| 文件 | 用途 | 时长 |
| --- | --- | --- |
| `public/conquest/v1/audio/march.wav` | 开始执行行军指令 | 0.38 秒 |
| `public/conquest/v1/audio/capture.wav` | 服务端确认占领地区 | 约 0.61 秒 |
| `public/conquest/v1/audio/round.wav` | 新一轮规划开始 | 0.56 秒 |

格式统一为 22,050 Hz、16 位、单声道 PCM WAV。音色由正弦、低电平倍频、平滑起止与指数衰减包络组成；不使用突变方波或噪声。

`manifest.json` 记录生成方式、来源与许可说明、生成器源码 SHA-256、每个成品的 SHA-256、字节数和采样帧数。生成命令和复验命令均从仓库根目录执行：

```sh
python3 -B games/conquest/assets/generate_audio.py
python3 -B games/conquest/tests/audio_contract_test.py
```

先行场景见 `../tests/AUDIO_FAILURE_SCENARIOS.md`。验证在生成器与成品尚不存在时先确认失败，再实现生成器；完成后检查 WAV 解码、格式、时长、峰值、均方根音量、起止样本及连续样本变化，并在工作树内独立生成两次比较完整字节。复验结果保存在 `.local/conquest/audio/verification.json`，临时重复生成目录自动清理。

发布对象目录为 `conquest/v1/audio/`，前端通过平台 `BoardProps.audio.playWithFallback` 使用 OSS 地址及本地回退。上传与回读验证由发布流程单独记录；本生成器不上传文件。已经发布的对象不能覆盖，修改声音应使用新的版本目录。
