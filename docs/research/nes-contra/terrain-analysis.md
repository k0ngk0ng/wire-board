# 日版第一关地图与碰撞数据

输入仍为固定 SHA-256 的日版原版 ROM。本轮从实际 ROM 数据恢复了第一关的 13 个不同屏幕、1 个前瞻别名和 108 个超图块，并用原版自然执行验证首屏及流式加载。其他七关没有按首关规则生成替代数据。

可直接审查的数据和原版夹具：

- [第一关完整地图导出](evidence/static-level1-terrain.json)：头部、原 ROM 范围、压缩流及边界、超图块、每屏图块／属性／碰撞。
- [原版碰撞与加载夹具](evidence/static-terrain-oracle.json.gz)：3,234 次实际调用、13 份不同的碰撞窗口、1 次后续屏幕解码、输入脚本及来源摘要。
- [验证结果](evidence/static-terrain-verification.json)：逐项匹配范围、两个录制运行的一致性及输出摘要。
- [地形 REA 证据](evidence/static-terrain-rea.json.gz)、[关卡头部 REA 证据](evidence/static-header-rea.json.gz)：完整分析记录；地址汇编同样进入 [统一索引](evidence/static-index.json)。

## 数据位置与编码

| 内容 | 日版位置 | 解释 |
| --- | --- | --- |
| 第一关头部 | PRG 偏移 `$A87B`，bank 5 映射 CPU `$A87B` | 32 字节，加载到 RAM `$40..$5F`；入口为 bank 4/5 的 `$A816` |
| 屏幕指针表 | bank 4/5，CPU `$8001` | 14 个 16 位指针；0–12 为 13 个不同屏幕，13 回指 0 |
| 超图块定义 | bank 6/7，CPU `$8001` | 108 个，每个 16 字节，表示 4×4 个 8×8 图块 |
| 超图块属性 | bank 6/7，CPU `$86C1` | 每个超图块一字节，对应四个 16×16 象限的调色属性 |
| 当前／下屏超图块 | RAM `$0600/$0640` | 每屏 56 个有效条目，即 8×7 个 32×32 超图块 |
| 两个碰撞窗口 | RAM `$0680/$06C0` | 每窗 64 字节，每字节四个 2 位碰撞格；当前关卡有效地图为 16×14 格 |

头部的 `$48=11` 指定替换图形的屏幕位置，`$58=11` 指定 Boss 自动滚屏触发所在屏幕。`$E067` 还检查屏内滚动阈值；横向阈值为 `$A0`，自动滚动计时为 `$60`。这些字段不应被客户端任意改写。各屏完整游戏经过、最终 Boss 和替换图形仍需动态验收。

屏幕解压入口为 `$E09C`，程序以 `$14/$15` 切换到 PRG bank 4/5。压缩命令有三类：字节 `< $80` 直接输出超图块索引；`$80..$EF` 的低 7 位为后一个字节的重复数；`$F0..$FF` 从本次输出的指定行复制 8 个条目。首关输出为 56 个条目，另一个窗口从 `$40` 偏移开始。导出保留每条命令的原 CPU 地址、编码长度、SHA-256 和复制来源行。

最后一个指针是供前瞻使用的重复引用。导出仍保留它，并明确 `alias_of: 0`；没有将它算作第 14 个独立地图。13 个不同屏幕全部能从原字节有界解码，后续的实际经过范围与静态解码范围分别报告。

## 碰撞从图块生成

`$DF30` 使用每个 16×16 象限左上角的图块编号生成碰撞码，首关阈值来自头部 `$49/$4A/$4B = $06/$FB/$FF`：

| 图块编号 | 存储 2 位值 | 返回碰撞码 |
| --- | --- | --- |
| `$00` | 0 | `$00`，空 |
| `$01..$05` | 1 | `$01`，地板 |
| `$06..$FA` | 0 | `$00`，空 |
| `$FB..$FE` | 2 | `$02`，水 |
| `$FF` | 3 | `$80`，实体 |

