# 关前与关间演出的分屏呈现

当前 Session 与规范化素材可以精确呈现项目已经固定的原版参考：FCEUX 2.6.6、`SDL.NewPPU=0`。真实 `RuntimeRenderer.load/draw` 已与 4 个 Intro、3 个 Interlude 参考帧逐像素比较，430,080 个 RGBA 像素全部一致。这里是孤立的渲染器验收，未把原版后态、截图或视频送入游戏／Session 入口。

所有坐标使用原生 256×240 画面的零基像素。先在该坐标系合成，再缩放到 PC／手机画布。

## 输入与默认值

渲染只读取同一帧的 `PresentedFrame`：`AppliedOAM` 对应导出的 `OAM`，当前已应用的 `Patterns`、`Palette`、`Background`、普通 `Scroll` 和 `Display` 控制。不要读取尚未提交的 `Sprites.OAM`、`Patterns.Desired`，也不要读取已经自增的逻辑对象 Y／`Scene.IntroScroll`。

正常 NMI 在 `$F8A7..$F8B6` 恢复 `$FE` 掩码，按 Busy 决定是否暂时空白；`$F926..$F945` 恢复 `$FD/$FC` 的普通滚动和 `$FF` 控制。嵌套 NMI 的 `$F8E2..$F905` 也恢复掩码、滚动和控制。因此中段参数不是永久开关。

每个新模拟帧开始时，生产者先清 `SplitActive`／`RasterActive`，当帧实际执行对应 Scene control 时才激活。保留数值可以，但没有 active 标记时不得沿用。重复绘制、全屏缩放和重连应重画同一快照，不能在第一次 draw 后自行清 active 标记。

## Intro：64 行开始使用新的背景水平滚动

原版 `$F968` 先等待 sprite-zero hit，再等待一段固定循环，随后写两次 `$2005`。第一写使用当帧发出的 `Display.SplitScroll`，第二写仍使用普通 `$FC`。

8 个关前布局的标记均为 `OAM0 = [61, 3, 0x23, 80]`。按原始 2-bit 图形与背景计算，首次相交点均为 `(83, 62)`。

- 第一次滚动写在读到 hit 后 128 个 CPU 周期；第二次在 135 个周期后。
- 第 63 行中途先改变 fine-X，新的 coarse-X 到第 64 行才生效。
- 第 63 行的非零背景只在 x=83/84。8 个布局均验证了 x=120..255、fine-X=0..7 的背景索引为零；写入后的区域透明，因此固定素材下的可见结果不受该行的轮询相位影响。

可直接执行的规则：

```text
0 <= y < 64：背景使用普通 Scroll.X / Scroll.Y / control
64 <= y < 240：背景使用 Display.SplitScroll / 普通 Scroll.Y / control
精灵坐标与滚动无关，仍按同帧 OAM 合成
```

这不是从单帧猜出的裁切。619 次只读原版调用的周期差一致；4 个完整生产渲染器输出帧也逐像素一致。Intro 只有实际执行 `$F968` 的序列发出 split；零滚动值同样可能是有效的 split。

## Interlude：用真实 sprite-zero hit 决定两个区域

原版 `$9FC6` 只在 Interlude sequence 2 且 Busy=0 时等待 hit，然后写：

```text
$2001 = $FE & 0xEF    // 清精灵显示 bit 4，保留背景等其他位
$2000 = $FF | 0x01    // 切换水平 nametable，保留其他控制位
```

标记的 X 为 5。前 5 关标记 OAM.Y 从 64 开始，后两关从 24 开始，最后一次使用 185；第 8 关不播放这段关间演出。7 个有效布局的所有标记 Y 均验证：`hit = (9, OAM.Y + 2)`。对象 Y 在写完寄存器后才递增，必须使用当帧已应用的 OAM。

通用 hit 算法仍应读取素材，而不是直接把 `Y+2` 当作独立于素材的规则：

