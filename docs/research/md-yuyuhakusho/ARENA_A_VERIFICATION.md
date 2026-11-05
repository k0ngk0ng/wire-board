# 场地 A 主回调与子对象验证

范围是原场地偏移 `0a` 的 `21126`、固定槽 `9e00` 的 `21258`，以及子对象调用的 `14d0` 精灵描述符提交函数。原生竞态和 Node/Wasm 均通过 4 项场地 A 测试。此处不声明公共 world 调度、平台选场、辅助图块呈现或完整场景像素已经通过；这些由根任务接入。

## 来源与可达边界

- ROM：`c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06`。
- 挂钩内核：`e87739d0804e62423fda890377fa46a8932fe1a6fa0132b1ad7f13b8b3af7306`。原内核只供研究对照，生产代码和 Go/Wasm 测试均不读取 ROM 或调用 MD 内核。
- [主、子完整静态](static/arena-a-full.txt)：`21126` 经 `2113a`、公共相机和 `21690` 返回；`21258` 完整到 `212d2 RTS`。
- [描述符消费者](static/arena-a-draw.txt)：`14d0` 完整到 `150c RTS`，无额外调用。
- [初始化、排序和音频入口](static/arena-a-consumers.txt)：`125a`、`129e`、`219ac`。音频后续源见既有 `audio-loop-sequencer.txt`，RAM IRQ 源见既有 `world-frame-irq.txt`。

raw 68000 不在当前 REA 输入支持范围。上述静态来自独立私有项目中的直接 Ghidra，不标为 REA 反编译结果。所有运行结论来自固定内核实际执行。

## 原 CPU 结果

| 参考 | 主回调 | 子回调 | 即时提交 | IRQ | 音频清除对 | 未归属字节 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 冻结私有边界 | 2,592 | 1,128 | 6,768 | 0 | 0 | 0 |
| 冻结单初态连续时钟 | 192 | 192 | 1,152 | 0 | 0 | 0 |
| 合法菜单自然对照 | 1,203 | 1,203 | 7,218 | 6 | 4 | 0 |

合法输入和唯一未挂钩控制来自菜单分工的 `arena-a-inputs.json`、`arena-a-reference-frames.json`。本 collector 没有另建原版控制。全部 3,602 帧 RAM、视频摘要和尺寸与控制一致。自然主、子回调第一次均在帧 2,398、阶段 0 执行；同一轮父对象创建子对象，子对象随即初始化并提交六条描述符。

同一 PC 在 Stage Select 预览也会执行：帧 1,961～2,130 的主对象为 `8380`，子对象仍为 `9e00`，但父引用为 `8380`。自然 manifest 的 `excluded_menu_contexts` 分别保留各 170 次预览上下文。ready-match 参考只声明 `8180` 主对象及父引用 `8180` 的子对象，不把预览调用混入独立对局验收。

自然帧 2,720、2,802、3,211 各在主回调中进入两次 IRQ。每组一次经过 `FFF000 → FFF09A`，其中实际进入 `219ac → 375a6`；另一次经 `FFF000 → FFF0A8` 返回。`37c56/37cac` 入口及紧随 CLR 的 PC 成对记录，共 4 对。回调自身无音频、随机或清除调用。

内核的执行 hook 会将 RAM PC 报为 `FFFFF000`。collector 同时注册 24 位和 32 位别名，并记录实际 IRQ 入口、RTE 前出口、6-byte 异常帧、音频清除前后值和消费者差异。最终 collector 对每个主、子调用的有序片段重放完整 64 KiB，逐段校验旧值，末态与实际 RAM 完全一致。完整 actual delta、首尾 RAM 摘要和归属集合均保留；未通过扩大允许地址范围消除 IRQ 差异。

## canonical 字段

所有偏移均为十六进制。

| 对象 | 原字段 | 唯一持久所有者 |
| --- | --- | --- |
| 主 | `+06.word` | `Header.Stage` |
| 主 | `+08.word` | `ScreenX` |
| 主 | `+0c.long` 曲线指针 | `ScreenY`、`Aux0E` 两个 word |
| 主 | `+10.word`、`+14.word` | `Body.VelocityX`、`Body.VerticalVelocity` 的高 word，低 word 保留 |
| 子 | `+18.word` 缓存 X、`+1a.word` 父引用 | `Aux18` 的高、低 word |
| 子 | `+1c.word` 缓存 Y | `GraphicsSet` 高 word，低 word 保留 |
| 子 | `+20.word` Y、`+22.word` size/link | `TilesBase` 高、低 word |
| 子 | `+24.word` tile | `Body.Attributes` |
| 子 | `+26.word` X | `Body.Layer`、`SkipGraphics` 两个 byte |
| 子 | `+28.word` 描述符终止符 | `GraphicOffsetX` |

