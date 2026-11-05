# 纯 Go PCM／FLAC／fMP4 验收

基线为 `0f3bea2f`。失败清单和 6 项顶层测试先于实现；缺少 Config／Encoder／Segment 的真实 Go red 以及缺少验收 CLI 的 E2E red 已保留。实现后冻结测试源码未改。

## 已通过

- Native race 与 Node／Wasm 各 6 项顶层检查：格式／配置、分块不变性与短尾、block／fragment 变体、alias 与错误原子性、36-bit／uint32 边界，以及独立 CRC／box 校验。
- 24 组主 E2E：确定性整数 PCM 65,537 帧和原 `82` 前 12 秒 529,200 帧，分别使用 3 种配置、2 种输入分块，在 native／Wasm 完整编码。全部 Init、每片段及同配置分块变体的字节相同；独立 FFmpeg 解码逐 byte 等于输入，PTS／DTS／duration 连续。
- 4 组额外边界 E2E：最小 block 16、最大 block 65535 在两端真实编码和 FFmpeg 回读。短尾包含 1／2 帧，测试中的 byte／sample 边界没有丢失或填零。
- CRC8、CRC16、tfdt 和 trun duration 破坏分别被独立解码／时间轴检查拒绝。FFmpeg 默认可能输出 CRC 错误但仍退出 0，负向解码因此使用 `-xerror`；该验收器原失败另存，未修改生产编码来迎合它。
- Go 自写 Init／variable FLAC 已在真实 Chromium 152 的原生 HTMLAudio／MSE 播放。default 的 130 个 media fragments 和 small 的 517 个 media fragments 都保留原字节，分别组合成 12 个网络追加 chunk，实际跨 11 个 chunk 边界，playing 后 waiting／stalled／seeking 为 0，src 设置一次，录音只在最终约 12 秒结束。
- `go vet` 通过。运行包 import 只有 `encoding/binary` 和 `errors`，没有外部编码器、ROM、原模拟内核、CPU VM、cgo 或播放器依赖。

分组追加仅证明两种 Go 编码的真实浏览器解码兼容。最终平台验收另使用 Go small-bulk（block 256／fragment 1024）的 517 个实际片段，通过当前平台自管的 HTMLAudio／MSE 逐个提交，真实跨越 516 个样本时间边界。Init、各片 SHA、sequence、sample 范围和最终 529,200 sample／nextSeq 518 已核对，平台和测试源码 SHA 与当前文件一致。

[最终平台报告](../../verification/audio-platform-stream/restored-final1/result.json)覆盖五组：初始化替换与旧 epoch、字节复制／顺序／单 in-flight、暂停恢复、停止／owner／静音清理，以及有界增量完整播放；另实际检查后台与 dispose 清理。播放主过程使用 0.26 秒供给目标、0.5 秒上限，观测最大向前缓冲约 0.25946 秒、496 次安全历史移除，playing 后 waiting／stalled／seeking／error 为 0。所有片段都来自原 PCM 的纯 Go 编码，未重建音源或切换到有损编码。

平台单段测试的旧暂停初始化曾只投 8×1024=8192 帧（约 185.76 ms）而等待启动。独立实测 small 需要第 9 片共 9,216 帧（约 208.98 ms）；按实测修正测试初始化后通过，主供给目标和前瞻上限均未放宽，编码器和冻 Go 测试没有修改。原失败与修正谱系由 [bootstrap-lineage.json](../../verification/audio-platform-stream/bootstrap-lineage.json) 保留。

这个平台闭环仍不证明扬声器逐样本相等、任意未来效果声混音或 input-to-sound 低延迟。样本 byte 等价由独立编解码验收证明，原生 currentTime／跨界和生命周期是另一层证据。通用 encoder 最大配置的约 5.9 MB 片段也不自动满足平台 1 MiB 单片上限；调用适配器须按 README 的输出上界公式提前拒绝不兼容配置。

## 复跑

在 worktree 根目录使用工作区内 Go cache／tmp／遥测目录：

```sh
TEST_TELEMETRY_DIR="$PWD/.local/go-telemetry" \
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 ./games/yuyuhakusho/audio/stream

PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_go_audio_stream.py \
  --out .local/md-yuyuhakusho/audio-stream-review both

PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_go_audio_stream_edges.py \
  --out .local/md-yuyuhakusho/audio-stream-review/edges \
  --base .local/md-yuyuhakusho/audio-stream-review
```

CLI 只读取 PCM 并写编码结果，不运行 FFmpeg。脚本中的 FFmpeg 是独立验收解码器。规范样本、命令、红灯、native／Wasm 日志及 SHA 索引见 [audio-stream-encoder-validation.json](../../data/audio-stream-encoder-validation.json)。大型容器／PCM 与浏览器资料留在 `.local/md-yuyuhakusho/audio-encoder-plan/`，没有上传或进入正式构建。

## 保留边界

编码器不是独立音序／DSP、原版任意混音或无限 BGM。默认 4096／44100 约 92.88 ms 是编码聚合跨度，另有输入批次、平台供给、MSE 和设备延迟；小配置 1024／44100 约 23.22 ms 同样不能直接等同可见／可听响应。Safari／手机和平台生命周期须由对应端到端验收证明。
