# 从空闲状态启动的 PCM 命令与总线语义

命令层已恢复普通音效、音乐 DAC、双槽追加／替换、模式切换、私有 bus-held 暂停、DMA 等待及硬复位的 typed 语义。研究模型从 empty/idle 开始，只接收音频资源、已接受的命令和明确总线窗口，自行产生事件时刻。新增真实 address port 记录后，23 个验收场景的 519,256 个有序 address／data／总线／reset 事件逐项相等；1,274 个参考帧的 937,609 个立体声 PCM 帧和完整 DAC DSP 状态相同。每帧保留的 YM 地址 latch 也一致。场景含一个延长至自然结束的原流，不能把相同前缀重复计为独立输入种类。

可保存恢复的 Go 基线已在 `e59e22f0` 提交。其 12 组、837 帧、210,318 个原有序事件、615,994 个立体声 PCM 帧与状态相等，原生 race／Wasm 各 5 项顶层检查、28 个子场景及 vet 通过。每一帧都先 JSON 保存恢复再继续推进。Go 控制 v2 另通过 22 个验收场景、1,269 帧、325,741 个有序 data／control／reset 事件，原生 race／Wasm 各 9 项顶层检查、50 个子场景及 vet 通过；17 条非 reset 控制流的 802,191 个立体声 PCM 帧与完整 DSP 状态相等。它保留 raw reset 逆序，未用旧单调 Queue 强行验 reset PCM。自适应总线桥和真实address port接线继续在独立revision实现，旧基线来源、原测试和源码归档保持冻结。

Go v2 证据见 [validation-controls-v2.json](../../../games/yuyuhakusho/verification/audio-pcm-command-go/validation-controls-v2.json)。Go API 与基线验收见 [AUDIO_PCM_COMMAND_GO.md](AUDIO_PCM_COMMAND_GO.md)及 [validation-v1.json](../../../games/yuyuhakusho/verification/audio-pcm-command-go/validation-v1.json)。本报告说明额外原生语义与证据边界，不把研究 prototype 计为 Go 新 revision 已验收。

## 资源、协议和仲裁归属

普通音效使用 `380a8` 描述的 7 字节协议。音乐 DAC 使用 `37724` 的前 6 字节，pan 来自轨道，不读取普通第 7 字节。普通 single 的第 7 字节是 B6 pan，dual 则解释为目标 routing。资源目录按 encoded SHA-256 唯一保存 payload，命令设备只借用只读 Handle 并保存逻辑 ID。

普通优先级／目标选择由已有 `engine.SelectPCMCommand` 负责，新层接收已接受的目标和 `StartDual` 决策；后者来自原仲裁采样，不能在发布时重新读模式取代。音乐仲裁仍属于 score：当前 PCM 提供 `Blocked/Priority/PanRedirect` 临时视图，新设备不保存第二份 score 状态。

| 条目 | 原版事实 | 本层处理 |
| --- | --- | --- |
| 06～7E | 121 个普通有限音效描述 | 普通 lookup，按描述 native rate／flags／routing 发布 |
| F0 | 飞影黑龙实际请求；索引直接重映射为 122，描述在 `38478` | 独立合法条目，6670 字节，不能当 7A alias |
| 7F | flagsE0、rate20、长度1，属于内部静音项 | 保留独立 lookup；不加入公开有限 WAV 列表 |
| 音乐 DAC | 6 个实际 payload，描述与轨道 pan 分离 | 复用 score 描述身份和统一 payload owner |
| 02／03／04 | FM／PSG 行组或驱动清理，原实测没有立即结束本批 PCM | 不翻译为 PCM stop |
| 音乐 DAC track-end | 清音序活动位并退出本 driver tick | 当前样本继续自然结束，不附加 stop／enable／pan |

`1ffd` 的语义分开保存：普通启动为 `80`，音乐为 `1`；Z80 据此选择普通 pan 或音乐 pan。`1ffe` 为请求而非稳定播放速率：0 表示无请求，`1..7F` 进入 single，负值进入 dual 初始化。已经初始化后的 rate0 延迟测试，不能用来宣称发布 request0 能启动样本。

