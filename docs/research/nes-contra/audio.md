# 日版魂斗罗音频资源与请求规则

2026-10-25：已从固定日版 ROM 的原声音驱动导出 **46 项本地资源**：29 个效果音、5 个有限转场曲、7 首音乐、4 个打击乐声音和 1 个持续场景效果，另有 **110 份物理通道分轨**。原版代码和数据没有改写；开发包装只位于经验证的空白区域，用 FCEUX 原 APU 输出 PCM。

这些是本地研究候选，未上传 OSS，也没有作为正式回退资源接入。驱动状态周期、波形接缝和浏览器实际混音是不同验收项：重复录制与来源校验已通过。七首音乐与开场 `$14` 的 23 条派生循环已通过数值接缝与原生媒体边界检查；独立控制器和平台适配已通过原版记录及浏览器验收。跨媒体同步、实际设备听感和完整对局混音仍未验收，不能据此宣称逐样本还原。

## 原版来源与入口

ROM SHA-256：`6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe`。音频代码位于 PRG 8 KiB bank 2／3，运行时映射到 `$8000..$BFFF`；固定 bank 14／15 映射到 `$C000..$FFFF`。

| 日版入口 | 用途 | REA 证据 |
| --- | --- | --- |
| `$F9BC` → `$87A2` | 请求一个原声音 ID，按 slot 判定是否接受 | `ev_5554287f70ce87694caf2b0e255945c28784299ecbf9b7ca1acec11107b001ee` |
| `$F9D4` → `$8719` | 清六个声音 slot，重置／静音通道 | `ev_e08468d95ffdf4089bc624a0f88f5e2d3dfb5b3d5c14a12676026f0450989006` |
| `$80D5` | 原版逐帧音序更新 | `ev_6a2ed3b2bf0081386bb276de49f9405700f628e02dba6091e997a365f97b84e1` |
| `$809F` | 读取游戏暂停状态，静音／恢复声道 | 见 [完整索引](evidence/audio-index.json) |
| `$CC7D` | 开局选择当前关卡音乐 | 见 [完整索引](evidence/audio-index.json) |
| `$CDF8`、`$CD1F` | 通关阶段／Game Over 音乐请求 | 见 [完整索引](evidence/audio-index.json) |

完整 REA bundle 保存在 [audio-rea.json.gz](evidence/audio-rea.json.gz)。输入静态切片从已确认的 `$80D5` 或 `$806D` 指令开始；前方音量指针区未映射进该分析切片，不将此限制隐藏。离线录制使用完整的原音频 bank 与固定 bank。

## 已确认的声音与场景

声音 ID 必须使用日版编号。普通枪为 `$1A`，不能使用美版 `$10`；落地为 `$0B`，死亡为 `$72`。下列效果的片段和分轨均有来源摘要：

| 日版 ID | 事件 | 目标逻辑 slot |
| --- | --- | --- |
| `$0B` | 落地 | 4、5 |
| `$1A` | 普通枪 | 4、5 |
| `$1C` | M 机枪 | 4、5 |
| `$24` | L 激光枪 | 4、5 |
| `$26` | F 火焰枪 | 4、5 |
| `$28` | S 散弹枪 | 4、5 |
| `$2F` | 共用敌人爆炸，包括枪盒销毁 | 4 |
| `$38` | 武器拾取 | 4 |
| `$72` | 玩家死亡 | 4、5 |
| `$74` | 暂停提示声 | 4 |

纯按键的 M 拾取场景实际观测到普通枪、落地、爆炸、拾取、M 枪和死亡请求。Start 暂停／恢复场景实际观测到 `$74`；其他武器的编号由日版武器表和原版武器调用验证。八个关卡初始化的只读声音钩子记录与原初态记录的完整 RAM 逐字节一致，见 [请求记录](evidence/audio-requests.json)。

| 场景 | 日版 ID | 驱动周期候选 |
| --- | --- | --- |
| 丛林、机库（第 1、7 关） | `$46` | 2,976 tick，约 49.518458 秒 |
| 瀑布（第 3 关） | `$4A` | 1,920 tick，约 31.947392 秒 |
| 雪地（第 5 关） | `$4E` | 1,920 tick，约 31.947392 秒 |
| 能量区（第 6 关） | `$52` | 1,296 tick，约 21.564467 秒 |
| 异形巢穴（第 8 关） | `$56` | 1,386 tick，约 23.062018 秒 |
| 基地（第 2、4 关） | `$5A` | 2,688 tick，约 44.726349 秒 |
| 基地 Boss | `$5E` | 2,100 tick，约 34.942449 秒 |