主对象阶段 0 只写子对象 header 和 `Aux18` 低 word，不清空其余字段。子对象没有派生对象、删除或额外分配路径。各测试比较完整的 256-byte 验证视图，保留未写字节。

`F6DF` 是 VDP register 7 的底色 byte，命名为 `BackdropColor`。依据为既有 `arena-assets-reference.json.gz` 中 CPU RAM 的 `F6D0` 起寄存器缓存对 `80xx/81xx/…/87xx/…/8bxx`，以及同帧真实 VDP 寄存器。`F6E7` 是 register 11 的滚动模式。`ArenaSceneState` 仅作瞬时视图，行数据回到 Cells，反馈数据回到共享 Stream，不增设第二份持久 PanA/B。

主对象使用共享 `LinearRows[x][40:104]` 的 64 个 word；行库每个 X 从原 `24000+(x-2e0)*256` 起恢复 128 个 word。A 的写区间为 `[80:184]` 常量、`[184:248]` 曲线；B 为 `[16:176]` 半值和 `[176:240]` 近景值。其余行和间隙保留。`SourceB` 按原 word 双倍、算术右移、清低 bit、再加偏移的顺序计算，反馈按实际源地址读取共享行窗口。

## 六次提交及硬件边界

主对象创建的子对象大小为 `40`。子对象阶段 0 写 `+22=0ffc`、`+28=ffff`，然后推进阶段。正常更新缓存 `父+08+pan` 和 `cameraY-shake`。暂停位 7 或非零慢标志只跳过初始化与缓存更新，末尾的六次提交仍执行。

| 次序 | tile | Y 加数 | X 加数 |
| --- | --- | ---: | ---: |
| 0 | `05d0` | 144 | 224 |
| 1 | `05d0` | 176 | 224 |
| 2 | `05e0` | 144 | 256 |
| 3 | `05e0` | 176 | 256 |
| 4 | `05f0` | 144 | 496 |
| 5 | `05f0` | 176 | 496 |

每次先重写 `Y=-cachedY+Y加数`、tile 和 `X=cachedX+X加数`，再调用 `14d0`。`ArenaASprite{Y,SizeLink,Tile,X}` 是此刻四个 word 的值快照。返回后对象只保留最后一条描述符，不能在回调结束时取一次状态再复制六份。

`14d0` 读取硬件计数、输出游标和层链末端。正常 size byte `0f` 表示 4×4 个 tile。三个起始 tile 对应 VRAM `BA00..BFFF` 的 48 个独立图块，正好是额外资源 `297056→BA00`；A/B 地图不引用此块。辅助素材由素材分工提供，不能因 A/B 去重而丢弃。

| 入口计数低 byte | 原消费者实际接受 | 六次后计数 |
| ---: | --- | ---: |
| 2 | 全部六条 | 8 |
| 79 | 仅第一条 | 80 |
| 80、127 | 全部拒绝 | 原值 |
| 128 | 全部六条；比较按 signed byte | 134 |
| 255 | 全部六条；计数低 byte 回绕 | 5 |

生产规则始终发出六条语义提交。呈现层负责正常精灵预算、排序和最终 link，不把 SAT 缓冲、计数或游标加入 world 持久状态。非零阶段的任意 size/link word 按原值保留，私有边界另覆盖 200 组 dirty size/link 与消费者预算组合。

## 测试顺序与限制

先保存 [失败清单](../../../games/yuyuhakusho/engine/ARENA_A_FAILURES.md)，再冻结私有 CPU 参考、测试和针对 `StepArenaA`、`StepArenaAChild` 的真实缺实现失败，随后写实现。缺实现输出为 `testdata/arena-a-frozen-symbols-red.txt`；私有参考 producer 的精确源码另存 `arena-a-capture-private-frozen.py.txt`，SHA-256 为 `639a7036b335b6108394f32089bb72382882ef53a479c014346373515049ed50`。

时序说明：本分工先冻结私有参考并实现，随后才收到并补齐合法 natural 夹具。Natural 测试函数在实现前已固定，接入夹具时没有改断言；这不能称为「全部自然参考在实现前冻结」。`VideoFlags/ColumnMode → BackdropColor` 仅作机械命名更正，原测试全文和摘要谱系另有保存。切换共享反馈函数后仍通过相同冻结参考。