## 预取与未消费数据

single 启动真实读取头部，并预装 128 字节。长度1的 7F 仍产生128次 payload 区域 bank 读取，之后只解码该1字节并自然结束。其余127个原读取值没有进入解码器，独立设备只保留读取时刻、次数和总线效果，不调用资源外 `ByteAt`，也不填零或打包相邻 ROM。

dual 描述先处于 pending80，再转换为 active40，随后复制源描述、重新建立预取阶段。新描述的发布、源转交和解码启用有不同时间点；旧低半段 mixer 结果在转交期间仍须保留，不能因资源替换而提前覆盖。真正的流水线值属于 typed 状态，缓冲低字节地址等可派生量按阶段计算。

## 原子边界与外部总线

Go 使用固定音频语义 phase 与有界进度计数。静态时长分区用于计算一次语义操作在总线申请处的原子完成边界，例如 idle 的13／4／10T分区、rate延迟的13T网格。生产没有原 PC、固件、寄存器／RAM 影子、动态 opcode 或原未来时间表。

外部申请总线时，原 core 先让 Z80 执行至给定预算，允许当前原子操作过冲，再授予 68k。释放时直接将 Z80 周期重设为 `ceil(release/15)*15`；长期 held 状态中原 uint32 周期可能因视频帧结尾归一化而下溢，不能对旧值取 max。设备以执行 frontier 和明确 held 状态表达这件事，不保存一份原 wrapped 计数器。

每次 bank 读在其发生点调用 `phaseclock.ResolveBankAccess`，包括头部、真实样本、超前及 dummy 读取。DMA 先前推到区间末端，再计算45周期辅助等待及70／77周期主机延长，使用当前 reference frame 相位。当前连续 DMA 场景均为 `Remaining=0`；非零剩余涉及外部 scanline 预算、wait 置位和清除，须在独立桥接域证明，不能只记录 helper 返回值后继续整帧运行。

本组 `single-pause/dual-pause` 是明确私有 bus-held transport 窗口。游戏 Pause 的 producer 及最终持有时机仍由世界／IRQ 层决定。`37b90` 虽先申请总线，其后调用的 YM helper 会释放同一硬件 level；不能仅从该叶函数推断整帧 Z80 状态。自然暂停的长期 held 与恢复另有 `score-clock-natural/natural-windows.json.gz` 原证据，应按实际外部 producer 接线。

## 硬复位与原调用顺序

参考 `fm_reset` 先 `fm_update(requestCycle)`，再执行真正 YM reset 和 busy 清理。它保留 resampler／FIR、已推进的采样 frontier 和其他未由该函数清理的状态。硬件 reset 的 assert 与 release 各调用一次；release 后执行固件 boot 的 pan0、key-off6、enable0 和状态清理，随后回到 idle。它与 `sound_reset`／全音频冷初始化有不同范围。

原子过冲可以造成 raw cycle 逆序。在私有跨 tick 场景，single DAC store 先完成于 `7183080`，随后 reset callback 保留请求时刻 `7183006`，两者之间有芯片 tick `7183008`。将回调按 raw cycle 排序时，首个 PCM 差异位于第8帧第56字节；保留原执行顺序、不 clamp raw cycle，全部8,832个立体声 PCM 帧和完整 DAC DSP 状态相同。

最初20组排序后没有产生 PCM／状态差异，原因包括第二次 reset 和采样网格。初次文件名误标为 red 已纠正，原字节和订正记录保留；真实反例来自随后单独声明的新输入，没有改写原向量。见 [sort-hypothesis-correction.json](../../../games/yuyuhakusho/verification/audio-pcm-command/sort-hypothesis-correction.json)与 [dsp-sort-chip-tick-red.json](../../../games/yuyuhakusho/verification/audio-pcm-command/dsp-sort-chip-tick-red.json)。

