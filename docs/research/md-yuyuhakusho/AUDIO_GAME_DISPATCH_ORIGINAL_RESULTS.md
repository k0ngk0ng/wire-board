# 原版声音分派只读审计

本次核对全部 256 个 selected ID、22 个不同音乐 header、12 个 primary header、名义 secondary 结构和 PCM descriptor。没有执行新 CPU 捕获，没有修改 production、catalog、既有 claim 文件或主覆盖报告。原版 ROM 为固定 Japan 文件，SHA-256 `c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06`。

完整语义清单见 [AUDIO_GAME_DISPATCH_ORIGINAL_IDS.json](AUDIO_GAME_DISPATCH_ORIGINAL_IDS.json)，来源与范围绑定见 [AUDIO_GAME_DISPATCH_ORIGINAL_EVIDENCE.json](AUDIO_GAME_DISPATCH_ORIGINAL_EVIDENCE.json)。原字节、地址和序列位置仅在 private provenance；本报告区分 dispatcher 分流、表结构存在、静态轨道路径、自然 selected 和实际控制执行。

## 全部 ID 的 handler 行为

| ID | 数量 | 原 handler 行为 |
| --- | ---: | --- |
| `00` | 1 | Selected 为零时上传固件并重置；queue 中零表示空，不能以普通 `RequestSound(00)` 提交此操作 |
| `01` | 1 | 设置 40 步 fade、初始间隔 3，并停 DAC music 行；后续 fade 间隔为 3 |
| `02` | 1 | StopPrimary |
| `03` | 1 | StopSecondary |
| `04` | 1 | 清 M68K driver、FM keyoff 和 PSG mute；已有 PCM 的独立生命周期不由该命令直接停止 |
| `05` | 1 | 设置 50 步 fade、初始间隔 32，并停 DAC music 行；后续 fade 间隔为 3 |
| `06..7F` | 122 | 普通 PCM descriptor 索引 0～121 |
| `80` | 1 | 路由到旧入口，但首 guard 对该 ID 直接返回 |
| `81..9F` | 31 | 音乐表；22 个不同 header，见下文 |
| `A0..AB` | 12 | primary FM／PSG 表 |
| `AC..EF` | 68 | secondary 首 guard 与正常 dispatcher 输入没有交集，均在初始化前返回 |
| `F0` | 1 | 独立 PCM descriptor 索引 122，不能当作普通 `7A` |
| `F1..FF` | 15 | 返回；`FF` 另为已选声音的空闲标记 |

总计：selected-reset 1、控制 5、PCM 123、音乐 ID 31、primary 12、handler-return 84。`handler-return` 只表示 handler 未初始化声部；根 queue／priority 消费可能已发生，不能把它概括为整个系统无副作用。

## secondary guard 与表存在性

正常 dispatcher 只将 `AC..EF` 送入 secondary handler。该 handler 的首条件却要求 D7 的低 byte 无符号小于 `AC` 才进入初始化，否则返回。D7 来自先清零再读取 selected byte；正常输入与 guard 的可进入集合为空。`80` 的旧入口有同样的范围不相交情形。

guard 后仍能找到连续的 26 个结构指针：`AC..BA` 为 15 个 F2-only 结构；`BB/BC/BE/C0/C2/C4` 与 primary `A0` 共用 header，`BD/BF/C1/C3/C5` 与 `A1` 共用 header。再往后的按索引读取会进入已知音乐 header／序列数据，不能据此认定有 68 个有效 secondary definition。

这些结构只有在假设绕过首 guard 时才涉及初始化、claim 或 keyoff。它们不是正常 AC～EF 分派已经执行的证据，也不能仅凭 F2-only 称为无副作用。原 ROM 的对齐绝对 call／jump、PC-relative call／jump、Bcc／BSR byte candidate 扫描只找到正常 dispatcher 到 handler 的分支和 handler 自身的 guard 分支；二者均与保存的 driver listing 一致。该有界检索没有发现直接绕过候选，但不能排除 computed、运行期或其他未分析入口。

## 音乐目录与自然流程

`81..96` 为 22 个不同 header，均含 6 条 DAC／FM 和 3 条 PSG 记录。`81` 的 9 条轨道均以 F2 结束；`97..9F` 与 `81` 共用 header。它们仍经过音乐初始化，不能作为整个 dispatcher 的 no-op。未取得足以把这些 ID 定为「只供诊断」的游戏调用证据。

非 stub 的不同音乐为 `82..96`，共 21 首。相对当前 11 曲目录，额外不同音乐为 `89/8B/8C/8E/90/91/92/93/94/96`。全部 22 header／198 轨和 12 primary／16 轨的有限或循环数据路径均完成静态展开，没有未知扩展、越界或共享 loop／return 栈失败。扩展的 note-code 观察与冻结 walker 的其余结果逐项相同。

已保存的 5,500 帧自然流程原版对照确认下列额外音乐，不是私有 sound-test 输入：

