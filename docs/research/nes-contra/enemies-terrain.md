# 敌人地形请求与图形队列

已将日版固定 bank 的 `$C3D1`、`$C656`、`$C6D7` 恢复为独立图形请求代码，位于 `games/contra/engine/graphics_queue.go`。它负责图块／属性的延迟写入、原版队列预算和调用者所需的定位结果，不负责敌人胜负或碰撞状态转移。

原版 ROM SHA-256 固定为 `6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe`。完整 [REA 证据](evidence/enemies-terrain-rea.json.gz)、入口汇编、调用记录及实现摘要保留在 [验证记录](evidence/enemies-terrain-verification.json)。静态切片只映射固定程序窗口；实际 bank 6/7 的图块读取另由原版执行及已恢复的首关数据验证。

## 原版契约

| 入口 | 已核实行为 |
| --- | --- |
| `$EB1B` → `$F9E4` | 敌人侧包装：保存图块码，X/Y 各减 12，映射 bank 6/7 后调用 `$C3D1`。任一坐标借位时不调用队列，仍以 C=0 返回 |
| `$C3D1` | 仅当当前 `$21 < $40` 时接受；接受后追加属性及 4×4 图块包，更新 `$21` 并返回 C=0；拒绝返回 C=1 |
| `$C656` | 将像素坐标与滚动组合，处理水平进位换屏及 240 像素垂直边界，返回图块／属性地址及半超图块模式 |
| `$C6D7` | 写入单字节属性包的头部 |

容量判断针对当前字节游标。`Used=63` 仍能接受一个请求，追加后可达到 101；`Used>=64` 才拒绝。不能把它替换成「追加后不得超过 64」。`$23` 不是这个函数的锁：受控记录中 `$23=$FF` 同样可接受请求，并保持原值。

`$C3D1` 不改 `$0600` 超图块索引、不改 `$0680` 碰撞窗口，也不修改 `$23`。它返回的 `$12/$13` 是该请求的起始 nametable 地址，供调用者后续的碰撞辅助函数使用。碰撞变更由敌人规则另行执行，不能在图块请求时提前清除。

## 队列表示与接入

`GraphicsPacket` 保存 `Kind`、固定 `Length` 及若干 `GraphicsSegment{Address,Values}`。原编码为：

```text
Kind, Length, SegmentCount, (AddressHigh, AddressLow, Values[Length])...
```

每包字节数为 `3 + SegmentCount × (2 + Length)`。Kind 1 每次写入后前进一个地址，Kind 2 前进 32 个地址。`GraphicsQueue.Append` 复制输入数据并累计 `Used`；`Writes` 展开为现有 `BackgroundWrite`，呈现边界使用 `Reset` 清空。

敌人请求接口：

```go
result, err := queue.RequestSupertile(x, y, code, GraphicsContext{
    Scroll:     world.Scroll,
    Supertiles: &scroll.Supertiles,
    Tileset:    tileset,
})
```

`JungleGraphicsTileset()` 从已有首关定义取得替换图块及属性；原版 `$C56E` 表指向 `$83B1/$86FC`，对应主超图块表的第 59 项起，共 49 项。它复制源定义，不另存一份可变地图。

`result.Accepted` 为真时，`result.Address` 是已定位的起始地址。拒绝时不读取 tileset、不修改队列，Address 无效。敌人包装在 X/Y 小于 12 时原版并未定位新地址；应由包装明确标记 `AddressValid=false`，不能将默认零地址当成有效碰撞位置。

## 属性拼接与时序

图块码 bit 7 表示跳过属性更新，低 7 位选择替换图块。图块包固定为 4 段、每段 4 个 tile，共 27 字节。属性更新有四种模式：

