# Source7 子对象资源审计

本审计核对 `136e6`、`1391e` 子对象和 `20666` 派生的 `149ac`、type `0x24` 特效所需的资源。`15094` 仅修改 pan，不产生 draw。范围限于已保存的原 CPU draw 请求、94 条新增 typed AN／shape、完整流内的主体 AN 消费者，以及现有 `world-sprites` 目录的精确资源。ROM 只作为开发时的只读来源，不写入报告或发行产物。

在编写审计脚本前，固定以下失败场景：

1. ROM、目录、规则、静态证据或原 CPU 夹具的 SHA-256 绑定不符；gzip、schema、重复键或输入变化使证据失效。
2. 子对象初始化的 GraphicsSet／TileBank／AN／shape 与 ROM 指令及保存的对象字段不符；十六进制 pose `0x3c..0x45` 被误读。
3. 94 条新增 typed AN 有缺失、重复、未解释的增加，或 animation 指针、8 字节记录、shape directory、shape group、sound 和图形参数与 ROM 不符；原 CPU 捕获库或实际 AN 提交缺少对应消费者。
4. `draw` 标记与精确请求不一致，或者请求与各自 `1580` 边界保存的对象字段不符；把无 draw 的 pan、creator、clock 或 record 误计为绘制。
5. 生成的 GraphicsSet／TileBank／Parameter 没有精确目录项，或者 transfer／piece／tile 缺失。暂停／慢速 stage-two 脏初态请求必须证明保留并单列，不得当作有效资源覆盖或透明回退。
6. graphic directory、transfer stamp、piece、原始 oracle payload 或边界与 ROM 不符；transfer 长度、tile ID、tile 字节、piece 的 slot 分配或非透明像素不符。真实透明参数 `0` 必须由其精确源 tile 证明。
7. 将 `PreloadedOnly` 特效误作自带图块的动态资源，或从 `TileBank=0` 猜测图块来源；未核对真实 `SkipTiles`、attributes 基槽和所需预载 slot。脏 `PaletteAttributes=0xffff` 造成的预载上下文不能当作默认槽覆盖。
8. 场景默认预载 tile 与原 CPU 保存的 VRAM 字节不同，或把场景 0 的依赖扩大为所有场景的保证。未恢复的压缩资源来源必须明确列为未知。
9. 正常资源键、typed AN 或 type `0x24` 序列缺少真实保存的消费者；审计报告不确定、不能复跑或含有 ROM／tile 原始字节。

审计通过，`missing_assets` 为空。现有目录可直接复用 21 个 GraphicsSet／TileBank／Parameter 精确键及 361 个去重 indexed tile。17 个动态资源键的 transfer、piece、slot、原始源字节及非透明像素均已核对；4 个特效键按原本的预载路径验证，没有把 `PreloadedOnly` 当作动态素材，也没有透明回退。

| 消费者 | GraphicsSet | TileBank | Parameter |
| --- | --- | --- | --- |
| 两个子对象的新增 AN 及实际 draw | `0xd8f1a` | `0x240000` | `0, 100, 102, 104, 108, 110, 112, 114, 116, 118` |
| 完整流内主体 AN 提交 | `0xce98a` | `0x1c0000` | `2, 8, 60, 78, 158, 172, 174` |
| 派生 type `0x24` 特效 draw | `0x27f618` | `0`（保留字段） | `206, 208, 210, 212` |

94 条新增 AN 的十六进制 pose 为 `0x3c..0x45`，均与 ROM 指针、8 字节记录、shape directory／group、sound 及图形参数一致，在三组夹具的实际 AN 提交中均有消费者。捕获库内另有 10 条主体 AN，已与现有基础 AN 目录及 ROM 核对；这些主体 AN 消费者没有被当作完整主体 draw。JSON 逐键保存全部 104 条 AN 的来源地址、摘要和提交计数。新增 94 条与当前基础目录的 4,732 条没有重复。

type `0x24` 的原 ROM 初始化入口为 `0x14d4e`，序列为 `0x14eb6`。七个有效项按 `103, 104, 105, 106, 105, 104, 103` 选择 graphic，每项时长 4，再由 `149ac` 乘 2；末项时长 0，不构成 Parameter `0` 资源请求。七个有效序列位置均有实际 draw 消费者。`20666` 的八处 type `0x24` 写入也已核对 ROM 指令。

