# 原版音频资产研究

本轮得到 134 个经原版驱动结束状态、精确静音尾部和双次 PCM 回放验证的有限 WAV。11 名角色动画目录引用的 78 个非零声音编号均有对应有限资产。默认场地音乐 `82` 与酎的音乐 `87` 仅取得限长片段，尚未确认循环。完整音频播放调度与平台接入仍未实现，`audio-validation.json` 因此保留 `goal_complete: false`、`release_ready: false`。

研究脚本为 [`extract_audio_assets.py`](../../../scripts/yuyuhakusho/extract_audio_assets.py)，没有修改共享 `reference.py`。最终媒体保留在工作树内的 `.local/md-yuyuhakusho/audio-assets-final/`，未上传、未嵌入服务器。原 ROM、模拟内核与原音频程序只用于开发对照；正式游戏通过平台 `BoardProps.audio` 播放离线媒体。

## 先列出的失败场景

实现前失败场景：把 255 项优先级表等同于 255 个可播放音效；将停止、暂停、淡出或驱动控制误作声音；把世界混音直接裁成单音效；未清除旧 BGM／延迟命令；读取回调缓冲后生命周期已结束；声道、采样率、端序或样本数量错误；只比较 RAM／视频而未比较 PCM；假定原存档恢复也恢复混音器／重采样相位；用任意静音阈值猜音乐循环；把限长截断误称完整语音或完整循环；观察器改变原版运行；向正式服务器或 worktree 外写入音频；最终浏览器绕过平台 audio 创建另一套播放器。

## 驱动分派与已恢复事件

以下编号和地址均为十六进制。静态证据位于 [`audio-dispatch.txt`](static/audio-dispatch.txt)、[`audio-control.txt`](static/audio-control.txt) 与 [`audio-z80.txt`](static/audio-z80.txt)。M68K 原始镜像走 direct Ghidra headless 备用路径；这些指令清单不是 REA 原生分析结果，也不代表执行覆盖。Z80 文件使用 `z80:LE:16:default`，文件头补充了实际固件来源，未改动指令。

分派入口为 ROM `37cb4`。优先级表中的每个编号不等于一个声音资产。

| 命令 | 入口或表 | 已确认含义 |
| --- | --- | --- |
| `00` | `38a7a` | 上传原 Z80 固件并复位，不是媒体 |
| `01` | `38916` | `f806/f804` 写入 `03/28`，不是媒体 |
| `02` | `38808` | 停止主效果声道，不是媒体 |
| `03` | `388ac` | 停止次效果声道，不是媒体 |
| `04` | `38a2c` | 清理驱动并 key-off，本轮用于建立静音 |
| `05` | `38928` | `f806/f804` 写入 `20/32`，不是媒体 |
| `06..7f` | `380a8 + (code-6)*8` | PCM 描述符，共 122 项；`7f` 单独列为静音诊断 |
| `81..9f` | `3a492 + (code-81)*4` | 音乐指针表，本轮仅采集 `82/87/8f` |
| `a0..ab` | `3a60c + (code-a0)*4` | 12 个 FM／PSG 序列 |

未列出的命令类别没有做完整提取或行为验收。

PCM 描述符的银行与小端指针按原 M68K／Z80 指令恢复，122 项的头部和编码数据均在 ROM 边界内。位置公式如下；这不是浏览器运行时解码器。

```text
bank = (descriptor[0] << 16) | ((descriptor[1] & 0x80) << 8)
headerPointer = LE16(descriptor[2:4])
headerAddress = bank + (headerPointer & 0x7fff)
header = ROM[headerAddress : headerAddress+4]
sampleStart = LE16(header[0:2])
encodedLength = LE16(header[2:4])
sampleAddress = bank + (sampleStart & 0x7fff)
```

已恢复的请求绑定包括：默认场地音乐 `82`、默认酎对手覆盖音乐 `87`、结算音乐 `8f`、开场 `58/59`、碰撞／防御 `77`、物理命中 `7a/7b/7c`、其他命中 `78`、落地 `7d`、酎 KO `3f`、酎结算语音 `6f`。这些绑定不包含未经核验的台词转写或曲名。动画目录的 78 个编号是数据引用覆盖，不能证明所有引用都能在正常对局到达。

