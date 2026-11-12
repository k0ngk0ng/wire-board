# 发动机连续播放核验

本轮修复发动机声部反复启动、初始音高错误和原生短文件循环产生的周期空洞。原始 27 个 FLAC、PCM 摘要及公共音频清单保持不变。最终冻结构建的五车真实混音均完成 36 秒录音，按预先冻结的数字输出连续性门槛通过；可播放样本、原始捕获、控制时间线和摘要均已保留。

## 已确认的问题

旧实现会在声部增益低于阈值时销毁播放器。再次进入相应转速或负载区间后，声部从文件头启动。五车型原始浏览器基线分别创建 12、13、7、9、12 个原生播放器；保留零增益声部后，分别降为源文件对应的 5、8、3、5、6 个。

原生播放器还存在初始音高问题：设置 `src` 会将 `playbackRate` 恢复为 `defaultPlaybackRate`。Porsche 911 的声部 36、38 曾以速率 1 启动，首次油门更新后才降至 0.875、0.597。修复先设置 `defaultPlaybackRate`，再设置 `src` 并重新应用控制。该问题有真实声部捕获与频谱佐证。

仅保留声部不能解决原生短文件循环空洞。Lamborghini Diablo SV 的原始声部 41 在稳定怠速下，每轮出现 124～126 ms 的低能量区间；原始 PCM 没有对应静音段。固定速率 0.690267 时，理想循环周期为 1.428609 秒，浏览器实际原生 seek 周期约为 1.536 秒。

独立实验在相同原生播放器上比较 FLAC／相同 PCM 的 WAV，以及开启／关闭录音。四种组合的 seek 周期相同，实际 FLAC 与 WAV 捕获均有最长 126 ms 空洞。另一声部的 5 ms 首尾处理只能把最长空洞从 126 ms 降至 108 ms。以上结果将问题限定在当前浏览器的短文件原生循环机制；不据此推断其他浏览器或原版 Windows 输出。

| 证据 | 仓库内路径 |
| --- | --- |
| 原始 FLAC／PCM 摘要、接缝统计 | `.local/racing-nfs-reference/audio-continuity-source/source-loops.json` |
| 初始音高的基线／修复频谱 | `.local/racing-nfs-reference/audio-continuity-source/p911-{baseline,candidate}-38-*-spectrum/` |
| 正式游戏钩子的怠速空洞 | `.local/racing-nfs-reference/audio-continuity-source/ldia-idle-native-gap-proof.json` |
| 独立 FLAC／WAV 及录音开关实验 | `.local/racing-nfs-reference/native-container-probe-01/summary.json` |
| 原始 A／最小修复 B 五车记录 | `.local/racing-nfs-browser/audio-{A-five-773,B-five-candidate-04}/` |

A／B 可播放 WAV 是「原生声部捕获 + 实测音量包络混音」，保存在 `.local/racing-nfs-browser/audio-native-AB-analysis-01/`。捕获已经包含实际音高，只应用一次实测音量。它们不作为最终连续后端的混音证明。

## 连续播放实现

游戏继续通过 `BoardProps.audio.loop` 发出请求，使用可选的 `continuous: true` 和 `seamOverlapMS: 5`。平台解码经校验的原始 FLAC，在独立播放副本上处理最多 5 ms、且不超过源长度 2% 的首尾重叠。处理不改写源文件，不归一化，不增加淡出到静音。

平台持有声部与输出生命周期。每个声部只启动一次，后续转速、负载、换挡与零增益只调整速率和增益。RPM 表使用线性插值；增益和速率分别采用 25 ms、35 ms 的目标平滑时间常数。

实际路径为 `AudioBufferSource → Gain → MediaStreamDestination → HTMLAudio`。原生媒体元素承担最终播放，统一音量只作用于该原生元素。可信手势恢复、`playback` 音频会话、静音、隐藏、离桌和 owner 清理仍由平台管理。其他未请求连续播放的游戏保留已有原生文件循环行为。能力或解码失败显式标记 fallback，不能通过连续性验收。

## 预先冻结的验收口径

- 五车型均使用正式游戏钩子录制完整数字混音，保留可播放录音和控制时间线。
- 捕获唯一的 `audio[data-platform-continuous]` 所持 `srcObject` 音轨；该流已含声部速率、增益与接缝处理，不再逐声部混音或二次变调。
- `srcObject` 不包含原生元素的主音量；可播放示例只额外应用一次实际主音量，记录其时间线。
- 稳定怠速、原地负载各去除初始 0.5 秒控制／解码过渡。以 2 ms 窗口测量 RMS，低于相应稳定段中位数 5% 的最长连续区间须不超过 20 ms；不得重复出现 40 ms 以上周期空洞。
- 加速、换挡、制动另列低能量区间，并与真实 RPM、挡位和油门对应。所有原始捕获保留，不能隐藏容器空洞。
- 输出期间须只有一个连续原生 sink，context 为 `running`，无 fallback；声部 `starts` 在零增益、换挡及负载变化期间保持 1。

这些门槛只描述数字输出连续性。音频感知工具不可用，未作模型主观听评；硬件扬声器、设备媒体音量和手机铃声静音开关未实测。