| ID | 首次 request 帧 | 首次 selected 帧 | 已证明范围 |
| --- | ---: | ---: | --- |
| `8E` | 835 | 836 | 自然 request／selected |
| `90` | 4,969 | 4,970 | 自然 request／selected |
| `92` | 375 | 377 | 自然 request／selected |
| `93` | 无直接 request 记录 | 557 | 自然 selected |
| `96` | 1,465 | 1,466 | 自然 request／selected；不等于已观察 E9 执行 |

静态音序另有 `92` 的 FM4 在 quanta 142 执行 replace-first 请求 `93`，以及 `93` 的 FM5 在 quanta 786 请求 `7D`。这提供了没有普通 request 调用仍可能 selected 的原音序来源，不把静态关联冒充唯一实时调用归因。

平台 fight 跳过原菜单的范围与原版完整流程目录是不同的问题。11 曲 fight 闭包由主审结合 MatchStart、Lifecycle 和已有音序队列边验证；本审计保留原菜单的真实路径，不据此扩大 11 曲既有实现的验收范围。

## 控制覆盖与参数边界

| 已查数据域 | E9 硬件 LFO／AM | FE FM3 offsets | ED／EE 直接 YM 写 | 初始 pitch control／F1／F4 |
| --- | --- | --- | --- | --- |
| 22 个不同音乐 header | 仅 `96`，共 3 处 | 无 | 均无；参数列表为空 | 初始低 7 位全为 0；无 F1／F4 |
| 12 个 primary header | 无 | 无 | 均无；参数列表为空 | row 清零后为 0；无 F1／F4 |
| 名义 secondary 结构 | F2-only 或上述 A0／A1 别名，无 E9 | 无 | 无 | 无；且正常 dispatcher 不进入这些结构 |

`96` 的 FM2／FM3 在 quanta 99、FM5 在 quanta 105 具有相同 E9 语义参数：AM enable 为归一 slot 1，global LFO enable、rate 1，AMS 3，FMS 5，不额外 OR-enable 左右 pan。这里证明静态轨道路径存在。现有自然窗口只证明音乐 selected，可能受菜单生命周期截断，未证明这三次 E9 实际读取或 chip 写入已经发生。

E9 的原 AM 基础读取还存在需后续实际 oracle 确认的细节，包括 MOVEA 后的条件码来源、track 共享字段的取值，以及增加 A0 却经 A1 读取的源行为。本次没有为此新增 CPU 捕获，也没有将静态参数解码表述为完整 E9 实现通过。

不能仅凭没有 FE 就排除直接 YM 写开启 FM3；本审计同时核对 ED／EE，在已查路径中两者均不存在。初始 pitch-control 低 7 位为 0、没有 F1／F4，而 F0／FC／FD 仅改变 modulation bit 7，因此在该已查数据语法域内未找到 pitch-envelope 选择路径。这些结论不延伸到任意 RAM 写、外部硬件写或未建立的游戏入口。

## PCM 覆盖

普通 `06..7F` 的 122 个 descriptor 均指向有界有效 payload，内容各异。`7F` 的 encoded length 为 1；不能仅凭短 payload 把整个仲裁／状态更新称为 no-op。已有有限媒体目录对该 ID 的处理不等于 dispatcher 表中不存在它。

F0 为独立的 6,670-byte DPCM4 payload，SHA-256 `c6409ebd28c594205970069fbc3233241cf15dc08ce8b847a5fa118b3e1a5382`；native rate 为 20，dual 模式、优先级高两位为 128、目标 B，不请求 duck。它与全部 122 个 ordinary descriptor 以及全部 25 个 music DAC descriptor 均没有 payload 内容重合。普通 `7A` 的 payload 为另一份 1,075-byte 数据，不能用它替代 F0。

内容独立性不是采样时基、解码、仲裁或混音完成证明。F0 的合法 Hiei dragon 请求路径由主审核对 engine 来源；本次独立确认 descriptor／payload 身份及与既有内容不重合。

## 复核与未知项

本次新增脚本只在 `.local/md-yuyuhakusho/audio-game-dispatch-original/`，运行不实例化模拟器、不修改 ROM／CPU、不生成生产资源：

```sh
python3 -B .local/md-yuyuhakusho/audio-game-dispatch-original/audit.py
python3 -B .local/md-yuyuhakusho/audio-game-dispatch-original/check.py \
  --audit .local/md-yuyuhakusho/audio-game-dispatch-original/audit.json \
  --out .local/md-yuyuhakusho/audio-game-dispatch-original/check-result.json
```

失败场景与缺失产物 red 先于新审计器；检查覆盖 256 ID 分类、guard 空交集、表别名、全部静态 token、自然证据、F0 内容独立性及语义／provenance 分离。静态路线复用固定 ROM 和 direct Ghidra 输出，不把 REA 包清单当作原逻辑证据。

仍未解决：全部音乐的合法游戏场景闭包、自然 `96` 是否实际执行 E9、computed／动态绕 guard 的完整排除、F0 独立合成／时基与混音验收，以及平台与原版菜单范围的最终交付选择。这些边界不能由表存在或既有 42 流未到达直接替代。