## 采集方法与结果

音频回调在返回前复制 PCM，格式为 44,100 Hz、双声道、有符号 16 位小端。每段 WAV 保留原请求到发声的延迟和静音尾部，没有裁切或归一化。

隔离采集先冷启动至第 2,400 帧，并逐帧核对完整固定 RAM／视频摘要。随后仅在私有参考进程清空非音频对象池 `8000..<a000`、延迟世界生产者 `a030/f560/f562` 与暂停请求 `f807`，通过原队列提交控制命令 `04`，再运行 120 帧。驱动无活动行、Z80 标志全部清零且最后 16 批 PCM 精确为零后，保存一份静音检查点。每个声音均从同一检查点恢复并采集两次，期间要求世界对象池始终为空。

驱动活动行位于 `f840..<fba0`，步长 `30`，首字节 bit 7 表示活动。检查的 Z80 标志包括 `1ffc/1ffd/1ffe/1f86/1fa6`；固件 `06ce..06fe` 的原生结束路径清除 `1ffc/1ffd`。有限资产必须观察到忙状态，再观察原生标志和活动行全部结束，连续至少 16 个视频步骤保持空闲，且最后 8 批 PCM 每个字节均为零。没有使用幅度阈值推断结束。

| 分类 | 数量 | 验证范围 |
| --- | ---: | --- |
| PCM 命令 `06..7e` | 121 | 原驱动有限结束，双次 PCM 完全一致 |
| FM／PSG 序列 `a0..ab` | 12 | 原驱动有限结束，双次 PCM 完全一致 |
| 结算音乐 `8f` | 1 | 有限结束，250 个视频步骤，WAV 约 4.172 秒 |
| 静音描述符 `7f` | 排除 | 编码长度为 1；600 步全零，未观察到忙状态 |
| 音乐 `82/87` | 仅诊断 | 各采集 7,200 步，约 120.155 秒，结束时仍活动 |

134 个有限资产及 3 个诊断文件都经 Python 标准 WAV 读取器验证声道、采样率、样本宽度、样本数量、完整 PCM SHA-256 和文件 SHA-256。每次采集的双次完整 PCM 字节和逐帧摘要一致。`82/87` 的双次回放一致仅证明固定时长采集可复现，不能将片段尾部接回开头当作已验证循环；本轮也未确认逻辑驱动状态周期。

另有三组原版对照：新增采集回调保持既有 3,600 帧 RAM／视频基线；不带观察器与带观察器内核在 5,500 帧自然流程中具有相同 RAM／视频及逐帧 PCM，观察到 109 次生产者请求和 107 次分派；第 2,400 帧保存并恢复后的 120 帧，PCM、样本数量及 RAM／视频均一致。自然混音 WAV 只用于对照，不作为隔离资产。

参考前端启用内核的快速存档路径，检查点还依赖同进程保留的 blip 缓冲及 `refresh_cycles` 侧载状态。本轮每个会话只使用一份静音检查点；不能把序列化字节单独当作可移植的音频存档。验证使用实际原版内核采集和回放，没有新增合成器、Go 音频包或 Go／Wasm 音频测试。

## 来源与证据位置

固定研究 ROM 为 `.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin`，SHA-256 为 `c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06`。原内核为 `.local/md-yuyuhakusho/tools/genesis_plus_gx_libretro.dylib`，Genesis Plus GX `v1.7.4 60bee4c`，SHA-256 为 `1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da`。内核报告帧率为 `59.92274340431231`。

原 Z80 固件上传源为 ROM `393fe`，长度 `c00`，SHA-256 为 `42ed1d2c4133d88d36c4024ff535a710f064dcb5d791b892eb3a07f6ded6ef27`。静态清单由 Ghidra 12.1.4 PUBLIC 和仓库的 `MdControlProbe.java` 生成。固定脚本、输入、动画目录、原版帧基线、观察桥、观察内核及静态证据的 SHA-256 保存在验证元数据中。

