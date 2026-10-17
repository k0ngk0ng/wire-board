# 热血篮球相机与世界投影

相机、人物拥挤标志和完整对象投影已恢复为独立 Go 组件。验收对照原 6,000 帧自然比赛记录；没有修改 RAM、注入后续状态或自行设计平滑跟随。实现未接入 `WorldActionState`，也未修改公共前端。

## 状态与调用顺序

`StepOriginalCamera(CameraState, CameraContext)` 对应固定 `$D66C`，位于每次常规世界更新的最前面。水平目标来自 `$03A2` 指定对象的世界 X，不直接改为持球者。垂直目标优先使用 `$053F` 持球对象的**旧投影**，球自由时改用 `$03A2` 对象的旧投影。

| Go 字段 | 原地址 | 含义 |
| --- | --- | --- |
| `CameraState.DisplayedX` | `$0397..398` | 已显示水平位置，仅作为本次相机输入。 |
| `CameraState.X` | `$039B..39C` | 下次显示和对象投影使用的水平位置。 |
| `CameraState.Y` | `$039D..39E` | 原两页纵向滚动编码，保留页间空隙。 |
| `CameraState.TileColumn` | `$03A1` | 背景瓦片列索引。 |
| `CameraState.Shake` | `$06D5` | 震动计数，先递减再按奇偶加减 2。 |
| `CameraContext.TrackedObject` | `$03A2` | 水平跟随对象及自由球时的垂直跟随对象。 |
| `CameraContext.Projected` | `$03E7/$03F4`、`$0401/$040E` | 上一阶段投影缓存，不能提前用新坐标覆盖。 |

水平跟随有 `[112,144]` 像素容差区，单次最多移动 8 像素，当前活动分支限制在 6 到 255。原代码另有未从当前入口到达的 40／231 限制分支；没有把它们错误加入当前路径。

`CameraStep.StreamColumn` 表示原代码会调用 bank 1 的瓦片列处理。当前 Go 组件返回该事件和列号，不模拟 PPU 上传缓冲。Canvas 可绘制完整场地，但不能因此把 `DisplayedX` 与 `X` 合并；原显示提交发生在不同边界。

`StepOriginalProjection(ProjectionState, ProjectionContext)` 对应 `$D348..D352`，按 0 到 12 依次执行 `$CD3C`。`ProjectOriginalObject` 是单个对象接口。`ProjectionContext.LayoutID` 来自 **`$0588` 画面布局**，不是物理和 AI 常用的 `$058B` 地形编号。

投影使用整数世界 X、地面深度和腾空高度。隐藏姿势 `pose & 0x7F == 0x7F` 保留两轴旧缓存；横向落在 `[-48,303]` 之外时只更新 X，保留 Y。可绘制 Y 区间为 `[-16,239]`。除布局 2 外，绘制列表按地面深度稳定插入；布局 2 按 `DrawCount` 追加。列表第一次遇到 `0xFF` 就停止移位，不能把尾部所有缓存槽位一并后移。

绘制列表的重置属于调用方原来的阶段边界。投影组件不擅自清空列表，也不根据未变化缓存猜测某对象当前可见。

## `$06D6` 的用途

`OriginalCrowdingFlags` 恢复 bank 3 的 `$BF42`。它仅设置或清除 bit 7，保留低 7 位。计算条件是某名人物与两名索引更大的可见人物处在接近的纵向精灵行中；通过 X 高字节判断是否在屏幕页内，但**不计算横向间距**。边界包含字节环绕和下界排除，不能替换成通用矩形重叠检测。

对全部 8 个 PRG bank 的 `$06D6` 绝对地址操作数做检索，并核对命中指令，得到：

| 位置 | 行为 |
| --- | --- |
| bank 3 `$BF97/BF9C`、`$BFAA/BFAF` | 拥挤函数置／清 bit 7。 |
| bank 4 `$9288/928D` | 特定球与篮筐交互条件下清 bit 7。 |
| 固定 `$D442`、`$D56A` | 阶段重置时整字节清零。 |
| 固定 `$F268` | 显示中断检查 bit 7，在 `$E4 == 0` 时调用 `$F002`。 |

REA 与指令核对确认 `$F002` 只把 `$0200..02FF` 中的 64 个 OAM 精灵记录反向排列；它不移动角色、不分配 AI，也不修改得分。其作用与 NES 同扫描线精灵显示顺序有关。以上检索边界是绝对地址引用，不能声称排除了所有间接寻址可能。

因此 bit 7 应保留为世界快照关联的**显示调度状态**，供原记录核对和显示使用；篮筐阶段清位时应同步。当前没有证据支持把它加入物理或 AI 决策参数。低 7 位尚无非零语义证据，接口保留原值。浏览器不需要人为制造 NES 精灵数量限制；如要宣称逐像素显示一致，则还须单独验证原 OAM 顺序与闪烁行为。

## 验收证据

原版观察脚本 `scripts/basketball/capture_camera.py` 复用 `hoop-control-cpu-long` 的全部输入和初始存档。采样后的 6,000 帧完整 RAM 与原记录逐字节相同，最后截图逐字节相同。没有修改游戏物理或 AI 样本。

- 相机：1,997 次完整阶段，所有接口拥有的字段一致。
- 拥挤：1,997 次完整阶段，标志结果一致。
- 投影：1,998 次完整 13 对象阶段，共 25,974 次对象投影；坐标缓存、列表和计数一致。多出的一次来自首次比赛更新跳过相机前置阶段。
- 原生竞态与 Wasm 使用同一阶段夹具验证；日志位于 `.local/basketball/rea/camera/`。

原阶段中的异步中断还可能更新声音、显示缓冲和时钟字段。这些变化不是相机算法的输出，当前相机对照没有把它们当作自身状态写入。自然记录只覆盖画面布局 1、跟随对象 6，未出现震动；其他布局、震动和边界来自静态指令支持的针对性用例，不能称为自然运行覆盖。

复现：

```sh
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/capture_camera.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/extract_camera.py
go test ./games/basketball/engine -run 'TestOriginal(CompleteCamera|CameraBoundaries|Crowding|CompleteProjection|ProjectionVisibility)' -v
```

失败场景先记录在 `games/basketball/CAMERA_VERIFICATION.md`，测试先于实现编写，失败记录为 `test-before.log`。原始 RAM、调用次序、源 SHA 和阶段摘要保存在 `.local/nes-basket/runs/camera-cpu-long/` 与 `.local/basketball/rea/camera/capture.json`；压缩后的语义夹具及摘要位于 `games/basketball/engine/testdata/*-stages*`。

REA 主证据：`ev_7d4bb1bab4e26ad2dfb292ff555c68dac98d509b93da9ad6632b1fda0efd131c`。绘制排序和 OAM 反向排列证据：`ev_10ba6dfb306da34bbed8036d16307088dea75d4d4640237d4668e7360a833b46`。完整批次、函数报告和快照均保存在 `.local/basketball/rea/camera/`。