接口只接收已恢复的曲线和行窗口。缺表、非法相机样本偏移或反馈越出行窗口时，主对象和子对象均原子保留。非初始化路径若人为破坏 `+28=ffff`，原消费者会继续读取额外内存；当前窄接口原子报缺少有界描述符，不声称支持该非法内存组合。合法自然状态满足终止条件。私有矩阵不代表全部任意 64 KiB dirty RAM 组合均受支持。

## 辅助精灵的排序和素材审计

[窄审计 JSON](../../../games/yuyuhakusho/data/arena-a-aux-order-audit.json) 保留来源文件、来源 manifest、正文和 gzip 摘要，以及逐帧索引结果。[审计脚本](../../../scripts/yuyuhakusho/audit_arena_a_aux.py) 只读取已有夹具。连续两次输出逐 byte 一致，报告 SHA-256 为 `a7eb2b4253a358ea914efb2aa173afe0136e96d8d82e2a49211c5c92a366f401`。

- 20 个实际绘制 SAT 链的 120 条辅助精灵记录，与同一 CPU 帧的六次即时提交逐项一致，均占最终链尾，末 link 为 0。CPU 帧 `f` 对齐 `active_vram` 和视频 `f+1`；例如帧 2,700 的索引为 `30..35`，帧 2,720 为 `32..37`。
- 20 份帧末 VRAM 加 20 份实际绘制 VRAM 的 `BA00..BFFF`，共 40 份，均等于 48-tile 素材，解码摘要为 `43993f1d01e867cbd11750e7526ec6ce33e8d395f3445ff9b0643d47ebbb9343`。
- 既有 83 帧精灵参考中的普通非预载 transfer 描述符目标并集，与 `05d0..05ff` 无交集。完整不连续区间见 JSON 的 `slot_ranges_half_open`；其最小、最大槽为 `0050`、`03ef`。统计包含 cache 命中和 SkipTiles 情况下仍存在的描述符，不能称为实际写入次数。默认天空对象 `9c80/9d00` 明确排除，它们的描述符覆盖 `05f8..05ff`。

`14e4` 按 link byte `& fc` 选层，`129e..12be` 按层从小到大连接，层内保持提交顺序。当前稳定排序和即时 Count 索引可复用；raw 提交不经过人物中心、Depth、Offset、cache 或 transfer 过程。`14da/14de` 的预算条件是 `int8(uint8(Count)) >= 80`，拒绝时不添加条目，也不修改 Flags。索引 0 或越出正常 80 项范围会终止整条链，不应跳过后继续。

根任务选择独立瞬时 aux bank：辅助 Part 使用局部 tile `0/16/32`，保持同一层序、精灵预算、9-bit 坐标和 column-major tile 顺序，仅在光栅读取图块时选择素材所有者。此审计不修改共享呈现代码，也不将样本中的无覆写扩大为所有人物、场地和时间的证明；最终精灵／背景像素合成由呈现分工验证。

### 辅助精灵窄审计的预先失败条件

这项补充只读取已经保存的原 CPU、VDP 和精灵参考，不重新执行原版，也不修改共享呈现代码。审计遇到以下任一情况必须失败：来源正文或 gzip 摘要与来源 manifest 不符；素材不是 IDa／`21126`／`21258`／`297056` 的 48 个图块；样本重复、缺帧或 `video_next_frame != frame+1`；实际绘制时的 SAT 基址不一致；精灵链循环、索引越界、六条描述符缺失、次序或即时值不一致；六条不是最终链尾或末 link 非零；帧末／实际绘制 VRAM 的辅助块不等于固定素材；普通非预载 transfer 描述符目标并集与 `05d0..05ff` 相交。统计范围和默认天空对象的排除必须明确保留，不能推广成所有场地、人物和时间的无覆写证明。

## 复跑

从 worktree 根目录运行。研究重采需要已固定的 ROM、内核和共享菜单控制；Go/Wasm 测试只需要仓库内夹具。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/audit_arena_a_aux.py

PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_arena_a.py \
  --out .local/md-yuyuhakusho/arena-a-replay \
  --inputs docs/research/md-yuyuhakusho/arena-a-inputs.json \
  --expected games/yuyuhakusho/engine/testdata/arena-a-reference-frames.json

GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -json -run '^TestArenaA' ./games/yuyuhakusho/engine

GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -json -run '^TestArenaA' \
  -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine
```

原生竞态、Wasm 日志和固定摘要集中于 `engine/testdata/arena-a-validation.json`。最终研究重采保留于 `.local/md-yuyuhakusho/arena-a-final/`；最初冻结的私有测试夹具保持原内容。已清理本分工的早期重采目录、空失败日志、Ghidra 私有项目和 Java 缓存，保留最终取证、原始缺实现失败与可复跑源码。没有提交、上传或发布。