| 路径 | 内容 |
| --- | --- |
| [`audio-assets.json`](../../../games/yuyuhakusho/data/audio-assets.json) | 有限资产、诊断、描述符来源、动画／事件绑定、媒体及 PCM 校验和 |
| [`audio-acquisition.json.gz`](../../../games/yuyuhakusho/data/audio-acquisition.json.gz) | 全部采集记录、逐视频步骤 PCM 摘要及原生终止状态 |
| [`audio-natural-events.json.gz`](../../../games/yuyuhakusho/data/audio-natural-events.json.gz) | 自然流程的生产者／分派事件及 5,500 帧 PCM 摘要 |
| [`audio-frontend-replay.json.gz`](../../../games/yuyuhakusho/data/audio-frontend-replay.json.gz) | 存档前后 120 帧的 RAM／视频和 PCM 对照 |
| [`audio-validation.json`](../../../games/yuyuhakusho/data/audio-validation.json) | 范围、结果、源码绑定、证据文件校验和及发布限制 |
| [`audio-cleanup.json`](../../../games/yuyuhakusho/data/audio-cleanup.json) | 本轮重复采集、临时 Ghidra 目录及冗余日志的清理清单 |
| `.local/md-yuyuhakusho/audio-assets-final/isolated/samples/` | `06..7f`、`a0..ab` 的本地 WAV；`7f` 仅供诊断 |
| `.local/md-yuyuhakusho/audio-assets-final/isolated/music-excerpts/` | `82/87` 片段及已经确认有限结束的 `8f` |
| `.local/md-yuyuhakusho/audio-assets-final/frontend/`、`natural/` | 原版回放报告和自然混音 WAV |

`runtime_asset_ready` 只表示采集的有限音频通过完整性检查。全部媒体的 `published` 均为 `false`；本轮未完成发布许可确认、媒体转码、CDN 发布或平台播放验收。

## 复现与后续边界

在本工作树根目录运行，使用已固定的 ROM、两个内核和 `hook_filter.dylib`；无需重新下载。完整命令会重新采集全部资产，并更新默认 `games/yuyuhakusho/data/` 中的元数据。若要保留交付文件不变，使用示例中的 `--metadata-dir` 将复核结果写入新的 `.local/` 子目录。所有输出必须位于本工作树内。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/extract_audio_assets.py \
  --mode all --out .local/md-yuyuhakusho/audio-review \
  --metadata-dir .local/md-yuyuhakusho/audio-review/metadata \
  > .local/md-yuyuhakusho/audio-review-console.jsonl 2>&1
```

`all` 固定执行 3,600 帧前端基线、5,500 帧自然观察、`06..7f/a0..ab` 隔离采集，以及 `82/87/8f` 最多各 7,200 步的音乐采集。成功退出后，日志末尾报告 `audio_assets_ready: 134`、诊断编号 `[127,130,135]` 和 `animation_bindings: 78`。任一来源、RAM／视频、双次 PCM 或 WAV 校验断言失败即停止；保留新目录便于定位，不覆盖原最终媒体。

定向复核可选择 `--mode probe --frames 3600`、`--mode trace`，或 `--mode isolate --codes all-samples --music ''`。`isolate` 的 `--codes all-sequences` 只采集 `a0..ab`，逗号分隔编号按十六进制解析；`--music 82,87,8f --music-frames 7200` 控制音乐列表和最长采集步数。每种模式都需提供独立的工作树内 `--out`；`--frames` 只影响 `probe`。

后续需要恢复完整音乐循环和生命周期、其他音乐命令，以及原版混音中的声道抢占、暂停和淡出时序。离线单段叠放不能直接保证原版混音等价。平台实现只维护游戏事件到媒体的映射，通过 `BoardProps.audio`、原生媒体播放通道和全局静音控制；静音、隐藏页面、离桌和失败恢复按公共音频规范处理，不另建播放器、`AudioContext` 或游戏专用静音开关。需要播放时间偏移时应使用已记录的首个非零样本位置，另行验证，不能直接把原始前导延迟删掉。
