# 新增音乐结构与原生无损承载研究

新增八首音乐均取得全部 9 条轨道的重复回跳证据。原内核与观察内核合计比较 130,480 个音乐视频步骤，RAM、视频、928 字节驱动状态、PCM 摘要与样本数量全部一致。研究用时值／控制流模型在延长捕获中逐条匹配 21,147 条序列命令，包括 `95` 的中途变速。没有导出精确循环成品，也没有修改游戏、平台音频接口或既有音效资产。

机器索引为 [audio-music-survey-validation.json](../../../games/yuyuhakusho/data/audio-music-survey-validation.json)；原生载体另见 [audio-native-carrier-validation.json](../../../games/yuyuhakusho/data/audio-native-carrier-validation.json)。`pass` 表示本研究断言通过，`goal_complete`、`release_ready` 与 `exact_pcm_loop_assets_ready` 仍为 `false`。

## 曲目范围与原版步骤

本轮补齐 `83/84/85/86/88/8a/8d/95` 的 header 和实际序列执行。它们与已有 `82/87` 一起覆盖八场地和规则 0 对手覆盖的音乐编号；`8f` 保留既有有限结算音乐证据。这里不为曲目补写未经确认的曲名。

第一轮每首上限为 4,096 步，共 32,768 步。第二轮按每条轨道两个节奏循环设定不同上限，共 88,377 步；`8d` 再按已观察的原样初始化边界补至 9,335 步，另存证据，不覆盖第一轮或原 8,431 步记录。所有运行都是明确的私有音频隔离：先复核原 2,400 帧，再清空非音频世界和延迟生产者，通过原命令 `04` 建立精确静音检查点。所有恢复显式补回 `refresh_cycles`；不将这个同进程检查点当作可移植音频存档。

每次原版运行与观察运行使用同样的隔离输入，逐步骤比较完整 RAM／视频及 PCM。世界池和非音乐效果行全程为空，因此这些音乐不包含其他游戏音效的混音。核心、ROM、观察桥、脚本和每个捕获文件的 SHA 均由索引绑定。

## 时值、共享节拍与周期

音乐表 `3a492` 的指针和 `384d8/385xx` 的相对寻址证明八首均有 6 条 DAC／FM 记录及 3 条 PSG 记录；第一条包含 DAC，不能解释成六个独立 FM 声部。`F7` 的八个计数字节和两格调用返回地址共享同一对 word，研究模型没有另存两份可变 owner。

研究模型只恢复域内音符时值、有限计数、子程序和共享 tempo 时钟，尚未导出完整的音高、音色自动化或独立 DSP。该模型不执行 M68K／Z80，不进入游戏运行时；原音序源地址仍只属于研究资料。首轮 72 轨、7,498 条命令和第二轮 21,147 条命令均与原观察逐项匹配，比较保留步骤、轨道、源位置与命令顺序。首个模型明确拒绝了 `95` 的迟发 tempo 命令，真实失败保留；补齐共享节拍后，冻结的验收脚本原样通过。

| 编号 | 单轨节奏周期（quanta） | 共同节奏／tempo 周期（视频步） | 本轮完整原样 driver 周期 |
| --- | --- | ---: | --- |
| `83` | 主轨 3,768；噪声 24 | 4,239 | 通过；从步骤 14 后连续相等 4,462 对 |
| `84` | 主轨 3,840；噪声 96 | 76,800，结构候选 | 未捕获完整共同窗口 |
| `85` | 主轨 2,784；噪声 96 | 4,176 | 通过；从步骤 595 后连续相等 4,304 对 |
| `86` | 主轨 3,048；噪声 2,840 | 1,623,060，结构候选 | 未捕获完整共同窗口 |
| `88` | 各轨 5,172 | 21,981，结构候选 | 未捕获完整共同窗口 |
| `8a` | 各轨 5,568 | 41,760，结构候选 | 未捕获完整共同窗口 |
| `8d` | 各轨 2,886 | 3,367 | 补采通过；从步骤 2,472 后连续相等 3,496 对 |
| `95` | 各轨 3,408；中途 tempo 变化 | 3,767，原版观测 | 通过；从步骤 6,327 后连续相等 6,390 对 |

