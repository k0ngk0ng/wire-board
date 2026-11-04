# 世界精灵呈现

此包消费已解码的索引素材和世界绘制请求，不读取 ROM、不执行模拟器，也不生成硬件 DMA／SAT 缓冲。`Catalog` 为只读资源；`Actor` 为一次绘制请求；`TileSlots` 是全局当前图块映射。

```go
catalog, err := presentation.DecodeCatalog(assetJSON)
slots, err := presentation.DefaultTileSlots(catalog)
frame := presentation.BeginFrame(slots, initialSpriteCount)
var actor presentation.Actor
for _, request := range drawRequests {
    // 初始化、延迟等规则可能返回静默结果；只提交实际 Draw 请求。
    frame, actor, err = presentation.ComposeActor(frame, request.Actor, catalog)
    // 把 actor.Cache 写回该对象唯一的呈现缓存。
}
palette := presentation.CompactPalette(worldPaletteWords)
raster, err := presentation.RasterizeSprites(frame, catalog, palette, 320, 224, false)
image := raster.RGBA()
```

以上省略错误处理。初始精灵计数由世界提供；参考默认场景为 2，不能为了方便改成 0。`ComposeActor` 在裁切／禁用时保留输入，进入可见绘制后才查帧、检查缓存和应用赋值。`SkipTiles` 不为零时保留四缓存和全部旧槽。旧槽上下文缺失会明确报错，不补透明。

`Actor.OffsetX/OffsetY` 对应图形偏移 `+28/+2A`；不能传入投影过程的临时偏移 `+54/+56`。后者已经由世界的投影阶段处理。

每帧只保留短窗口的 `Parts`／`Transfers`；下一帧用 `BeginFrame(frame.Slots, count)` 重置它们。全局 `Slots` 和每对象四项 `Cache` 必须跨帧保留，回滚／重连时恢复当前映射即可，不需要完整历史或原 VRAM。对象初始化和按前缀清槽需要同步清理对应缓存；不能把所有存活对象都强制绘制。

`OrderedParts` 按原层组升序、组内提交顺序排列，保留原始组合写出的片段。原循环可能写出终止的第 81 项；手动 Canvas 绘制有序片段时，应在 `Index==0` 或 `Index>=80` 处停止整个列表，不能跳过后继续下一层。`RasterizeSprites` 已执行这个停止规则。

片段保留整体／局部翻转、中心缩放、word 回绕和颜色覆盖。光栅输出包含颜色、透明度、优先级及来源对象，便于与独立背景层组合；非零颜色索引即使为黑也保持不透明。它使用普通 H40 模式的 9-bit 屏幕坐标，不模拟逐扫描线精灵溢出、硬件 X=0 掩码或影亮模式。

`ApplyPalette` 只替换一个世界槽的颜色 6～15，保留前六色和其他槽。颜色输入应与当前呈现帧的实际上传时点一致；不能在淡入／淡出时把上一帧物理色盘当作已完成世界的色盘。帧图块赋值和颜色是独立状态。

`world-sprites.json.gz` 包含父角色、酎子对象、通用／附着特效、开战及结果图形的有界目录。公共 `27f618` 以 `PreloadedOnly` 标记，它的传输描述符不是可据未知 `TileBank` 提取素材的来源。默认预载槽来自已验证的场景 0，仅是初始化依赖，不替代其他场景、动态 HUD 或资源加载。

本包属于客户端呈现代码。素材文件须按角色／场景按需加载并遵循平台的不可变 CDN 发布流程，不得将图块载荷嵌入正式服务器包。完整验证和边界见 [WORLD_PRESENTATION.md](../../../docs/research/md-yuyuhakusho/WORLD_PRESENTATION.md)。
