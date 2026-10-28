# 调色输出、循环与雪地视觉控制

本项实现原版 `$CEE3`、`$CF6D`、`$CC02` 和 `$CE59` 的语义接口。关卡 Header 由 World 唯一持有；调色循环只更新对应的调色索引，雪地程序只修改唯一 `CHRAnimation.Pace`。

## 实现前失败场景

- 把 Load(16) 当作重建全部 32 色，错误读取未请求的精灵调色组。
- 把 RequestedColors 累加／取最大值，遗漏原版覆盖语义。
- 淡化时错误处理首个背景组、基地 Boss、精灵组、负值或超范围计时器。
- 仅在颜色真正变化时才报告 `$06` 的调色指针低字节写入。
- 提交时直接复制数组，忽略 PPU 调色地址镜像，导致闪屏颜色不生效。
- 忽略日版图形队列 `$47` 门槛，提前提交或错误清除待提交数量。
- 循环／暂停／闪屏的顺序变化，或用已经递减前的 FlashTimer 选择颜色。
- 基地 1 Boss 的 `$23C2` 四属性写包丢失、排序错误、错误套用一般图形请求的 64 字节门槛。
- 原版雪地阈值的屏内比较、一次只推进一项、终止标记或共享 Pace 被重复维护。
- 无效资源或队列空间不足时部分更新输出、Header 或共享状态。

先采原版完整函数前后状态，保留自然与受控输入的区别，再写 red 测试与实现。PPU 提交仍由 World 的显示边界调用。

## 接入接口

[`palette.go`](../../../games/contra/engine/palette.go) 提供：

```go
result, err := output.Load(count, sceneType, header, resources)
result, err = output.Step(PaletteCycleContext{
    GameRoutine: routine, Counter: frame, Level: level,
    SceneType: sceneType, Alternate: alternate,
    Header: &header, Resources: resources, Queue: &queue,
})
committed, err := output.Commit(queue.Used)
```

`PaletteOutput` 保存 `Desired`／`Applied` 各 32 色，以及 RequestedColors、FadeTimer、Cycle、PauseCycle、FlashTimer。`PaletteResult.GenerationBiasChanged/Value` 表示原版 `$CF09` 写入的最后一组调色指针低字节，即 `$D0E3 + paletteID×3` 的低字节；每次有效 Load 都报告写入，即使数值没有变化。

Load 只读 Header；Step 通过 World 唯一的 Header 指针更新 PaletteIndices[2]/[3]，与调色输出和追加的图形包共同提交。资源错误或队列域错误不会产生部分修改。Terrain 的 SetFade／Header／Load effects 仍按原顺序调用，不能用最终 after 状态替代这些调用。

### Load 与淡化

- 只支持原版游戏阶段实际使用的 16／32 色请求；新请求覆盖 RequestedColors，不累加、不取最大值。
- Load(16) 只更新前 16 色，不读取未请求的精灵调色组。已排队的其他 Desired 字节保持原样。
- 非 Boss 场景的第一个背景调色组不受淡化影响；Boss 场景包含该组。所有精灵组均不受这条背景淡化路径影响。
- FadeTimer 为负字节或大于等于 9 时，受影响颜色为 `$0F`。正值 1..8 使用减色量 `[0,0,16,16,32,32,48,48]`，下溢回到 `$0F`。
- 仅正值 FadeTimer 在完整 Load 后减一。零值及负字节保持不变。

`LDA $CF64,Y` 的合法 Y 为 1..8；真正数据位于 `$CF65..$CF6C`。`$CF64` 的 `$BB` 是前一条 BNE 的操作数字节，未把它当成零计时器的淡化量。

### 提交与闪屏

日版 `$CC02` 在 `graphicsUsed >= $47` 时不提交，也不清空 RequestedColors。这个门槛与美版注释中的 `$30` 不同。World 应在旧图形队列被清除之前，按照原版显示边界调用 Commit。

提交按原地址写入顺序处理调色镜像：`$3F10/$14/$18/$1C` 分别镜像 `$3F00/$04/$08/$0C`。闪屏在 Desired[16/20/24/28] 写入颜色并请求 32 色提交，后写的精灵通用色会覆盖实际背景通用色。因此 `Applied = Desired` 不等价于原版行为。

### Cycle、Flash 与基地 Boss 属性