每字节从 bit 7–6 到 bit 1–0 依次存放左至右的四格；每行 4 字节。初始地图数据并不包含炸桥、Boss 门体等全部可变地形行为，后续世界状态需按原版事件维护这些变化。

## `$DFEC` 查询契约

调用输入为寄存器 A 中的屏幕 X 和寄存器 Y 中的屏幕 Y；读取 `$FD` 水平滚动、`$FC` 垂直滚动和 `$FF` PPU 控制值。X 加水平滚动的进位会翻转 nametable 的低位，选择两个 64 字节碰撞窗口之一。Y 按原版 `$F0` 边界的字节运算修正，再选择行和字节内的 2 位槽。

原始输入 Y ≥ `$E0` 时返回空，即使计算出的源字节中有非零碰撞。结果 A 为 `$00/$01/$02/$80`，C 表示地板，N 表示实体。夹具保留以下真实观测字段，Go 测试应直接读取这些结果，而不是在测试中复写实现：

```text
collision_maps: [{ id, bytes: <128 字节 hex> }]
queries: [{ frame, x, y, scrollX, scrollY, ppuControl, map,
            code, carry, negative, sourceIndex, sourceByte, column }]
```

`negative` 为 0 或 128，`carry` 为 0 或 1；`sourceIndex` 是真实 `$13`，`column` 是真实 `$12`，`sourceByte` 是返回时读取的 `$0680 + sourceIndex`。本次自然操作覆盖空、地板和水；实体 `$80` 只有静态规则和数据依据，尚无自然调用覆盖，不能据此声称实体碰撞已动态验收。

玩家的室外脚下查询另经过 `$D8D9`，实际采样点是 `($0334+x, $031A+x+$10)`；Y 加 16 溢出时直接返回空。玩家 Y 是角色锚点，不能直接当脚底坐标。

## 动态验证范围

起点为原版双人正常入场后的 `coop-play.fcs`。独立录制目录为 `.local/contra-static/oracle`，只读复制初态并校验摘要，不与其他录制共享可写 FCEUX 配置。控制输入是双人同时右移和射击，按固定帧给出跳跃短按；观察 hooks 从未改写游戏内存。

对原版 `coop-play-entry` 的第 200 帧，验证了第 0 屏的 56 个超图块索引、896 个背景图块、56 个属性字节和 64 个碰撞字节全部一致。第 1 屏的 56 个索引已全部解压，但背景仍在流式写入，因此只比较已写入的左侧 16 列，其余 PPU 和碰撞字节按原观测保存在夹具中。

600 帧自然操作录制中，3,234 次碰撞调用的源索引、槽位、源字节、码和标志均与静态恢复算法匹配；后续第 2 屏的实际解压结果也与原 ROM 解码一致。相同初态和输入在另一个运行名称下再次执行，RAM、碰撞调用、窗口和解压记录逐字节相同。

尚未覆盖的是第一关全部屏幕的自然经过、实体碰撞、动态地形破坏、Boss 场景替换图形，以及其他七关。导出文件明确这些边界。

## 重复执行

先通过公共原版录制脚本生成 `coop-play.fcs` 与入场记录，随后执行：

```sh
python3 -B scripts/contra/static_prepare.py
node scripts/contra/static_rea.mjs .local/contra-static/terrain-spec.json
node scripts/contra/static_rea.mjs .local/contra-static/header-spec.json
.local/board-art-env/bin/python -B scripts/contra/static_capture_terrain.py --name terrain-walk-jump-01
.local/board-art-env/bin/python -B scripts/contra/static_capture_terrain.py --name terrain-walk-jump-02
python3 -B scripts/contra/static_extract_level1.py
python3 -B scripts/contra/static_export.py --runtime-run .local/contra/runs/boot-solo-r2
```

录制器拒绝覆盖已存在的运行目录。需要再次重复时给两次录制使用新的 `--name`，并通过提取脚本的 `--primary <第一个名称> --repeat <第二个名称>` 选择对应证据；不要删除仍在使用的原版初态或其他任务产物。
