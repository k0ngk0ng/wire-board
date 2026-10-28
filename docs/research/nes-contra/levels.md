# 八关地图与场景资源恢复

目标是恢复日版原版八关的地图、超图块、属性、CHR 组合、基地 Boss 场景和关末替换资源，供独立 World／Scroller 接入。ROM SHA-256 固定为 `6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe`。

## 实现前失败场景

- 从第一关的首数据指针推断所有表长，截断基地相邻的透视表，或误读瀑布倒序指针。
- 把前瞻别名算成额外地图，或将基地四个透视阶段当成四个独立房间。
- 纵向关卡仍解压 7 行，丢失第 8 行；RLE 行复制读到未初始化数据。
- 混用地图 bank 4/5 与超图块／属性 bank 6/7 的 CPU 地址。
- 只修改 CHR，遗漏关末碰撞阈值和调色循环；把零 alternate 索引当作替换为第零组。
- 基地 Boss 继续使用普通地图指针、普通 CHR 或未切换的场景类型。
- 将 `$C848..$C857` 的内联分派表当作 6502 指令。
- 只用自己的解码器往返自证，没有对照原版实际加载输出。
- 将受控加载器输入、受控场景边界或受控关卡初始化称作自然经过或八关通关。

先核对 REA 加载路径，保留原版加载调用及输入覆盖，再导出规范化数据。完整原版通关、敌人和关卡结局不属于此数据恢复证明。

## 已交付数据和验证范围

标准化数据为 [levels-catalog.json.gz](evidence/levels-catalog.json.gz)。它只包含关卡定义与来源信息，运行时无需 ROM 或程序 bank。静态数据和原版运行分别验证：

| 项目 | 结果 |
| --- | --- |
| 普通地图表项 | 144 项，包含重复引用和基地透视阶段 |
| 基地 Boss 地图 | 2 项，分别用于第 2／4 关 |
| 原版解压执行 | 146 次真实 `$E09C` 调用，输出的 8,256 个超图块索引逐字节匹配；未写入的窗口部分保持原样 |
| 关末资源切换 | 8 次原版滚动边界，碰撞／调色 15 字节、CHR 选择及基地 Boss 指针切换全部匹配 |
| 超图块集合 | 8 套；基地 2／4 共用一套，两个基地 Boss 共用另一套 |
| 初始状态 | 八关原版初始化头部全部匹配；对照原版当前地图窗口和实际 CHR 页，保留别名及动态变化 |

完整原版记录在 [加载器证据](evidence/levels-loader-original.json.gz) 和 [切换证据](evidence/levels-transitions-original.json.gz)，[验证报告](evidence/levels-verification.json) 保存范围、摘要及 REA 汇编索引。

这里的加载器验收有明确控制输入：复制已经保存的受控关卡初态，在实际滚动入口一次性设置流式加载游标，并在原版 `$E09C` 首次调用时替换请求的地图索引。Boss 加载器单项验证还显式选择原版 `$DD45` 的六字节指针组。原程序实际读取、解压并写入地图窗口；测试没有用自己的解码结果回填 RAM。

关末切换验证则只把原版世界设置到已确认的边界，由原版自己选择 Boss 指针、CHR 和新碰撞／调色参数。它们是原版函数与状态边界验证，不能称为正常操作经过所有地图或自然通关。

## 八关表边界

| 关卡 | 普通场景 | 地图表 | 表项／不同指针 | 每项行数 | 超图块数 |
| --- | --- | --- | --- | --- | --- |
| 1 | 横向 | `$8001` | 14／13 | 7 | 108 |
| 2 | 基地透视 | `$8276` | 24／6 | 7 | 118 |
| 3 | 纵向瀑布 | `$853E` | 10／10 | 8 | 117 |
| 4 | 基地透视 | `$82A6` | 32／6 | 7 | 118 |
| 5 | 横向 | `$8719` | 22／16 | 7 | 109 |
| 6 | 横向 | `$89AF` | 14／13 | 7 | 110 |
| 7 | 横向 | `$8C0B` | 16／15 | 7 | 122 |
| 8 | 横向 | `$8ED1` | 12／11 | 7 | 124 |
| 基地 Boss | 独立战场 | `$90E3` | 2／2 | 7 | 128 |