八关初始音乐编号来自真实原版初始化请求；基地 Boss 的切换由 `$DD28` 的日版代码确定，尚未通过自然走完整个基地验证。

有限曲另包含 `$62` 通关曲、`$66` 最终关通关曲、`$6E` Game Over。不能在所有关卡 Boss 倒下时立即播放 `$62`：日版 `$CE32` 的八项表实际是 `00 00 00 00 62 62 00 66`，零项表示此入口没有立即请求，其他演出入口仍需分别恢复。原版 Game Over 的 `$CD96` 明确请求 `$6E`。

## 请求不是可以随意叠加的播放列表

`$87A2` 对每个目标 slot 分别比较新 ID 与当前 ID：新 ID 更低时忽略；相等或更高时重新初始化该 slot。相同 ID 跨 tick 是真实重触发，不能全局去重。

16 组受控同 tick 原版请求验证见 [audio-priority.json](evidence/audio-priority.json)。它们仅在开发包装中顺序调用原请求函数，不改写结果：

- 散弹 `$28` 连续请求 5 次，在第一次音序更新前的完整驱动状态与请求 1 次一致，不代表 5 个并发声音。
- `$1A,$1C` 与 `$1C,$1A` 都留下 slot 4／5 的 `$1C`。
- `$1A,$2F` 与 `$2F,$1A` 都留下 **slot 4 为爆炸、slot 5 为普通枪噪声**。全局只选最大 ID 会丢声音；把两个完整 WAV 叠加又会保留不该存在的枪声脉冲。
- 通关曲 `$62` 活跃于音乐 slot 1 时，普通新效果被拒绝，但落地 `$0B`有专门例外。`$77` 还存在终局特殊保护，不能只靠通用大小比较覆盖所有情况。

逻辑 slot 与物理通道也不同。音乐 slot 0／1／2 对应 pulse1／pulse2／triangle；音乐 slot 3 是打击乐音序，实际发出低优先噪声 `$02` 和 DPCM `$7F..$81`。效果 slot 4 占用 pulse1，slot 5 占用 noise。DPCM 是独立物理通道：`$87A2` 先把 `$7F..$81` 分派到 `$88DA`，绕过普通 slot 比较。

受控「能量区音乐＋死亡」记录证明：死亡仍占用 slot 5 时，低优先 `$02` 被丢弃，而 DPCM 请求和 `$4015` 启动写入继续发生。因此不能把整个打击乐混音静音，也不能在高优先效果结束后补播被丢弃的噪声请求。

## 给平台音频接口的接入约束

平台提供 owner 隔离的 `track`、`stopTrack`、`muteTrack`、`pauseTrack`、`resumeTrack`、`seekTrack`，游戏只通过 `BoardProps.audio` 使用它们。原声音优先级和音序时间线属于魂斗罗规则层，平台不解释 ROM ID。

建议保留原始请求顺序，再按 slot 计算接受事件。每个 tick 的重复同码请求可压成每 slot 最后一次接受事件；不同 tick 的同码接受应生成新的 `playbackID`。ID 绑定房间／局 epoch、接受事件和 slot，普通快照或取消全局静音不得重播旧 ID。

- 音乐脉冲通道被效果占用时，背景相应通道继续计时但静音，不能暂停后恢复导致错拍。
- 噪声与 DPCM 可按 [原始打击乐时间表](evidence/audio-percussion-timeline.json.gz)调度；低优先噪声请求走同一 slot 5 规则。时间表保留同帧原请求次序，七首音乐的完整周期事件序列均与下一周期相同。
- Start 暂停需冻结已有旋律／效果的进度，提示声 `$74` 仍可播放一次。`$74` 本身不是暂停整个音频系统的命令；原版另读 `$25`。DPCM 在暂停时会被停止，其恢复边界须按下一次原请求处理。
- `$F9D4/$8719` 是显式重置，不要伪装成已核验的声音 ID 0。
- slot 的释放时间不等于 WAV 时长。例如普通枪 slot 4／5 的释放为 52／34 次原更新，而文件可能含滤波尾部或已静音段。[资源清单](../../../games/contra/assets/audio-source.json)保留每个 slot 的真实 tick 生命周期。
- 原版 NMI 先更新声音，再执行世界逻辑。游戏中 `$F9BC` 请求在下一次声音更新生效；音乐内部生成的打击乐属于当前声音更新，不能统一再延迟一帧。

