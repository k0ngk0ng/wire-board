# 其余七场地的紧凑背景素材

本批恢复原场地偏移 `2/4/6/8/a/c/e` 的完整 A／B indexed 背景和场地 `a` 的 48 个子对象图块。七场各 20 个自然样本共比较 9,535,530 个未被精灵遮挡的原视频像素，差异为 0。素材来源、完整原版基线、负向验证和重复导出结果见 [验收索引](../../../games/yuyuhakusho/data/additional-arena-assets-validation.json)。这些结果验证素材和指定显示样本，不代替独立规则、整局玩法或最终精灵合成验收。

## 素材与原版来源

场地偏移是原 `A060.word` 的偶数值。原菜单名称及合法固定输入由 [场地菜单取证](ARENA_MENU_CONTROLS.md) 单独确认；本批素材按偏移绑定，不根据名称猜资源。

| 偏移 | callback／清单 | 去重图块数 | gzip bytes | 原视频比较像素 | 差异 |
| --- | --- | ---: | ---: | ---: | ---: |
| `2` | `20ae0`／`2175c` | 480 | 12,093 | 1,373,487 | 0 |
| `4` | `20cb2`／`21798` | 311 | 8,270 | 1,378,156 | 0 |
| `6` | `20e28`／`217cc` | 462 | 10,472 | 1,366,902 | 0 |
| `8` | `20fb4`／`21800` | 478 | 12,802 | 1,369,326 | 0 |
| `a` | `21126`／`21834` | 455 | 12,102 | 1,301,022 | 0 |
| `c` | `212f8`／`21870` | 420 | 11,039 | 1,373,304 | 0 |
| `e` | `21412`／`218a4` | 463 | 10,414 | 1,373,333 | 0 |

七个 `arena-assets-<id>.json.gz` 均保持 schema1：局部去重的 4bpp 图块、完整两张 64×32 平面、每单元的色盘／翻转／优先级，以及各场初始 16 色。运行包不携带 ROM、RAM、VRAM 快照、原视频、CPU、VM 或 MD 内核。初始色盘不是整局固定色盘；当前色盘、逐行滚动、垂直偏移、可见行及窗口由独立世界提供。

离线提取器解析固定 ROM 的八表注册和当前场地清单，使用原 `988/9ba` 格式解码源数据。独立验证器采用另一种 token 分组实现，逐源核对压缩摘要、解压长度、解压摘要及 token count。每个源还与加载后的实际 VRAM 逐 byte 相等，包括公共 `27db80→0000`、`27dcb4→1800`、ID2 的 `28bde8→a000` 和 IDa 的 `297056→ba00`。

ID2 两张地图各写满 `0x1000` bytes。其余六场从 `c100/e100` 各写入 `0xe00` bytes，因此完整地图的前后各 `0x100` bytes 不在场地清单的上传范围内。原版 20 个样本确认这些位置全部为 `0000`；素材保留实际加载值，manifest 分别记录范围、摘要和观察值，没有推定补零。每场全部 4,096 个单元及引用的 32-byte 图块在 20 个样本逐项相同，图块库拒绝多余未引用图块。

`arena-a-aux-assets.json.gz` 单独保留 `297056` 解码后的原始 48 块顺序，压缩后为 564 bytes。原 `21258` 六条精灵提交使用三组 4×4 图块，局部起点为 `0/16/32`；位置、当前色盘和提交顺序由世界规则提供。运行数据不保存硬件图块地址；`ba00` 和原首图块 `5d0` 只出现在取证 manifest。该范围在全部 20 个场地 `a` 样本中与原上传源相等。背景 A／B 不引用这个额外范围。

## 行状态与像素证据

输入和唯一原版 3,602 帧控制组由菜单取证提供，本提取器没有重建原版控制组。挂钩内核从冷启动重放同一输入，逐帧核对完整 64 KiB RAM、视频摘要和尺寸。七场全部 25,214 帧一致。合法 Stage Select 会按选项加载多个场地预览；观察器保留全部已知清单加载，验证末次正式加载严格对应所选场地，不能把整个菜单周期误认为只加载一个清单。

七场正式加载都发生在第 2,377 帧，原初色盘在第 2,392～2,397 帧的 `83c` 资源完成出口直接观察。20 个样本取自已确认实战的第 2,700～3,550 帧；每个样本再次校验 `A060`、`8180` callback、正式资源清单和 `f→f+1` 视频时序。观察 wrapper 在原背景绘制入口读取每行寄存器、CRAM、滚动和 VRAM，然后恰好调用一次原绘制函数；CPU 观察维护空白重画、函数重绑定和数据端口写后的色盘状态。