地图指针与压缩数据在 PRG bank 4/5，超图块和属性在 bank 6/7。所有地址均绑定相应 bank，导出保存原 ROM 偏移、长度与 SHA-256。

第 2／4 关地图表相邻，并共享从 `$82E6` 开始的数据区。第 4 关自身最小数据指针为 `$83FA`，不能据此将表延长到那里。第 3 关的指针顺序总体倒序，最后一项指向 `$8552`，不是按第一个指针推断长度。提取器综合头部中的表位置、共享数据起点和原版调用输出确定边界。

`aliasOf` 表示同一表中相同源指针的先前索引，原索引始终保留。基地的重复地图用于四个透视阶段，不按去重后的六张图替换其 24／32 项顺序。纵向数据有 8×8 个 32 像素超图块，但原版垂直滚动在 240 像素处回绕；256 像素的数据高度与 240 像素的 nametable 周期是不同概念。

## 原版加载与切换路径

- bank 4/5 的 `$A816` 以 `stage × 32` 从 `$A87B` 复制 32 字节到 `$40..$5F`。八关都经过此路径，未按第一关重新拼装头部。
- `$E09C` 在 bank 4/5 查地图指针，根据 `$41` 解压 56 或 64 项；保留字面值、重复值和已有行复制三种命令。有效流均在单字节读取偏移范围内结束。
- `$C76E` 从 `$C78B` 选择关末 CHR 组，零表示保持当前选择；`$C778` 将 `$C793 + preset × 8` 的八页复制到 `$07F0..$07F7`。
- `$CEC6` 将 `$48` 设为 `$FF`，从 `$D05A + stage × 15` 更新 `$49..$57`，同时改变碰撞阈值、调色循环和八个调色索引。只更换图案页会漏掉这些规则资源变化。
- 基地在 `Screen==AltPos、PerspectivePhase==3` 时从 `$DD45` 复制 `(map=$90E3, tile=$B68A, attr=$BE8A)`。第 2／4 关选择 Boss 地图 0／1；之后 `$DD16` 设置场景 `$80`，并选择 CHR 组 8／9。

关末 CHR 选择表为 `[13,0,14,0,15,16,0,17]`。第 7 关仍更新碰撞／调色数据，但不重新选择一组 CHR；不能把这里的零解释为切换到第一关图案。

`$E09C` 的 Ghidra 结果保守地包含了 `$E10A` 之后的碰撞代码。这是末尾条件分支的静态边界问题；本轮有效地图流均在 `$E09C..$E109` 的解压路径终结，没有把后续代码作为地图格式的一部分。`$C848..$C857` 明确是 CHR 动画分派表，不是指令或 graphics queue 操作码。

## 可变地形图块来源

`$C56E` 保存九组「替换超图块／替换属性」指针，前八组按关卡选择，场景类型为负值时选择第九组基地 Boss 数据。它们都是对应基本超图块集合的后缀，导出通过 `updates.baseSupertile` 和 `updates.count` 保留替换代码到源 ID 的关系。

| 集合 | 替换起始 ID | 替换数量 | 替换属性表 |
| --- | --- | --- | --- |
| 1 | 59 | 49 | `$86FC` |
| 2／4 | 25 | 93 | `$8EE1` |
| 3 | 71 | 46 | `$96DF` |
| 5 | 81 | 28 | `$9E39` |
| 6 | 105 | 5 | `$A5E7` |
| 7 | 100 | 22 | `$AE2E` |
| 8 | 73 | 51 | `$B653` |
| 基地 Boss | 74 | 54 | `$BED4` |

调用方可见的属性查询高字节必须取 `updates.attributesSource.cpu >> 8`。`$C3F7..$C3FF` 把替换属性指针写入 `$0E/$0F`；后续虽然读取基本属性表 `$46` 来合并象限，也没有重写 `$0F`。例如第 5 关基本属性表高字节为 `$9D`，替换属性表为 `$9E`。若入口 `$0F` 非零而跳过属性查询，原值则保持，不应强制写成任一表的高字节。

## 动态 CHR 与提交时序