正常特效使用 `SkipTiles=1`、基槽 `506`，所需绝对槽为 `506..509`。场景 0 的资源表 `0x21720` 在 `0x21728` 以 kind `6` 将压缩源 `0x27dcb4` 加载到 VRAM 字节地址 `0x1800`。离线解码得到 10,176 字节；其中偏移 `10048..10175` 精确等于现有目录这 4 个槽的 tile 字节，也与两组已保存原 CPU 场景 0 的 frame `2400` 前后所存 VRAM 一致。四块图分别有 `1, 3, 7, 15` 个非透明像素。报告保存压缩源摘要、解码摘要、tile 摘要、tile ID、offset 和检查点引用，不保存原始 tile 或解压内容。

| 原 CPU 夹具 | 调用 | draw 请求 | 精确 frame 覆盖 | 资源来源核对 | 暂停／慢速脏请求 | 脏 palette 预载上下文 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| oracle | 1,378 | 860 | 860 | 860 | 0 | 0 |
| boundaries | 11,323 | 8,752 | 7,456 | 7,456 | 1,296 | 0 |
| sequences | 7,948（8 流） | 2,952 | 2,952 | 2,728 | 0 | 224 |
| 合计 | 20,649 | 12,564 | 11,268 | 11,044 | 1,296 | 224 |

1,296 次暂停／慢速 stage-two 脏请求的完整 draw 与调用前一致，逐条记录在 `retained_dirty_paused_requests`，全部标为 `asset_coverage: false`。它们是边界夹具保留的任意输入，不是正常缺失资源。

私有流 `59-1-False` 将 `PaletteAttributes` 设为 `0xffff`，从而使 224 次特效 draw 的基槽变为 `2047`。这些请求的精确 frame 均存在，但实际所需槽为 `2047, 0, 1, 2`，不属于默认预载槽；原先槽内容没有在子对象夹具中捕获。JSON 逐条单列 `dirty_palette_preload_context_requests` 并标为 `asset_coverage: false`，不据此新增猜测素材。其余 1,120 次特效 draw 使用正常基槽 `506`。表中「资源来源核对」表示动态源图或正常预载源图已核对，不代表已验证这些请求的 cache／skip／全局槽合成。

从 worktree 根目录复跑：

```sh
python3 -B scripts/yuyuhakusho/audit_fighter7_children_assets.py \
  --out games/yuyuhakusho/verification/fighter7-children-assets.json
python3 -B scripts/yuyuhakusho/audit_fighter7_children_assets.py \
  --out .local/md-yuyuhakusho/source7-children-assets-repeat.json
cmp games/yuyuhakusho/verification/fighter7-children-assets.json \
  .local/md-yuyuhakusho/source7-children-assets-repeat.json
shasum -a 256 games/yuyuhakusho/verification/fighter7-children-assets.json
```

最终脚本已由两个独立 Python 进程复跑。报告逐字节一致，大小均为 870,589 字节；33 个来源文件（含审计脚本）均保存 SHA-256，并在结束前复核输入未变。报告含 21 项 transfer／预载 descriptor 和 76 个 piece 的证据。

- 报告 SHA-256：`90206e8fb5d453d029325632946b7607c193b106101363a29338398b84a7604c`。
- 脚本 SHA-256：`e6b643044901bdd5c45d8535e36a8f8d07521c4485e3190e94ebae52aa0c272a`。
- catalog gzip SHA-256：`200e28e775a809de89c19df0dfef314d5697224fcb6a145853beb7344b5833ef`，核对前后不变。
- 子对象 typed AN SHA-256：`72d54890df9bbb66b7a70e0f026e343dc749d971b9450979d103324e65d16aec`，核对前后不变。
- 正常预载源解码 SHA-256：`26b68ec6f0b880e4eca3880bbf2a07df732563391d39b3c606ad4ea6090951d7`。
- 选定 ROM SHA-256：`c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06`。

本审计读取已保存的直接 Ghidra 12.1.4 有界静态清单、冻结的原 CPU 夹具及 ROM 字节。REA 不支持本任务的 raw 68000；本次不执行原 CPU、模拟器或渲染器，也不声称 REA 反编译。资源身份、原图非透明像素和保存范围内的消费者已核对；规则 bundle 接入、cache／skip／全局槽持久化、调色板时序、完整自然帧和 scanline 行为仍由世界／呈现验收负责。

本 owner 只新增本说明、独立脚本及 JSON 报告，未修改 production、共享目录、bundle、UI、Go 测试或 `PROGRESS`，未创建 commit。复跑副本 `.local/md-yuyuhakusho/source7-children-assets-repeat.json` 已清理；`-B` 执行没有生成 Python 缓存。ROM、共享缓存、工具及其他 owner 的文件均保留。