## 最终浏览器结果

前端索引 SHA-256 为 `27ddd412ec8422257aa1a4504ac0fde5900a607118e82b5d6f79543a615301e7`，构建与源码绑定见 `.local/racing-followup/final-build-identity.json`。Porsche、Ferrari、Lamborghini、McLaren 取自 `.local/racing-nfs-browser/final-continuous-five-01/`；Corvette 取自同一构建的独立复录 `.local/racing-nfs-browser/final-ccor-isolated-01/`。逐车型来源映射保存在汇总 JSON 中，未将整组首次运行改标为通过。

| 车型 | 完整录音 | 声部数 | 怠速／原地负载最长近静音 | 加速／制动最长近静音 |
| --- | ---: | ---: | ---: | ---: |
| Porsche 911 Turbo | 36 秒 | 5 | 0／0 ms | 0／0 ms |
| Ferrari 550 Maranello | 36 秒 | 8 | 0／0 ms | 0／0 ms |
| Lamborghini Diablo SV | 36 秒 | 3 | 0／0 ms | 0／0 ms |
| McLaren F1 GTR | 36 秒 | 5 | 0／0 ms | 0／0 ms |
| Chevrolet Corvette | 36 秒 | 6 | 0／0 ms | 0／0 ms |

五车型全录音范围的最长近静音均为 6 ms，包含录制起始段；无 PCM 裁切。所有已接受时间线保持唯一原生 sink、运行中的 context、原生速率 1、无 fallback。每行包含车型的完整声部集合，源摘要匹配原始 FLAC，各声部 `starts` 始终恰为 1。实际平台静音后释放原生 sink，可信手势可恢复当前发动机，离桌后旧 sink 脱离、暂停并清空音轨。

汇总与五个可播放 WAV 位于 `.local/racing-nfs-browser/final-continuous-selected-analysis-01/`，WAV 文件名为 `<车型 ID>-actual-continuous-mix-native-master.wav`。汇总文件 `actual-continuous-audio.json` 保留原始 WebM、WAV 和捕获 JSON 的 SHA-256、主音量处理、各阶段完整指标及样本选择。独立复核 `.local/racing-nfs-reference/final-continuous-review.json` 另行检查原始源／录音摘要、完整声部、平台生命周期，并用 ffmpeg／NumPy 再次从真实混音计算相同 RMS 结果。

首次 Corvette 样本未通过：实际状态包接收出现 3,035 ms 空窗，19.228 秒时既有快照过期保护停止发动机，21.416 秒恢复后创建新 sink；原始 WebM 仅长 19.2 秒。原样本与 `.local/racing-nfs-reference/final-ccor-stale-state-diagnostic.json` 保留。停止另外三条异常验收进程、取消并行浏览器旁路检查后，独立复录的最大状态接收间隔为 149 ms，完整 36 秒通过，期间未修改运行时代码。证据确认中断与快照失效保护对应；首次状态空窗的外部根因尚未证实。

## 可重复执行

先执行隔离回归和类型检查：

```sh
node --experimental-strip-types --test \
  web/tests/platform-audio-continuous.test.mjs \
  web/tests/platform-audio*.test.mjs \
  web/tests/audio-preferences.test.mjs \
  games/racing/tests/classic-audio-continuity.test.mjs \
  games/racing/tests/classic-audio.test.mjs
node web/node_modules/typescript/bin/tsc --project web/tsconfig.json --noEmit --pretty false
```

上述范围当前为 149 项通过、0 项失败，类型检查通过。日志位于 `.local/racing-nfs-reference/audio-continuity-final-tests.log`。实现前失败场景见 `games/racing/tests/classic-audio-continuity-failures.md`。

原始资源审计与原生循环机制诊断：

```sh
python3 -B scripts/racing/nfs-engine-source-audit.py --examples
python3 -B scripts/racing/nfs-native-container-probe.py \
  --out .local/racing-nfs-reference/native-container-reproduction
```

浏览器验收必须选择父流程构建的冻结前端，以及与之对应的 `--build-identity`。`check-nfs.py` 不构建前端；输出目录必须为仓库内的新目录。音频分析需 ffmpeg 和 NumPy，下例使用已位于仓库内的分析环境。

```sh
python3 -B scripts/racing/check-nfs.py --phase audio-continuous-all \
  --dist web/dist --build-identity .local/racing-followup/final-build-identity.json \
  --out .local/racing-nfs-browser/audio-continuous-reproduction
.local/racing-native-audio-analysis/venv/bin/python -B scripts/racing/analyze-native-audio.py \
  --continuous .local/racing-nfs-browser/audio-continuous-reproduction \
  --out .local/racing-nfs-browser/audio-continuous-reproduction-analysis
```

本次原始样本的独立复核可直接重跑：

```sh
.local/racing-native-audio-analysis/venv/bin/python -B \
  .local/racing-nfs-reference/verify-final-continuous.py
```

Corvette 独立复录命令已保存在 `final-ccor-isolated-01/report.json`，报告为通过；其临时浏览器 profile、socket 与 fixture 已清理。较早 `05e20a94` 构建的 Porsche／Lamborghini 短测只作诊断，未纳入本节最终结果。
