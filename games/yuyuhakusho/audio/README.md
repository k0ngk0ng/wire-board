# 有限音频目录与媒体校验

`audio` 包提供 134 个已验收有限 WAV 的目录、原动画引用及完整媒体校验。它不调度、混音或播放声音。范围为 `06..7e` 的 121 个 PCM 命令、`a0..ab` 的 12 个 FM／PSG 序列，以及有限结算音乐 `8f`。控制 `00..05`、静音诊断 `7f`、有界音乐 `82/87` 和其他 BGM 均不在目录内。

本批运行元数据见 [audio-finite-catalog.json](../data/audio-finite-catalog.json)，共 37,134 bytes，不含 WAV 正文、`.local`／绝对媒体路径、URL 或 ROM 地址。源目录和原始 WAV 未修改。完整来源、先行失败、原生竞态、Node/Wasm、独立 CLI、全部媒体及重复导出结果见 [验收索引](../data/audio-finite-catalog-validation.json)；实现前失败范围见 [FAILURES.md](FAILURES.md)。

## API 与编号

```go
func DecodeCatalog(raw []byte) (Catalog, error)
func (c Catalog) Lookup(code uint8) (Clip, bool)
func (c Catalog) AnimationCodes(sourceCharacter uint8) ([]uint8, error)
func (c Catalog) ValidateWAV(code uint8, raw []byte) error
func (c Clip) DurationSeconds() float64
func (c Clip) MediaPath() string
```

`Catalog` 成功解码后不可变；失败返回零值目录。`Lookup` 返回标量值副本，`AnimationCodes` 返回新切片，调用方不能修改目录内部数据。`Clip` 保留 `Code`、`Kind`、`StereoFrames`、`WAVSHA256`、`PCMSHA256`、`FirstNonzeroStereoFrame` 和始终为 `false` 的 `Loop`。kind 使用源分类值：`pcm-sample-command`、`finite-fm-psg-sequence`、`finite-music-cue`。

`DurationSeconds()` 直接返回 `float64(StereoFrames)/44100`，目录不另存时长。`MediaPath()` 按已知代码派生 `yuyuhakusho/audio/<两位十六进制代码>.wav`，供调用方拼接可信的不可变素材地址；排除代码返回空字符串。路径不来自目录中的可替换 URL 字段。

`AnimationCodes` 使用原 source character `0..10`，保留 11 组、139 次引用、78 个唯一代码。它只表示原动画数据引用，不代表全部玩法可达，也不是完整必要预加载集合。KO、胜利等声音可能由动画之外的规则产生。不得把平台席位编号直接传作 source character。

## 目录、内容与信任边界

schema1 固定 ROM 摘要、原目录摘要、`44100 Hz / stereo / signed16-little-endian` 和 `source_origin="isolated-capture-start"`。decoder 严格检查完整 134 代码集合、kind、字段存在性／类型、大小写准确的字段名、小写 SHA、帧数、onset、`loop:false` 及完整原动画绑定；缺项、重复、冲突、未知字段、重复 JSON key 和尾随 JSON 均原子拒绝。

ROM／原目录摘要字段只声明来源身份。它们不能证明任意改写的 clip 摘要、帧数和 onset 为真。调用方必须先通过可信不可变 manifest 的摘要认证 manifest，再按 manifest 的 `catalog_sha256` 认证原始目录字节，最后调用 `DecodeCatalog`。不能从同一不可信下载临时计算一个摘要，再把这个摘要当作认证来源。

`ValidateWAV` 将下载的完整 WAV 和 PCM 分别绑定到已经认证的目录。校验还覆盖 RIFF 总长、唯一的 16-byte PCM fmt chunk、唯一 data chunk、chunk 边界和 padding、声道、采样率、位宽、block align、byte rate、帧数及第一个非零立体声采样帧。数据不裁切、不归一化、不重新采样，也不存入目录对象。源文件保留原始前导延迟及完整尾部；onset 是观测位置，不是删除前导延迟或自动 seek 的指令。

`8f` 的旧元数据缺少 onset。导出器从已验收 WAV 实读为第 971 个立体声采样帧，时长由 183,987／44,100 派生，为约 4.172040816 秒。它是 `loop:false` 的有限结算音乐，不是连续 BGM。此目录通过不改变旧音频研究中 `82/87` 精确 PCM 循环、声音时基、混音和平台播放尚未完成的结论。

## 复跑有限媒体闭环