原生媒体分轨仍有实际限制：NES/FCEUX 的 pulse 和 TND 混合为非线性，单独渲染后的线性叠加不等于原完整混音；效果覆盖后的振荡相位也有差异。分轨有原版来源，不等于实际组合已经通过验收。当前不提供误导性的「逐样本还原全部混音」结论。

## 录制、预热与验证

输出为 44,100 Hz、单声道、16 位 PCM，FCEUX 渲染音量 192，SDL dummy 声音设备不向用户扬声器播放。采用离屏 Qt、软件画面、私有工作区配置及写入 sandbox。`audio_wave_bridge.c` 仅调用 FCEUX 现有 WAV 录制器并记录真实 PCM 游标，不改 CPU、APU 或驱动状态。

冷启动探针发现，在目标请求前有 1,440 个不属于目标声的非零样本，峰值 11,788；单纯按第一个非零样本剪裁会保留启动瞬态。该批候选已废弃，[审计记录](evidence/audio-cold-start-audit.json)保留原始前缀及摘要。最终录制先让原驱动静默运行 120 次更新，再只请求一次目标声；验证请求前 100 ms 均低于阈值 8，输出从实际请求的 PCM 游标之后剪裁。

开发 NSF 仅在音频 bank 原有 `FF` 空隙放入 15 字节 init 包装与 26 字节 play 包装，使用 `$05F0/$05F1` 作为开发计数器。完整原驱动／数据和固定 bank 不变。原音乐实际使用 `$FC00` 长度 81 与 `$FCC0` 长度 593 等 DPCM 区间，验证每个读取范围都来自未修改的固定 bank；未禁用 DPCM 来简化录制。

46 项资源各录制两次，完整 PCM、APU 写入、驱动 RAM、调用与样本游标逐字节一致。效果、有限曲和打击乐验证非静音、无削波及尾部完整。110 分轨使用 FCEUX 主机侧通道音量隔离，逐份确认原驱动／APU／样本时间线与完整混音一致，两次分轨 PCM 相同，剪裁范围与完整资源相同。

七首音乐均找到 130 字节驱动状态完全相等的周期；这不保证 APU 振荡相位或 PCM 接缝相同。未加工的候选接缝仍有明显跳变，因此 `pcmSeamAccepted` 保持 `false`。派生循环的接缝和原生边界证据见 [循环资源说明](audio-loops.md)；此处原始候选的 `pcmSeamAccepted=false` 不改写。压缩分发、实际设备听感与完整对局混音仍待验收。

完整结尾剧情曲与全部对局中的密集混音尚未覆盖。已补录开场、过场、坦克、巨人、桥梁、冰雷、电击和 Heart 结束等当前独立规则会发出的声音；持续开场 `$14` 的两条循环已单独通过数值和原生边界验收。新增声音请求应继续核对原版入口与数据，不按编号或相似名称猜测素材。

## 产物与复查

- [audio-source.json](../../../games/contra/assets/audio-source.json)：46 项资源、110 分轨、原 slot 定义、优先级和 tick 生命周期、音乐周期及本地文件 SHA。
- [audio-verification.json](evidence/audio-verification.json)：来源、数值检查和明确未证明的范围。
- `.local/contra-audio/export/`：预热后的完整原录、剪裁 WAV、调用／APU／驱动记录及清单。
- `.local/contra-audio/stems/`：物理通道原录、同范围 WAV 及分轨清单。noise／DPCM 连续分轨也可用于对照，但不能代替请求级优先处理。

```sh
python3 -B scripts/contra/audio_prepare.py
node scripts/contra/static_rea.mjs .local/contra-audio/spec.json
node scripts/contra/static_rea.mjs .local/contra-audio/extra-spec.json
python3 -B scripts/contra/audio_export.py
python3 -B scripts/contra/audio_verify.py
python3 -B scripts/contra/audio_priority.py
python3 -B scripts/contra/audio_stems.py
python3 -B scripts/contra/audio_stems_verify.py
python3 -B scripts/contra/audio_catalog.py
```

已有录制目录只按完成标记复用，重新采样应选新的私有目录或明确移走本任务旧输出，不得清理其他游戏材料。素材分发许可没有因 ROM 身份核验而自动成立；当前所有音频均保留本地，未发布。

已删除冷启动中间候选，清理约 89 MiB 私有分析缓存／临时 profile。独立重复录制先完成逐字节比较，再对相同内容建立硬链接，减少约 651 MiB 重复存储；去重后的资源再次完整回读通过。[清理记录](evidence/audio-cleanup.json)列出具体范围。原始用户 ROM 和其他任务产物未修改。