`$CF6D` 先比较 RequestedColors 与 GameRoutine；未被门槛拦截时，只有 `frame&7==5` 且 PauseCycle 为零才推进循环。循环后可 Load(16)，之后再递减 FlashTimer；新 FlashTimer 为偶数时写闪屏颜色并请求 32 色。暂停循环不会单独阻止闪屏，而整个入口被未提交请求挡住时二者都不推进。

数据来源包括 `$D03D` 九项循环长度、`$D046` 八项基地 Boss 第二调色组、`$D04E` 闪屏色、`$D052` 普通第二调色组和 `$D056` 结尾第二调色组。Level 8 表示结尾调色分支，不是第九个关卡。新增所需色组由 [palette-additional-colors.json](evidence/palette-additional-colors.json) 绑定 ROM／SHA，已由关卡资源所有者合入运行数据。

基地 1 Boss 的循环还在共享 GraphicsQueue 后追加一个 9 字节语义包，写 `$23C2` 起的四个属性字节。它没有通用地形请求的 64 字节门槛；0、64、70、71、86 等预算均精确保留顺序，合法边界 183→192 也通过原版对照。

## 雪地唯一 Pace

[`palette_snow.go`](../../../games/contra/engine/palette_snow.go) 提供：

```go
changed, err := snow.Advance(level, screen, scroll, &chrAnimation)
```

`SnowControlState` 只保存 Visibility 和 Cursor。第 5 关的原版阈值为 `(0,0,1)、(2,0,0)、(3,128,1)、(17,0,0)`，随后为 `$FF` 终止标记。跨过阈值时一次只消费一项，将唯一 CHRAnimation.Pace 设置为 1（可见）或 3（不可见）；不重置其 Timer／Phase，也不另存一份 Pace。Visibility 供完整精灵组装的 SnowControl 使用。

## 验证与明确拒绝的输入域

| 范围 | 结果 |
| --- | --- |
| 自然原版 | 八关共 6,400 条完整函数记录，重复录制的 RAM 与输出字节一致 |
| 受控输入 | 137 条；覆盖 Load(16/32)、全部淡化分支、提交门槛／镜像、Boss 属性包、循环／闪屏／结尾和雪地阈值 |
| 精确匹配 | 6,534 条：Load 309、Cycle 2,107、Commit 2,060、Snow 2,058 |
| 域外拒绝 | 保留 3 条原版重叠队列记录，Go 原子拒绝，未伪称输出匹配 |
| 检查 | red 先于实现；合法域 green 和定向 race 通过 |

3 条拒绝记录是人为设置 used=246（两种 Boss）及基地 1 在 used=184 时追加 9 字节。原版图形区从 `$0700` 开始，而 Palette Desired 位于 `$07C0`；这些输入会让队列与调色甚至 CHR 工作字共用存储。独立语义对象不能靠补写 oracle after 或构造通用 RAM 镜像来支持这种无效队列。

因此 Palette.Step 要求已有队列不超过 192 字节；基地 1 追加后也不能超过 192。used=192 本身不重叠，基地 1 的最大可追加起点是 183。World 还应在持久化恢复和 Terrain 显式 Load 前验证相同所有权边界。这个域约束不改变正常 `$47` 提交延迟，也不把一般请求的 64 字节门槛错误套到 Palette.Step。

原版完整夹具为 [`original-palette.json.gz`](../../../games/contra/engine/testdata/original-palette.json.gz)，旁边 source 文件保存身份和范围。REA、red／green／race 和文件摘要见 [palette-verification.json](evidence/palette-verification.json)。本轮不实现 Canvas、音频或游戏注册。

## 重复执行

```sh
. scripts/env.sh
go test -race ./games/contra/engine -run 'Test(OriginalPaletteAndSnowPhases|PaletteFailureIsAtomic)' -count=1
```

并行任务仍有未实现的测试先行文件时，可只编译全部游戏实现和本组件测试，避免修改其他人的 red 用例：

```sh
python3 -B scripts/contra/palette_check.py --race
```

原版重录使用 `palette_prepare.py`、私有 `palette_rea.mjs` 和 `palette_capture.py --mode natural|controlled --label <新名称>`。`palette_export.py` 的 `--first-label`、`--natural-label`、`--controlled-label` 选择对应记录。所有文件留在工作区，用户 ROM 只读；已完成的显示提交仍由 World 统一调度。
