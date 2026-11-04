# 人物调色板与索引素材：实现前验证范围

目标为 11 个来源角色及原版换色的实际调色板绑定，随后以已确认的原生 4bpp 图块、图形帧与透明色语义建立独立索引素材。模拟器只用于采集原 CPU／VDP／视频证据；正式素材不依赖 ROM 或模拟器。

可能失败的场景：把来源 ID 与席位或调色板槽混用；仅凭 ROM 中相似的 32-byte 片段猜测绑定；把酎的颜色套到其他角色；遗漏同角色换色、队伍／位置或菜单到战斗的重定位；只观察 CRAM 后倒推未经执行的来源；读取错误的 VRAM／视频时点；调色板字节序、VDP 位重排、亮度、透明色、图块列序或翻转错误；重复图块合并后帧内索引不一致；只做自洽重编码而没有独立原 CPU／像素对照；素材包含原程序或无关 ROM 区；覆盖不全却声称全部角色可呈现；测试后补或临时文件写出 worktree。

先追踪实际调色板写入和设置入口，保存来源、静态引用及完整原调用结果；验证目录与帧的全部声明绑定。若编写独立素材组装／验证器，先保留夹具、编写测试并观察缺实现失败，再加入实现。预览图仅在 worktree 的 `.local/` 中保留；本阶段不发布资源或接入服务器包。

## 实际调色板路径

被动观察 `FFE080..FFE17F` 共记录 1,363 次写入，没有丢弃记录，固定输入的全部 3,600 帧 RAM／画面与未观察基线一致。`0x233c/0x24a6` 在 `FFE080`、`FFE100` 两个源色盘之间选择或插值，写入 `FFE000` 的当前色盘；这部分不等同于角色自身的换色表。自然角色颜色提交的写入 PC 为 `0xfc70`。完整观察保留在 [palette trace](../../../games/yuyuhakusho/data/character-assets-palette-trace.json.gz)。

`0xfc88` 按角色与颜色选择值返回来源指针；`0xfc6a` 选择目标槽并复制十个 word。确切绑定如下：

- 输入 `D2.b` 是来源角色 ID 加一，独立资产使用来源 ID 0～10，不混作房间席位。
- 原始颜色选择值 0～3 使用 `0xfcb8` 中的角色专属指针，加 `selector*32+12`。
- 选择值 4～11 使用共享表 `0x283cc0`，加 `(selector-4)*32+12`。
- `D1.w&3` 选择四个目标色盘槽。只写 `FFE080+slot*32+12` 起的 20 bytes，即颜色索引 6～15；索引 0～5 保留原世界色盘。

这里的 12 个值是已经执行验证的原始选择输入，不代表每条原菜单路径均已操作过，也不为未验证菜单入口起颜色名称。原程序对其他 byte 值存在位运算别名；独立资产接口只开放本次验证的 0～11。

| 来源 ID | 专属四色盘起点 |
| --- | --- |
| 0 | `0x0be8ba` |
| 1 | `0x0c0e5a` |
| 2 | `0x0c3e4a` |
| 3 | `0x0c695a` |
| 4 | `0x0c972a` |
| 5 | `0x0cbd3a` |
| 6 | `0x0ce90a` |
| 7 | `0x0d0fca` |
| 8 | `0x0d363a` |
| 9 | `0x0d63ba` |
| 10 | `0x0d8e9a` |

每个专属色盘段为 32 bytes，但前 12 bytes 不由战斗角色提交函数复制。因此不能直接把整段角色色盘当作最终战斗颜色。片段可以覆盖调色板槽，颜色 3～5 也会用于特效，调用者必须提供相应世界槽的公共前六色。颜色索引 0 透明；其他索引即使颜色为黑仍不透明。

用于绑定结论的入口和可达指令见 [palette flow](static/character-assets-palette-flow.txt) 与 [binding](static/character-assets-binding.txt)。当前 REA 不支持 raw 68000，静态记录来自直接 Ghidra；真实输入／输出由固定原 CPU 执行取得。

## 独立索引格式

[character-assets.json.gz](../../../games/yuyuhakusho/data/character-assets.json.gz) 是可离线读取的索引素材目录，包含 32,567 个按内容去重的图块和全部 1,133 个已确认图形键。它不包含原程序，不需要 MD 内核、VDP 寄存器命令或原 ROM。当前文件仍属于研究与验证目录；正式按角色分包、按需加载与 CDN 发布尚未接入，不得直接夹带进正式服务器构建。

