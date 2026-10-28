# 八关地形滚屏与图形队列

新增 `TerrainScrollState.AdvanceQueued`，按日版 `$DB81` 恢复横向、瀑布、基地四阶段及基地 Boss 切换。它使用已恢复的 `LevelResources`，向同一 `GraphicsQueue` 追加原版分组的延后显示指令，不维护另一份世界位置或调色状态。

先列失败场景并写差分测试，初次因缺少新 API 为 red，再实现规则。原版夹具使用真实关卡地图；没有把自行生成的地图或原版 after 状态作为实现输入。

## 当前验证范围

| 项目 | 结果 |
| --- | --- |
| 控制器路径 | 4,999 次真实原版滚屏调用；初态及 Boss 研究设置有明确来源 |
| 关末边界 | 3,047 次受控滚动／基地边界调用，覆盖八关 alternate 或 Boss 切换 |
| 计数器边界 | 24 次原版调用，覆盖 auto A/B 接力、保留已有滚动量、Tank 加法回绕、竖向忽略 B/Tank，以及基地清屏高位条件 |
| 合计 | 8,070 次游标、世界、Header、128 个超图块、128 字节碰撞、完整图形队列及有序效果请求一致 |
| 按关加载 | 同一差分只加载对应 `stage-N.json`，不依赖全八关资源同时存在 |
| 原子失败 | 队列溢出、缺少关卡、已开始写入后缺前瞻地图、后段超图块非法，均不提交任何部分状态或效果 |
| 物理存储边界 | 图形区在 `$07C0` 前结束，共 192 字节；真实 33 字节列在 159→192 时通过，160→193 和已越界游标均原子拒绝 |
| 公共预算 | 滚屏与敌人 `RequestSupertile` 使用同一队列游标，保留先检查 `$40` 再追加的原版准入规则 |
| 显示边界 | 8,070 次原版调用内部 nametable／PPU palette 均保持不变；图形和颜色请求等待原版后续显示边界 |
| Palette／CHR 联合 | 8,070 次真实 `PaletteOutput.Load` 与 CHR preset 组合，Desired、RequestedColors、FadeTimer、GenerationBias 和 desired CHR 全部匹配；Applied 未提前变化 |

完整记录保存为 [控制器路径](evidence/terrain-scroll-natural.json.gz)、[关末边界](evidence/terrain-scroll-boundaries.json.gz) 和 [计数器边界](evidence/terrain-scroll-counters.json.gz)。它们保留 31 个标量、实际地图／碰撞／队列、原版 Header 指针组、PaletteBuffer、desired CHR 及效果顺序。

核心差分比较 Scroller 所有状态和效果；`terrain_scroll_visual_test.go` 进一步调用真实 Palette 实现与资源 preset，逐条比较 PaletteBuffer、desired CHR、FadeTimer、RequestedColors 和 GenerationBias。after 只用于验证，未回填实现。这里仍不替代外层 World 的整个连续显示／音频链验收。

## 唯一状态归属

| 所有者 | 状态 |
| --- | --- |
| `TerrainScrollState` | Axis、TransitionRemaining、TileAddress、AttributeAddress、MapOffset、TankScroll、Supertiles、MapVariant、可变 AlternateGraphicsScreen |
| `OutdoorWorld` | Screen、ScreenScroll、SceneType、ScrollType、AutoScroll、IndoorScroll、FrameScroll、BossScrollComplete、逻辑滚动位置 |
| 世界当前 `PaletteState` | 原 `$49..$57` 的碰撞阈值、循环索引和八个调色索引，通过 `Header` 指针原地提交 |
| `CombatFrame.ScreenCleared` | 原 `$6A`，通过指针在完整基地房间结束时清零 |
| `GraphicsQueue` | 原字节预算和待显示 packet，滚屏与敌人共用 |
| 外层效果处理 | Palette／CHR、音频事件、敌人读取游标与两名角色的强制跳跃增量 |

`AlternateGraphicsScreen` 不能每帧重新从静态关卡表读取。原版 `$CEC6` 将它改为 `$FF`，并更新同一份 15 字节 Header，避免重复触发；切换后的碰撞阈值须立即用于本次滚屏后续绘制。

## 横向、竖向与基地差异

横向按字节计算 `FrameScroll + TankScroll`，每 8 像素写 28 个图块的一列，在低 4 位为 3 时写 7 个属性。列 packet 为 Kind 2／Length 28，33 字节；属性列必须是一份 Kind 1／Length 1／7 segments，24 字节。列末交换地图窗口并加载 `Screen+2` 的前瞻地图。

瀑布在 240 像素处换屏，地图数据仍为 8 行超图块。图块行 packet 为 Kind 1／Length 32，37 字节；属性行是 Kind 1／Length 8，13 字节。只有偶数图块行更新碰撞，碰撞写入固定的下 64 字节区域，`MapOffset` 只选择源地图。`Axis&3==3` 时，属性高半来自当前窗口，低半来自另一窗口。读取游标减到负值后换源窗口、加载 `Screen+1`，并恢复 Axis=29／TileAddress=`$23A0`。