在 worktree 根目录执行。前置条件是旧 [audio-assets.json](../data/audio-assets.json) 和 `.local/md-yuyuhakusho/audio-assets-final/` 内已经验收的原 WAV；本流程不运行 MD 内核，也不重新采集。所有新输出、Go 缓存、临时目录及 telemetry 目录均留在 worktree。不要执行 `go telemetry` 或写全局 Go 配置。

1. 导出到独立目录，并使用 Python 标准 WAV 读取器独立核对全部源。

   ```sh
   PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/export_finite_audio_catalog.py \
     --out .local/md-yuyuhakusho/audio-finite/replay
   PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_finite_audio_catalog.py \
     --catalog .local/md-yuyuhakusho/audio-finite/replay/audio-finite-catalog.json \
     --manifest .local/md-yuyuhakusho/audio-finite/replay/audio-finite-catalog-manifest.json \
     --report .local/md-yuyuhakusho/audio-finite/replay-source-verification.json
   ```

2. 配置本次命令的工作树内目录，运行原生竞态及全部 134 媒体校验。

   ```sh
   mkdir -p .local/md-yuyuhakusho/audio-finite/tmp
   export GOCACHE="$PWD/.local/go-cache"
   export GOMODCACHE="$PWD/.local/go-mod"
   export GOTMPDIR="$PWD/.local/md-yuyuhakusho/audio-finite/tmp"
   export TMPDIR="$PWD/.local/md-yuyuhakusho/audio-finite/tmp"
   export TEST_TELEMETRY_DIR="$PWD/.local/go-telemetry"
   YYH_FINITE_AUDIO_MEDIA_ROOT="$PWD/.local/md-yuyuhakusho/audio-assets-final" \
     go test -race -count=1 -json ./games/yuyuhakusho/audio
   ```

   不设置 `YYH_FINITE_AUDIO_MEDIA_ROOT` 时，纯目录／格式测试仍运行，完整原媒体 E2E 会明确跳过。完整验收必须设置该变量，或运行下一步的独立 CLI；不能以跳过后的测试结果称为全部 134 媒体通过。格式负向测试使用很小的合成 RIFF 夹具，不作为原音效来源证明。

3. 使用真实 Node 和最小环境运行相同 Node/Wasm 测试。

   ```sh
   finite_node_dir="$(dirname "$(command -v node)")"
   finite_go="$(command -v go)"
   finite_wasm_exec="$("$finite_go" env GOROOT)/lib/wasm/go_js_wasm_exec"
   env -i PATH="$finite_node_dir:/usr/bin:/bin" GOENV=off GOOS=js GOARCH=wasm \
     GOCACHE="$GOCACHE" GOMODCACHE="$GOMODCACHE" GOTMPDIR="$GOTMPDIR" TMPDIR="$TMPDIR" \
     TEST_TELEMETRY_DIR="$TEST_TELEMETRY_DIR" \
     YYH_FINITE_AUDIO_MEDIA_ROOT="$PWD/.local/md-yuyuhakusho/audio-assets-final" \
     "$finite_go" test -count=1 -json -exec "$finite_wasm_exec" ./games/yuyuhakusho/audio
   ```

   本次验收使用 `/opt/homebrew/bin/go` 和真实 Node `v24.15.0`。

4. 使用已独立确认的 manifest 摘要运行闭环 CLI。下面的固定摘要对应本批已验收 manifest。

   ```sh
   go run ./games/yuyuhakusho/cmd/verify-finite-audio \
     -manifest-sha256 35d818bf7790df1d8d5cf19e77ef1d23330672eb3c3c2514123f6887220bd620
   ```

   CLI 在验证可信 manifest、完整目录及旧来源身份后，按代码读取原 134 个 WAV，并调用生产 `ValidateWAV`。缺少可信摘要、摘要不符或任何目录／媒体差异都会失败。Wasm 使用同一 CLI 和上述最小环境，在 `go run` 后加 `-exec "$finite_wasm_exec"`。

5. 在另一个 worktree 内目录再次导出，使用 `cmp` 比较 catalog 和 manifest。成功应逐 byte 相同；本批结果见 [确定性记录](../data/audio-finite-catalog-determinism.json)。

本批没有向仓库复制原 WAV，没有上传素材，没有修改 engine、runtimebridge、web 或本地服务器。未来接入仍通过平台 `BoardProps.audio`，声音调度、混音、连续 BGM、浏览器 E2E 和发布分别验收。