固定 tempo 非零时，原 `389a8` 每 N 个视频步骤给活动轨的等待计数加 1，再由各轨减 1，形成一个暂停节奏推进的步骤。由此，`84` 噪声回跳间隔会交替出现 101／102，不能强行给它填同一个相邻差值。共同节拍周期还必须让 tempo 相位回归。`95` 则在 DAC／FM 路径中执行 `EA`，实际经过 `18/8/6/5/10` 等 tempo 值；按 header 的 `10` 计算整曲会出错。

`86` 的结构候选意味着两次共同窗口将超过 15 小时原音乐时间，本轮没有照此盲目录制。对这类长共同周期，研究已证明的是每轨控制流、实际回跳和已捕获步骤的共同节拍，不宣称整段原样驱动或 PCM 共同周期已经采完。

## 新候选仍不能成为精确 PCM 循环

完整 driver 周期相等的四首，其相邻候选周期 PCM 仍不相等：

| 编号 | 不同样本值 | 最大整数样本差 |
| --- | ---: | ---: |
| `83` | 6,229,127 | 16,397 |
| `85` | 6,136,271 | 11,050 |
| `8d` | 4,949,112 | 9,612 |
| `95` | 5,531,303 | 14,366 |

完整 driver 回跳没有恢复 YM 相位、PSG 噪声或采样器状态。比较使用原输出的完整立体声样本，不以 RMS 容差或听感判定循环。较长共同周期未被搜索完，也没有因此宣称不存在任何更长 PCM 周期。

已有 `82/87` 的音序视频相位与 44.1 kHz 采样相位若要同时精确回到同一起点，仅时钟有理数对齐就分别至少需要 715,909 轮，约 531／1,590 天及 8.1／24.2 TB 原始立体声 PCM。这个算术约束只针对同时对齐两种时钟，不证明不存在更短波形巧合周期；它说明盲录一个固定“超级 WAV”不是可用的实现路线。

## 原生无损分片载体已经实测

真实 Chromium 152 的能力探测接受 `audio/mp4; codecs="fLaC"` 的 HTMLAudio 与 MediaSource；WAV 和裸 FLAC 不支持 MSE，WebCodecs 的编码器也不支持 FLAC。能力声明之后另做了实际播放验收。

原型使用已验收 `82` 连续 PCM 的前 12 秒，保留 529,200 个 44.1 kHz 双声道采样帧及原前导延迟。离线 FFmpeg 9.0.1 无损编码为 FLAC／fMP4，拆成初始化段及 12 个媒体段。整文件解码、按片段重组后解码都与原 PCM 逐 byte 相同，PTS／DTS 和片段样本范围完整连续。

真实浏览器只创建一个原生 HTMLAudio 和一个 MSE SourceBuffer，播放开始后继续追加后续段，实际跨越 11 个边界。`playing` 之后的 `waiting/stalled/seeking` 为 0，没有换 src、重新建播放器、seek 或创建 AudioContext。native captureStream／MediaRecorder 一直活到最终约 12 秒的唯一 `ended`，未在首段边界提前结束。其 WebM 录音是 48 kHz Opus，仅支持时间轴和生命周期观察，不作为原 44.1 kHz PCM 等价证据。

验收还拒绝了样本范围缺口、修改后的 PCM、修改后的分片和真实浏览器不支持的 ALAC MIME。前三项在播放前按完整性契约拒绝；ALAC 路径在实际浏览器明确失败，范围没有混同。

这证明所测 Chromium 中，连续无损容器和原生媒体承载是可行路线。没有证明独立合成器、无限 BGM、低延迟动态混音、扬声器逐样本输出或 Safari／手机兼容已经完成。原型在开始前预缓冲约 2.04 秒已知音乐；这个设置不能直接用于需要即时响应的未知效果声。