1. 要求当前背景与精灵显示都启用。
2. 以 `OAM0.Y+1` 为顶部。按 PPUCTRL bit 5 选择 8／16 像素高度，并应用 attribute 的水平／垂直翻转。
3. 8×16 模式使用 tile 的低位选 CHR bank，`tile&0xFE` 选择上半片；下半片使用下一 tile。使用当前已应用的 CHR 页面。
4. 按 y、x 递增顺序寻找「精灵原始 2-bit 值非零」且「普通背景 opaque 非零」的第一个像素。priority bit 5 不影响 hit。
5. x=255 不产生 hit。x<8 时必须同时满足背景与精灵左侧 8 像素开关。不能用 RGB 是否相同来判断透明。

### 固定参考的准确区域规则

本项目原版采样采用 FCEUX 旧 PPU。其实现先完成背景行，再在整行末尾通过 `if (SpriteON) CopySprites(target)` 合成精灵。因此中途清 bit 4 会让 **hit 所在整行** 的精灵不出现；原版写控制位后，完整的新背景 nametable 从下一行生效。

```text
y < hit.y：普通背景、普通精灵 mask
y == hit.y：普通背景；精灵使用 RasterMask
y > hit.y：背景使用 RasterControl；精灵使用 RasterMask
```

背景仍使用原来的 `screenY + Scroll.Y` 和正常水平滚动。不能把下半区域重新从 y=0 开始采样。保留原 OAM 顺序、每行精灵上限和前后景优先级。

3 个完整参考帧的 hit 分别为 87／137／187，新背景分别从 88／138／188 行开始；每帧 61,440 个像素均一致。生产 TS 渲染器已按此规则通过，未保留「RasterActive 不支持」路径。

### 与真实硬件亚行时序的区别

真实 NES 的 `$2001` 写入会即时改变该行剩余像素。这里的状态轮询有 8／9 个 CPU 周期的读间隔；读到 hit 后，mask 写在 13／14 个周期后，control 写在 22／23 个周期后。确切的行内 X 因轮询相位变化，不能把一个样本 X 当成常数。

现有语义 Session 没有 6502 子扫描线相位，因此不能据此声明真实硬件的亚行像素完全一致。当前精确保证针对已固定的旧 PPU 参考；旧 PPU 本身不保留 hit 行中 mask 写入前的精灵片段。本项没有借助捕获像素补齐这些差异。若以后更换参考核心或要求硬件级亚行验收，应单独定义并恢复时钟／写入相位契约。

## 证据与复查

- [只读时序记录](evidence/campaign-scene-raster-original.json.gz)：44,443 条事件，包含 619 次 Intro split、122 次 Interlude raster；整个 10,000 帧 RAM 与既有 reserve30 路线一致。
- [几何与周期核验](evidence/campaign-scene-raster-verification.json)：8 个 Intro、7 个 Interlude 的素材几何及周期约束。
- [完整像素比较](evidence/campaign-scene-raster-pixels.json)：独立规范化合成器的 Interlude 全像素验证。
- [生产渲染器比较](evidence/campaign-scene-raster-runtime.json)：真实 TS 类、7 帧 430,080 像素，零差异，生产源码 SHA 与检查期间稳定性绑定。
- [REA 证据](evidence/campaign-scene-raster-rea.json.gz)：`$F889`、`$F926`、`$F968`；Interlude `$9FC6` 沿用此前 REA dossier。
- [参考实现来源](evidence/campaign-scene-raster-sources.json)：固定版本 PPU 源码、摘要与 Evidence ID。

```sh
.local/board-art-env/bin/python -B scripts/contra/campaign_scene_raster_analyze.py
.local/board-art-env/bin/python -B scripts/contra/campaign_scene_raster_pixels.py
node scripts/contra/campaign_scene_raster_runtime.mjs
```

这些脚本仅用于研究与渲染器核验。游戏入口继续从 `NewSession` 和规范化素材启动。
