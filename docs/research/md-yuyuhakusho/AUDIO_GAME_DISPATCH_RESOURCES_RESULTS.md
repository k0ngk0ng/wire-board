# 游戏资源中的声音请求编号

只读审计了当前规则清单绑定的 28 个资源文件，得到 99 个非零声音编号。资源字段中的高位编号只有 `a2/a3/a4/a5/a6/a9/aa`；没有在这些字段中发现 `ac..ef`、`81/89/8b` 或 `f0..ff`。这个结果限定于已列资源与字段，不能推导为全游戏不可达。

严格来源 SHA-256、每个编号的全部 JSON pointer、观察层级和未解决项见 [AUDIO_GAME_DISPATCH_RESOURCES_INDEX.json](AUDIO_GAME_DISPATCH_RESOURCES_INDEX.json)。本报告不修改生产代码、音序 catalog、参数包或主覆盖文档。

## 当前资源可提出的请求

「资源可请求」表示对应 record／type／table entry 被选中并满足 trigger 时可提出声音请求；queue、优先级和后续 driver 仍可能抑制它。不表示全部动画／规则分支已经由正常玩家输入到达。

| 资源 | 使用的字段 | 非零编号与边界 |
| --- | --- | --- |
| `animation-catalog.json.gz` | `/frames/*/data/sound`，4,732 条 frame | 78 个 ID；高位为 `a2/a3/a4/a6/aa`。全部 source set 的有界表项，不是全动作正常可达证明 |
| `chu-world-rules.json.gz` | `/animations/*/data/sound`、`/attachments/animations/*/data/sound`、`/effects/types/*/sound` | 合计 4,965 个 sound 字段，504 个非零引用，81 个 ID；完整逐项位置在索引 |
| `effect-sequences.json` | `/library/types/20..23/sound`、`/library/types/24..31/sound` | 分别为 `a5` 与 `79` |
| `fighter5-projectile-data.json` | `/frames/9/data/sound`、`/frames/16/data/sound`、`/frames/22/data/sound` | 分别为 `37/37/a3` |
| `fighter7-children-rules.json` | `/frames/64/data/sound`、`/frames/70/data/sound`、`/frames/77/data/sound`、`/frames/84/data/sound` | 分别为 `75/75/75/a9` |
| `match-overlays-data.json` | `/tables/character_sounds/*`、`/tables/team_sounds/*` | 19 个表项，18 个 ID：`5e/5f/60/61/62/63/64/65/67/68/69/6a/6b/6c/6f/70/71/72` |

其余清单文件包括各 fighter projectile／attachments、场地规则和 8 个场地资源；没有额外非零声音字段。索引仍逐文件保存零计数和 SHA，避免把「未含 sound 字段」混同于「对应场景不会由代码请求声音」。硬编码、运算得到的 ID 和 dispatcher 归一由主代码审计覆盖。

`source.sound_index` 不是请求编号。原提炼器先用它查声音表，再把结果写入 `data.sound`。例如把此 lookup index 按请求 ID 统计，会产生并不存在于这些资源请求字段的 music／控制编号。资源中的 `sound=0` 是无新请求，不当作 reset 命令。

## 已有原运行观察

以下来自保留的原 core／观察 core 证据，本轮没有重跑 CPU 或 Go。

| 层级 | 范围 | 结论 |
| --- | --- | --- |
| 原 CPU 隔离动画提交 | 4,732 个 frame 的 variant 0，从 ticks0、空 pending／queue 执行；与全 9,464 次原调用验证清单绑定 | 488 次实际非零 queue 写，覆盖资源动画中的全部 78 个 ID。证明该提交例程的字段效果，不证明正常玩家到达全部 frame |
| 正常输入的原请求／分派 | `audio-natural-events.json.gz`，5,500 步；RAM、视频与 PCM 对照一致 | 109 次 request entry、107 次 dispatch entry；raw `81..9f` 请求为 `87/8e/8f/90/92/96`，分派另有 `93` |
| 原 queue consumer 选择 | `audio-timing-natural-phases.json.gz` 的 baseline／pause 两条流 | 185 条 consumer 记录，其中 183 个非 `ff` 选择；两个 `ff` 为无选中 sentinel，不计入控制请求 |
| 既有私有 `f0` 注入 | `audio-dispatch-study/special-f0-{observed,original}.json.gz`，190 步逐项对照 | `f0` 在 B 槽从 pending `80` 到 active `40`。只证明该受控输入的原行为，不能由这条私有 case 单独建立场内来源 |

自然流的菜单／场景原始编号必须与平台场内闭包区分：平台跳过原菜单。这些 raw ID 尚未在本资源分报告中归一为独立曲目或 alias。具体原位置保留如下：

| Raw ID | request 字段／帧／caller | dispatch 字段／帧 |
| --- | --- | --- |
| `92` | `/requests/0/code`，375，`0x4d14` | `/selected/2/code`，377 |
| `93` | 本窗口没有对应 request entry | `/selected/3/code`，557 |
| `8e` | `/requests/5/code`，835，`0x8710` | `/selected/7/code`，836 |
| `96` | `/requests/9/code`，1,465，`0xdede` | `/selected/11/code`，1,466 |
| `87` | `/requests/12/code`，1,531，`0xf3ec` | `/selected/14/code`，1,532 |
| `8f` | `/requests/102/code`，4,579，`0x35a4` | `/selected/100/code`，4,579 |
| `90` | `/requests/103/code`，4,969，`0x118e4` | `/selected/101/code`，4,970 |

上表字段均属于 `games/yuyuhakusho/data/audio-natural-events.json.gz`，SHA-256 为 `8d445cd35d2c70ad850a8ca1de479d97d96dc43878e2502f7c9f228d683e580c`。`f0` 的场内 fighter4 head callback 证据由主审另行绑定；本资源扫描没有 `f0` 字段，不能否定该代码路径。

## 未解决项与复跑

`ac..ef`、`81/89/8b` 以及 `f1..ff` 的合法来源、别名和参数规则，仍需结合主审的代码闭包与其他原观察判断。这里不把优先级表的 255 个条目、私有任意 queue 预填值或隔离枚举输入当作正常游戏请求，也不把本窗口未观察到的编号判为死代码。

复跑只读取现有资源与保存的原证据，输出留在 worktree 的 `.local/md-yuyuhakusho/audio-game-dispatch-resources/`：

```sh
python3 -B docs/research/md-yuyuhakusho/AUDIO_GAME_DISPATCH_RESOURCES_AUDIT.py
```

`resources.json` 保留资源非零字段，`observations.json` 保留分层的原观察，`summary.json` 保留全部来源和 ID 矩阵。公开索引在这些结果上增加产物绑定；没有发布或改动游戏运行时。清理仅移除本轮早期、已由公开审计脚本替代的私有临时脚本，原资源、原观察和最终产物全部保留。