基地只在 `ScreenCleared` 高位已设置时推进；日版 `$DC95` 对 0 或正值直接返回，没有执行美版注释中的电网写图路径。每个透视阶段先加载原版地图，再用 32 个调用逐列追加图块和属性；计数器到零时改为 `$80`，下次调用才交换显示页并推进透视阶段。

第四阶段完成后才清屏标志、清屏内进度、增加 Screen、发出两个 ForceJump 增量并重置敌人读取游标。不是把四个透视阶段当成四个房间。

## Boss 地图提前切换

最后一个基地房间的透视阶段 3 开始加载时，`MapVariant` 先改为 `BaseBossMap`，据此选择 Boss 地图／超图块／属性。此时 `World.SceneType` 仍为基地；32 列装载完成且下一次交换页面后，场景才改为 `$80`。

因此 MapVariant 与 SceneType 必须独立。世界接敌人可变地形请求时，原 `$C56E` 的 replacement 集合仍按 SceneType 选择，而基本属性表按 MapVariant 选择；过渡期间可能使用混合来源，不能合并为一个开关。

第 7 关 alternate 没有新的 CHR preset，但仍更新 Header 和请求 PaletteLoad。它不会错误切换到 preset 0。

## 效果顺序与外层事务

`TerrainScrollResult.Effects` 使用具名请求：`chr-preset`、`palette-timer`、`palette-load`、`audio-reset`、`sound`。顺序来自实际原版调用，不由最终 after 状态反推：

- 横向 alternate：CHR preset（有定义时）→ Header 更新后的 PaletteLoad(32)。
- 竖向自动滚屏：写 PaletteTimer(16)；达到 alternate 时再选择 CHR → PaletteTimer(128) → PaletteLoad(32)。
- 基地 Boss：更新 Header／PaletteLoad(32) → AudioReset → CHR(8 或 9) → Sound(`$5E`) → PaletteTimer(12) → PaletteLoad(32)。

PaletteLoad 本身会递减正值 fade timer，必须交给真实 Palette 实现；Scroller 不预先假定请求后 timer 仍为 12。CHR desired 与实际显示页的提交也由世界显示边界处理。

`AdvanceQueued` 先复制 Scroller、World、碰撞、Header、清屏标志和队列。地图、图块、CHR／Palette资源或队列预算检查失败时返回空结果，所有输入保持原值。外层 World 应再将这些状态与 Palette／CHR／敌人效果放进同一 clone，效果应用全部成功后才提交整帧。

原字节游标虽可表示 255，但 graphics buffer 与 PaletteBuffer 的物理边界是 192。新 Scroller 在事务前后都检查这一边界，不把能够表示的字节值当成安全容量；原版正常样本不触发该保护。

## 接入与替换旧实现

新 API 位于 [terrain_scroll.go](../../../games/contra/engine/terrain_scroll.go) 与 [terrain_scroll_graphics.go](../../../games/contra/engine/terrain_scroll_graphics.go)。调用时传入当前唯一状态：

```go
result, err := scroll.AdvanceQueued(TerrainScrollContext{
    World: &frame.World, Terrain: &frame.Terrain,
    Header: &header, ScreenCleared: &frame.ScreenCleared,
    Queue: &graphics, Resources: resources,
})
```

成功后，外层按顺序应用 Effects；若 `ResetEnemyReadOffset` 为真，仅重置该读取游标；将 `ForceJump` 逐席位按 byte 增量应用，不能改成统一赋值 1。`GraphicsQueue` 留到下个显示边界，不在本调用中绘制或清空。

旧 `JungleScroll` 和旧文件没有修改。替换时将 `Column` 映射到 `Axis`，复制原有游标／窗口／TankScroll／AlternateGraphicsScreen，按场景资源指针确定 MapVariant；不从 SceneType 单独推导过渡期 Variant。

## 重复执行

研究进程、状态副本及缓存仅位于 `.local/contra-terrain-scroll` 与 `dist/cts-tmp`。新的 capture 标签避免覆盖原始运行：

```sh
python3 -B scripts/contra/terrain_scroll_prepare.py
node scripts/contra/static_rea.mjs .local/contra-terrain-scroll/spec.json
.local/board-art-env/bin/python -B scripts/contra/terrain_scroll_capture.py --scope natural --label <新标签>
.local/board-art-env/bin/python -B scripts/contra/terrain_scroll_capture.py --scope boundaries --label <新标签>
.local/board-art-env/bin/python -B scripts/contra/terrain_scroll_capture.py --scope counters --label <新标签>
source scripts/env.sh
go test ./games/contra/engine -run '^TestOriginalTerrainScroll|^TestTerrainScroll' -count=1 -v
```

关末控制样本会声明预置余命／屏障和滚动输入，用来让原版自身加载全部中间地图并执行边界；这些不是自然玩家经过所有关卡的证据。本结果不证明敌人、完整关卡、最终结算或浏览器联机已经通过。