| 字段 | 含义 |
| --- | --- |
| `tile_bytes` | 每图块 32 bytes，逐行、每 byte 高 nibble 在前的 4bpp 索引 |
| `frames[].assignments` | 按原顺序把图块库 ID 写入人物相对图块槽；同槽后写覆盖前写 |
| `frames[].pieces` | 片段中心、1～4 格宽高、首槽、片段翻转及可选绝对调色板槽 |
| `frames[].required_prior_slots` | 本帧使用、但未由本帧赋值的槽，必须保留之前的人物内容 |
| `palettes` | 11 个来源各四种专属颜色尾段，每段十色 |
| `shared_palettes` | 八种共享颜色尾段 |
| `source_address`／摘要字段 | 来源核验元数据，不能作为执行地址 |

运行时只需图块索引、片段和颜色；不需要重新生成 DMA 队列。是否应用本帧图块赋值仍由已恢复绘制规则决定，例如原人物 `+27` 非零会保留所有旧槽。片段中心根据整体翻转、人物深度以及图形偏移计算；片段内多图块按列排列，片段自身翻转再与整体翻转组合。几何语义与原 [sprite oracle](../../../games/yuyuhakusho/data/sprite-oracle.json.gz) 的全部 1,133 个键逐项绑定，该原始目录另有 9,218 次完整原 CPU 绘制输出验证。

原图形数据中有七个帧依赖旧内容，不能擅自补透明图块：

| 来源 ID／图形参数 | 保留的相对槽 |
| --- | --- |
| 1／184 | 83 |
| 2、3、5、6、9、10／0 | 0 |

验证器在缺少旧映射时明确失败；提供映射后保留未赋值槽，并克隆调用者存储。这个限制是原数据的状态依赖，不是可随意丢弃的坏帧。完整世界／呈现接入还需处理人物切换及旧槽生命周期。

## 原 CPU 与像素验证

执行 11 个角色 × 12 个颜色选择值 × 4 个目标槽，共 528 个完整原 CPU 色盘提交。每次比较完整 128-byte 源色盘，核对返回的 ROM 来源、正确目标槽和未改写的公共前六色；没有范围外 RAM 改写。

另对全部 132 个角色／选择值组合声明私有初态，实际执行角色初始化、原色盘选择／提交，然后推进原游戏帧。每个角色包含两种水平朝向；像素场景使用当前世界的槽 1 和站立图形参数 2。采集实际 CRAM、人物 VRAM 图块与原视频，独立索引绘制共比较 172,680 个不透明人物像素，零差异。对应关系保留为状态 `f` 对视频 `f+1`，没有拿同号视频替代。

私有原 CPU 函数调用在返回标记处停止，恢复固定内核至 `aerr_trap` 前的 10,484-byte CPU 前缀及原栈后，才继续推进帧。这避免从临时返回标记继续执行游戏。额外控制只运行一个无 RAM 副作用的选择函数，恢复后 10 帧完整 RAM／视频与原基线逐帧一致；核心内核和原版 RAM 来源没有被替换。

独立 Go 验证器不执行模拟器。它核对全部图块来源、图块去重、帧片段／赋值／旧槽依赖、528 次原 CPU 输出和 132 个像素场景。测试先于实现，真实缺类型／函数失败已保留；负向校验拒绝错误图块、来源、几何、色盘、时序、视频、依赖、摘要绑定，以及缺失或重复角色／帧／场景。

人可查看的预览为 `.local/md-yuyuhakusho/character-assets-preview/contact-sheet.png`，11 行按来源 ID 0～10、12 列按选择值 0～11 排列；没有把角色数字角标画到人物上。预览另核对 132 个单元格与全部 172,680 个验证像素均保留。原生竞态／Node/Wasm 结果、来源摘要和预览 SHA 见 [character-assets-validation.json](../../../games/yuyuhakusho/data/character-assets-validation.json)。

## 复跑和边界

从 worktree 根目录运行，固定 ROM、内核和观察器须已按[研究工具说明](../../../scripts/yuyuhakusho/README.md)准备。输出全部留在 worktree 内：

```sh
mkdir -p .local/md-yuyuhakusho/tmp
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/extract_character_assets.py \
  --trace --out .local/md-yuyuhakusho/character-assets-trace-replay
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/extract_character_assets.py \
  --out .local/md-yuyuhakusho/character-assets-replay
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 ./games/yuyuhakusho/cmd/verify-character-assets
GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" \
  ./games/yuyuhakusho/cmd/verify-character-assets
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go run ./games/yuyuhakusho/cmd/verify-character-assets \
  -preview .local/md-yuyuhakusho/character-assets-preview/contact-sheet.png
```

像素证明只覆盖声明的私有站立场景和普通高优先级精灵；不扩大为全部姿态像素、全部原菜单、世界色盘动态、所有场地、背景合成、扫描线溢出或影亮模式。正式图集／按角色资源包与平台渲染、资源发布仍待接入。完整游戏可玩性仍由人物／世界／AI／平台 E2E 单独验收。