## 独立控制器与平台适配

`games/contra/engine/audio_control.go` 只保存 JSON 可序列化的声音控制状态。`Queue` 按世界逻辑的原顺序收集 `reset`／`sound`，`Advance` 在下一原声音边界消费；正式运行不执行 ROM、CPU 或 APU。全部 129 个合法声音 ID 的 per-slot 生命周期、请求与结束回调来自 [原版控制库](evidence/audio-control-library.md)，与是否已有 PCM 无关。

新增失败场景先于实现列入测试。205 个独立案例、298,300 tick 全部匹配原版六个活动 slot；已有 8,140 tick 组合场景和 1,275 个请求 carry 组合继续通过。`go test ./games/contra/engine -run '^TestAudio' -race -count=1` 通过。carry 是游戏算术的输入，`RequestCarry` 为纯投影；`Clone` 后接阶段前缀再 `QueueSound` 不改变权威队列。

世界状态每次 `Advance` 前同步 `AudioState.Context.EndPlayersAlive = Campaign.AliveAtExit`。它对应 `$CDF8` 保存的 `$0194`，不是当前存活人数。`$77` 的 slot 5 结束时，只有该值大于 0 才请求 `$62`；共用音序的 `$7A/$7B/$7E` 没有这项回调。`$6A` 的 slot 3 结束后请求 `$6E`。旧 slot 先清除，新回调建立的较早 slot 不在同 tick 再更新。triangle 结束和暂停边沿会停止 DPCM。

`games/contra/web/audio.ts` 只使用 `BoardProps.audio`。构造时绑定当前对局 epoch；首次快照与历史缺口只恢复音乐，按媒体循环的实际周期换算位置，不补播旧效果。正常事件按序同步提交，不等待媒体 Promise 后才处理下一事件。音乐被效果占用时 soft mute，原音乐时钟继续前进；重新开始的同码效果使用服务端提供的新 `playbackID`。相同 ID、旧 epoch、倒序快照，以及静音／隐藏／停止留下的 tombstone 不被重新生成的 ID 绕过。

资源解析器接收已记录的路径与 SHA-256，由调用方提供已验证的本地测试 URL 或不可变 CDN URL。连续声音仅接受 `pcmSeamReady`、`nativeBoundaryReady`、`loopReady` 全部为 true 的分轨；原始连续 noise／DPCM 混音不作为循环轨。资源缺口由 `missingResources` 暴露，不使用猜测音效。开场效果 `$14` 属于持续效果，已解析两条通过验证的循环分轨，但不按持久音乐恢复旧播放。

[浏览器验收结果](evidence/audio-native-adapter-v3/result.json)使用生产 Go 控制器生成的快照、生产适配器和平台 `GameAudio`，通过 11 项真实 Chromium 原生媒体检查：分轨启动、枪声抢占、暂停和 cue74、恢复、场景重置、历史缺口、旧 epoch、全局静音、原生标签页隐藏及离桌清理。保留 PC 和 320px 截图。这是本地音频接入夹具，不是完整游戏或实际扬声器听感验收。

```sh
. scripts/env.sh
go test ./games/contra/engine -run '^TestAudio' -race -count=1
node --test games/contra/web/audio.test.mjs
python3 -B scripts/contra/audio_accept.py --out docs/research/nes-contra/evidence/audio-native-adapter-fresh
```

本轮清理约 145 MiB：移除本次私有模拟器配置、Swift 缓存及失败夹具；已逐字节核验的重复录制改为硬链接，主记录与每次录制的路径均保留。测试先红后绿的记录压缩保存在 [测试证据](evidence/audio-control-test-records.json.gz)，清理范围见 [清理记录](evidence/audio-control-cleanup.json)。[控制与接入总验收](evidence/audio-control-verification.json)绑定最终源码摘要。

## 60 Hz 平台集成更新

2026-10-26 的 [音频集成审查](audio-integration-review.md) 补齐了异步媒体位置校准、原录到 60 Hz 的播放速率、明确的后台持续音乐恢复，以及终局 `$6E` 完整曲续播。后台恢复只适用于仍属当前权威状态的持续音乐，不补播有限声音；显式停止与普通取消静音仍保留防重播约束。最终新增验证见 [本轮结果](evidence/audio-integration-review-verification.json)，早前 11 项原生夹具记录保留为历史证据。