## 完整实现的下一步

1. 将研究音序转为没有 ROM 源地址的 typed 音符、音色和参数事件；有限 `F7/F8/F9` 路径可在提炼时展开，真正的轨道循环由各自周期推进，共享 tempo 保留独立相位。当前时值模型只是这一步的验证基础。
2. 实现独立 Go 的 FM／PSG／DAC 合成与原版抢占、暂停和混音规则，连续保留相位、包络、噪声及重采样状态，不在轨道回跳处复位。禁止执行原 CPU 程序或把 RAM 镜像放入运行时。
3. 为浏览器运行时提供 Go／Wasm 无损 FLAC 编码和 fMP4 分片封装，按精确样本计数推进。浏览器自身 FLAC 编码器不可用；本轮 FFmpeg 只是研究工具。
4. 由平台 `BoardProps.audio` 管理流、原生媒体、静音、暂停、owner、离桌及错误；游戏不直接创建播放器。先验证短分片、有限前瞻和已缓冲数据的修订对响应延迟的影响，再决定动态混音如何进入同一时间轴。
5. 在真实 Safari／手机、循环至少两个共同边界、暂停／恢复、资源失败和平台生命周期中复验。固定 WAV、交叉淡化、有损 AAC／Opus 和 `ended→seek→play` 不能悄悄替代所需保真目标。

## 复跑

新采集均写入新的 worktree 内目录，不能覆盖现有结果。先做首轮和时值结构验证：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/survey_music_sequences.py \
  --out .local/md-yuyuhakusho/audio-music-review/probe

PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/validate_music_survey.py \
  --report .local/md-yuyuhakusho/audio-music-review/probe/survey.json
```

`check_music_score_structure.py` 与 `check_music_survey_full.py` 的规范夹具根是 `.local/md-yuyuhakusho/audio-music-survey/`；在该目录保留时可直接运行。完整新采集使用 `capture_music_tracks.py --out <新的工作区目录>`，按结构保存每首的独立上限。捕获摘要中的 `common_period_frames` 是首轮回跳差值的候选，最终共同节拍必须按本页结构表及对应验证结果解释。

`8d` 的额外原样边界来自实测步骤 `2472` 加两个 `3367` 周期及 128 步余量。可通过同一个已冻结的采集函数在新目录复验，不放宽首轮 CLI 的 4,096 步上限：

```sh
PYTHONDONTWRITEBYTECODE=1 PYTHONPATH="$PWD/scripts/yuyuhakusho" python3 - <<'PYCODE'
from reference import ROOT
from survey_music_sequences import run
out = ROOT / '.local/md-yuyuhakusho/audio-music-review/raw-proof-8d'
out.mkdir(parents=True, exist_ok=False)
report = run(out, 2472 + 1 + 2 * 3367 + 128, [0x8d])
assert report['songs'][0]['raw_driver_recurrence']['verified']
PYCODE
```

原生载体可在任意新的工作区目录复现：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/build_music_native_carrier.py \
  --out .local/md-yuyuhakusho/audio-carrier-review/fixture

PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_music_native_carrier.py \
  --out .local/md-yuyuhakusho/audio-carrier-review/e2e \
  --fixture .local/md-yuyuhakusho/audio-carrier-review/fixture
```

`probe_music_native_carrier.py` 只查实际 MIME／编码能力；它不能代替第二条真实播放验收。最终 JSON、数据文件和脚本的来源由机器索引完整绑定。全部媒体、捕获上下文和浏览器文件留在 `.local`，未发布或进入正式构建。

本轮清理了本 owner 的 35 个已关闭浏览器 profile／临时目录、重复负向夹具、空参考目录及自有字节码，共 41,541,722 bytes。原始音乐／PCM 证据、真实失败与最终日志、FLAC 片段、录音及旧媒体保留；清单为 [audio-music-study-cleanup.json](../../../games/yuyuhakusho/data/audio-music-study-cleanup.json)。