| 对齐情况 | 属性包 | 总请求字节数 |
| --- | --- | --- |
| 完整 32×32 对齐 | 1 段 × 1 字节 | 33 |
| 水平跨两个属性单元 | 1 段 × 2 字节 | 34 |
| 垂直跨两个属性单元 | 2 段 × 1 字节 | 36 |
| 同时跨四个象限 | 2 段 × 2 字节 | 38 |
| bit 7 跳过属性 | 无属性包 | 27 |

局部属性合并读取原 `$0600` 超图块索引对应的主属性表，不读取已经绘制到 PPU 的属性。因此连续重叠请求不能改成从上一条请求的画面结果继续混合。基准超图块和碰撞窗口保持由世界层唯一持有。

## 调用者复用的属性查找结果

原版红炮台在图块请求后会把共享 `$0F` 当成目标数组偏移：`$859D` 调用绘制，随后 `$85B7/$85B9` 消费它。这个副作用影响 AI，不能改成始终瞄准最近玩家。

实现使用具名返回字段 `AttributeLookupWritten/AttributeLookup` 保留该效果，没有建立通用 scratch RAM：

- 预算拒绝：`Written=false`，调用者保持之前的选择。
- 成功且跳过属性：`Written=true`，值为 `$80`。
- 成功并写属性：值为原替换属性表的高字节。首关为 `$86`，来自 `GraphicsTileset.SourceAttributePage`，不来自 Go 对象地址。

偏移 `$86` 在该红炮台调用点会读取玩家弹池别名，而不是普通玩家坐标；具体目标投影由射击状态机恢复。底层队列只返回原版已经产生的选择值。

## 验证与范围

全部生产实现之前先写失败场景及差分测试，保留 red／green。当前 4 个测试及 race 检查通过：

- 自然双人 M 枪盒路线记录 3 次请求，覆盖开合和销毁图块；增加观察钩子前后 700 帧 RAM 完全一致。
- 420 次受控函数输入覆盖四种属性模式、code bit 7、滚动与水平换屏、`Used=0/31/63/64/255`、`Mode=$FF`。252 次接受、168 次拒绝，队列追加字节、游标和地址全部一致。
- 另 420 次原版调用验证属性查找结果：180 次返回 `$86`、72 次返回 `$80`、168 次拒绝保持原值。
- 混合滚屏测试保留列包 33 字节、属性列包 24 字节，当前 57 字节时仍能接受 33 字节的敌人请求；到 90 字节后再拒绝。
- 包形状、数据所有权、拒绝原子性和 `BackgroundWrite` 投影通过针对性验证。

自然、受控及 ABI 录制均重复执行，完整 RAM 和调用记录逐字节相同。受控记录设置的是原版函数的输入，执行结果仍由原版代码产生；这些记录不是自然通关证据。首关替换数据已接入，其他关的替换表仍需随其场景恢复单独验证。

## 复查

直接复查留存的原版夹具：

```sh
source scripts/env.sh
go test -race ./games/contra/engine -run '^Test(OriginalGraphics|GraphicsQueue)' -count=1 -v
```

重建静态证据：

```sh
python3 -B scripts/contra/enemies_terrain_prepare.py
node scripts/contra/enemies_rea.mjs .local/contra-enemies-terrain/spec.json
```

重新采样时，给每次运行使用未出现过的标签，避免覆盖原始证据：

```sh
.local/board-art-env/bin/python -B scripts/contra/enemies_terrain_capture.py natural recheck-natural
.local/board-art-env/bin/python -B scripts/contra/enemies_terrain_capture.py controlled recheck-controlled
.local/board-art-env/bin/python -B scripts/contra/enemies_terrain_capture.py controlled recheck-abi abi
```

自然输入路线取自工作区已有的 `natural-machinegun-01` 记录；初态取自原版 `coop-play.fcs`，均校验源摘要。原 ROM、已发布资源和其他任务文件没有改动。清理仅覆盖本任务私有 Swift／Ghidra 缓存、FCEUX profile 和重复截图／中间 dump；保留两次调用及 RAM 记录以便复核重复性。