140 个样本保留 31,360 行实际显示输入。所有样本的可见行为 `14..207`，窗口为 `y<48`，使用正常颜色模式和背色索引 0。CRAM0 在 `y<16` 及 `y>=208` 为黑，其余行取当前世界色盘首色，`F140` 与 `E000` 首色绑定一致。各场窗口、A／B、背色及空白均有实际比较像素；每场 13 项负向破坏均被拒绝，包括错误身份、绑定、色盘、有效图块损坏、单元、额外载荷、错误时序、滚动、显示模式和原视频损坏。

ID4 在自然样本的 `E092..E097` 出现四种色盘状态。该场 48 个下方空白行的颜色 9～11 已出现不同于帧末世界输入的值；可见行的全部 64 色一致，空白行只显示颜色 0，因此实际像素仍完全一致。验证器在 `runtime_row_analysis.blank_palette_state_differences` 保留每次差异，不宣称 ID4 每个空白行的全部 CRAM 都等于单个帧末快照。其余六场的 4,480 行完整色盘均与对应世界输入及背色分段一致。

原版不透明精灵像素共排除 499,670 个。比较仍覆盖 9,535,530 个背景像素，其中非黑像素为 7,316,464 个。遮罩包括低优先级精灵，不能据此宣称精灵与平面的完整优先级合成已经通过。观察到的有效绘制行内没有背景 VRAM 变化，20 个样本之间的 A／B 图及引用图块相同；该结论不覆盖未采样的状态、其他玩法模式、列滚动、局部横向窗口、影亮、隔行或扫描线溢出。

## 复跑

在 worktree 根目录执行。前置条件为 [研究工具](../../../scripts/yuyuhakusho/README.md) 中的固定 ROM、原版内核及挂钩内核，以及菜单取证已经保存的七份合法输入、原版 frames 和 manifest。所有新输出均留在本 worktree。任一完整基线、场地身份、资源来源或像素检查失败时，停止导出和后续接入，保留实际差异。

1. 使用已保存的唯一控制组采集原始行状态。

   ```sh
   for arena in 2 4 6 8 a c e; do
     PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_additional_arena_assets.py \
       --arena "$arena" \
       --inputs "docs/research/md-yuyuhakusho/arena-$arena-inputs.json" \
       --frames "games/yuyuhakusho/engine/testdata/arena-$arena-reference-frames.json" \
       --control-manifest "games/yuyuhakusho/engine/testdata/arena-$arena-reference-manifest.json" \
       --out .local/md-yuyuhakusho/additional-arena-assets/references
   done
   ```

2. 导出紧凑背景及场地 `a` 的附加图块。

   ```sh
   PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/extract_additional_arena_assets.py \
     --references .local/md-yuyuhakusho/additional-arena-assets/references \
     --out .local/md-yuyuhakusho/additional-arena-assets/replay
   ```

3. 对照原版验证源数据、全部单元、逐行像素和负向破坏。

   ```sh
   PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_additional_arena_assets.py \
     --assets .local/md-yuyuhakusho/additional-arena-assets/replay \
     --references .local/md-yuyuhakusho/additional-arena-assets/references \
     --negative-checks \
     --previews .local/md-yuyuhakusho/additional-arena-assets/replay-previews \
     --report .local/md-yuyuhakusho/additional-arena-assets/replay-verification.json
   PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_additional_arena_assets.py \
     --assets .local/md-yuyuhakusho/additional-arena-assets/replay \
     --references .local/md-yuyuhakusho/additional-arena-assets/references \
     --aux-only \
     --report .local/md-yuyuhakusho/additional-arena-assets/replay-aux-verification.json
   ```

4. 再次导出，并逐 byte 比较八个运行包。

   ```sh
   PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/extract_additional_arena_assets.py \
     --references .local/md-yuyuhakusho/additional-arena-assets/references \
     --out .local/md-yuyuhakusho/additional-arena-assets/replay-repeat
   for name in arena-assets-2 arena-assets-4 arena-assets-6 arena-assets-8 arena-assets-a arena-assets-c arena-assets-e arena-a-aux-assets; do
     cmp \
       ".local/md-yuyuhakusho/additional-arena-assets/replay/$name.json.gz" \
       ".local/md-yuyuhakusho/additional-arena-assets/replay-repeat/$name.json.gz"
   done
   ```

八个运行资源已在两个输出目录重复导出，JSON 和 gzip 均逐 byte 相同；摘要见 [确定性结果](../../../games/yuyuhakusho/data/additional-arena-assets-determinism.json)。独立背景与原图预览保留在 `.local/md-yuyuhakusho/additional-arena-assets/previews/`。本批没有上传或发布资源，也没有修改默认场地素材、共享 decoder、世界调度、UI 或构建打包。