已有 DAC-only Queue 的单调输入合同保持冻结；有序复位域由独立芯片 revision 明确提供，不能对普通写入全局放宽乱序检查。

## 地址写与 warm reset

命令设备到完整芯片的接线须保留实际 address port 调用。新观察器以 kind11 记录 port0／2的真实地址写，data／reset 的原回调顺序不变。旧20场景的全部既有事件字段、每帧PCM／DSP／状态均作为新记录的严格投影保持相同。

额外场景在首个single的地址2A于71280写入之后、data于71535写入之前，于71300申请硬reset；原子操作在71340授予总线，随后跨帧持有。原frame0／1的YM地址latch均为2A，只在data时刻补地址的重放会保留此前2B，两帧实际失败；使用完整地址流则相同。真实single路径地址到data间隔17T（255master），dual路径为11T（165master），不能统一补在data时刻。

父研究模型已覆盖23场景的原始地址、数据和总线时刻；Go控制v2仍只承诺其data／control／reset域。地址写可在已有语义part发出，chip的地址latch仍由chip唯一持有，不在PCM设备添加第二份影子。新地址字段、实际失败与保留latch证明见 `data-only-address-red.json`、`actual-address-verified.json`、`autonomous-address-state.json`。

## 描述混合的拒绝边界

本层的资源由逻辑 ID 标识。两个声明的极短私有替换会让原程序拼接不同描述的物理字段：

- single 已锁存旧 header 后发布另一描述，实际读到第三个 header 地址 `70000`，既不是旧 `90000`，也不是新 `7001C`。
- dual 已锁存旧 bank05 后覆盖同槽，转交字段为 `050057b6f81c`，混合旧 bank 与新起点／长度；两个输入描述分别是 `050056ca0a0a` 和 `060057b6f81c`。

原核和观察核在这两组的完整状态和 PCM 仍相同。它们证明无地址资源合同的边界，未证明正常世界 producer 可以产生这种过密提交。Go 以原子错误拒绝对应窗口，保持自身状态不变，不将相邻 ROM 打包、猜测第三份资源或虚构未消费数据。直接事实见 [opaque-native-evidence.json](../../../games/yuyuhakusho/verification/audio-pcm-command/opaque-native-evidence.json)。

## 可重复验证

原生采样只使用已固定 ROM／两份 core，所有新文件在 worktree 内；重捕获为单个 `nice` 进程。普通原控制与观察逐帧比较完整 Z80 状态、RAM、PCM、视频和世界 RAM，观察桥严格委托原 handler，结束校验 wrapper 完整及零溢出。

```sh
PYTHONDONTWRITEBYTECODE=1 TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
nice -n 15 python3 scripts/yuyuhakusho/pcm-command-capture.py \
  --out .local/md-yuyuhakusho/pcm-command-review \
  --cases idle-to-single idle-to-dual-a dual-append single-pause \
  legitimate-f0-remap internal-single-byte single-reset-crossing-chip-tick
```

研究模型自主产生时刻，原 capture 仅用作 expected：

```sh
PYTHONDONTWRITEBYTECODE=1 nice -n 15 \
python3 scripts/yuyuhakusho/pcm-command-prototype.py \
  --capture .local/md-yuyuhakusho/pcm-command-native-expanded-final \
  --out .local/md-yuyuhakusho/pcm-command-model-review.json
```

自主事件到 DSP 的同一输入检查由 `pcm-command-dsp-prototype.py` 执行，`--sort-by-cycle` 在跨 tick case 应实际失败。Go基线和后续Go控制revision分别保留自己的原 expected、先行red、源码归档与验收索引，不把Python研究通过替代正式Go／Wasm验收。

世界 producer、自然 IRQ／VDP 协作、完整 FM／PSG 混合、流式播放器接线和扬声器输出仍是各自验收范围。本组没有修改已提交的 `audio/pcmplayer`、`audio/pcm`、`audio/dsp`、公开音频接口、原有限 WAV 或其他游戏。