初始 preset 只是起始赋值。[levels-chr-animation.json](evidence/levels-chr-animation.json) 另提供 `$C83B` 的字段、九行原版序列、每关计时规则和第 8 关特殊分支。原版实际调用在 [普通阶段记录](evidence/levels-chr-original.json.gz) 和 [边界记录](evidence/levels-chr-cases-original.json.gz)。

| 范围 | 结果 |
| --- | --- |
| 八关普通阶段 | 每关 256 帧，共 2,048 次 `$C83B` 原版前后状态 |
| 受控边界 | 14 组、28 次原版调用；覆盖基地 Boss／瀑布禁用、计时回绕、过渡页、雪地 pace 进位和第 8 关特殊行 |
| 提交时序 | 2,040 对连续帧都满足 `applied == 前一帧 desired` |
| 实际 PPU 内容 | 32 份采样中的 256 个 1 KiB 窗口与对应 CHR 原字节匹配 |
| 重复采样 | 增加函数观察钩子前后，八关完整 RAM、mapper 写入与映射记录字节一致 |

普通路径先递减 `$5C`，当高位清零时返回；因此 reload 值 4 对应包含零值等待的五个 tick，不能直接当成四帧周期。Phase `$71` 递减，负值回到 3，再从相应序列选择图案页。第 6／7 关分派直接返回；原表仍保留，但不是启用的动画。

第 8 关还有第九行动画数据：当 Screen 为 12 且 `frameCounter & 7 == 0` 时使用 row 8，而不是第九个关卡。此路径保留已经递减的计时器。非零 pace 会先递增，但使用旧 pace 值作为计时 reload；pace 回绕和 `12−Screen` 都按原版字节运算。独立实现应读取保留的原版前后状态验证这些边界。

CHR 页存在内容别名，例如页 10／14／16／18 字节相同。捕获器根据真实 mapper 写入确认页编号，再用 PPU 内容校验；不从相同图案内容反推唯一页号。当前 `desired` 与实际显示页相差一个原版提交边界，前端应沿 World 的呈现时间轴使用已经提交的映射。

## 数据 schema 与重复执行

```text
stages[8]: header, sceneType, scrollType, maps, tileSetId,
           initialPaletteState, chrPreset, alternate, optional baseBoss
maps: mapRows, entryCount, pointerTable, entries[index, aliasOf, encoded, supertiles, source]
tileSets: id, supertiles[tiles[16], attribute], tilesSource, attributesSource, updates
extraMapSets["base-boss"]: 两个 Boss 地图
chrPresets: id, pages[8], source
paletteColors: id, colors[3], source
```

基础关卡定义与动态 CHR descriptor 分开保存，便于逻辑加载与呈现提交各自维护。`paletteColors` 覆盖这些头部与关末资源实际引用的调色索引；不代表所有标题、结局或敌人临时闪光已实现。

```sh
python3 -B scripts/contra/levels_prepare.py
node scripts/contra/levels_rea.mjs .local/contra-levels/fixed-spec.json
node scripts/contra/levels_rea.mjs .local/contra-levels/header-spec.json
node scripts/contra/levels_rea.mjs .local/contra-levels/animation-spec.json
.local/board-art-env/bin/python -B scripts/contra/levels_capture.py --label v1
.local/board-art-env/bin/python -B scripts/contra/levels_capture.py --mode transitions --label v1
python3 -B scripts/contra/levels_export.py
.local/board-art-env/bin/python -B scripts/contra/levels_capture_chr.py v1
.local/board-art-env/bin/python -B scripts/contra/levels_capture_chr.py v2
.local/board-art-env/bin/python -B scripts/contra/levels_capture_chr_cases.py v1
python3 -B scripts/contra/levels_export_chr.py
```

这些命令需要原 ROM、已有 Ghidra／JDK／FCEUX 和八关初态。捕获器拒绝覆盖同名运行。重复时使用新的 label；关卡导出通过 `--loader <文件>`／`--transitions <文件>` 选择记录，CHR 导出通过 `--first-label`／`--repeat-label`／`--case-label` 选择对应运行。不删除仍在使用的初态或其他任务产物。当前任务只生成本地数据与证据，没有修改 World／Scroller／Player 或发布资源。
