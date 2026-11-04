# 人物精灵呈现：研究边界

目标是恢复原版人物图形集合、图块、调色板和精灵组合的格式，供独立渲染器使用。模拟器只用于原版运行与验证，不进入正式游戏运行时。

实现前失败场景：把动画／碰撞目录当作人物精灵；仅凭 ROM 地址间隔假定图块压缩；混淆人物图形集合与图块基址；忽略 `+3c` 图形参数的索引单位、相对偏移或大小；将原版 VDP 数据自动当作完整素材边界；图块行列顺序、水平／垂直翻转、调色板索引或透明色错误；只比较重新编码的自洽性而没有独立原版像素对照；只恢复站姿就声称全部角色／动画可呈现；观察器改变 RAM、视频或时序；缓存、图片和编译产物写到 worktree 外；把客户端素材带入正式服务器包。

先被动观察原版实际绘制读取，并核对固定 3,600 帧 RAM／视频基线。静态分析只跟随已确认的读取和调用入口；用原 CPU／VDP 输出验证解释，未验证的格式保持未知。若编写解码器，先保存原版夹具、编写测试并保留缺实现失败，再添加实现；媒体产物暂存 `.local/`，不发布。

像素验收另外防止：将 `retro_run` 返回后的 VRAM 与同号视频错误配对；遗漏 MD 精灵按列排列的图块顺序；把调色板非零索引的黑色误作透明；忽略整片水平／垂直翻转、精灵链优先级或高优先级覆盖；把所有像素都透明当作零差异；仅验证 VRAM 解码而未核对 ROM 图块传输来源；误称完整背景、低优先级合成或 MD 所有显示模式均已恢复。

## 已确认路径

固定自然输入第 1,800～3,600 帧中，人物图形字段的读取 PC 只有三处：`0x16ea` 读取 `+1c`，`0x16ee` 读取 `+3c`，`0x1922` 读取 `+20`。目标 owner 为 `8400/9000`，没有观察器记录丢弃，全部 3,600 帧 RAM／画面与原基线一致。观察范围仅限所声明人物字段与该输入片段，不代表全 ROM 所有绘制分支均已执行。

完整绘制入口是 `0x1580`，依据人物 `+3e` 选择普通、`0100` 和 `00d8` 的坐标分支。`0x18e2` 比较人物四个缓存描述符，`0x191a` 建立图块传输请求。`0x722` 只把原生 VDP DMA 寄存器写入排队，`0x614` 在 VBlank 写入 VDP 控制口；此路径没有图块解压。静态证据见 [sprite-consumers.txt](static/sprite-consumers.txt)、[sprite-submit.txt](static/sprite-submit.txt) 和 [sprite-depth-d8.txt](static/sprite-depth-d8.txt)。当前 REA 不支持 raw 68000，本轮静态记录来自直接 Ghidra。

| 人物字段 | 已确认用途 |
| --- | --- |
| `+1c.l` | 图形集合：动画提交的 `+3c.w` 是相对 word 表的字节索引 |
| `+20.l` | 原 ROM 图块基址；每个图块 32 bytes，原生 4bpp |
| `+24.w` | VDP 图块基号、优先级、调色板与整体翻转位 |
| `+26.b` | 精灵层组，按 `&fc` 选择链尾 |
| `+27.b` | 非零时跳过图块请求和描述符缓存改写 |
| `+28.w / +2a.w` | 加入屏幕坐标的图形偏移 |
| `+2c..+3b` | 最多四个已去除终止位的图块描述符缓存 |
| `+3e.w` | 精灵片段中心偏移的定点缩放，硬件片段尺寸仍为 8～32 像素 |

## 图形帧格式

设图形集合为 `G`、图形参数为 `P`，帧地址为 `G + signed16(ROM[G+P:G+P+2])`。参数是字节偏移，不能再乘二。现有动画／形状目录实际引用了 11 个来源集合的 1,133 个不同 `(G,P)` 键；这不是 1,133 个招式，也没有证明全部表项均可通过正常玩法到达。

每帧先包含一至四个 big-endian 32-bit 图块描述符。bit31 标记末项；即使没有末项位，原循环最多读取四项。去除 bit31 后：高 16 位为来源图块号，低 word 的高 byte 是目标图块偏移，低 byte 是图块数。来源地址为 `tilesBase + sourceTile*32`；目标 VRAM 地址为 `((descriptor&ff00)>>3) + (attributes&07ff)*32`，保留 word 回绕。DMA 长度是图块数乘 16 个 word。缓存未改变时不重复排队，最后一项之后的旧缓存槽清零。零图块数也按原指令生成请求，不能凭常识额外改写原行为。

随后是 32-bit 精灵片段，bit31 标记末项。高 word 的 bits8～14 与 bits0～6 分别编码 X／Y 中心偏移，普通表对应 `index-64`，整体翻转使用 `64-index`。低 word 的 bits8～11 编码宽高：宽为 `((size>>2)+1)*8`，高为 `((size&3)+1)*8`；低 byte 是相对图块号。bits12～15 右移一位后与人物属性异或，包含片段翻转及可覆盖人物调色板的位。

中心偏移先乘有符号 `+3e.w`，算术右移八位，再减去片段半宽／半高，最后加屏幕中心。`0100` 不缩放，`00d8` 的专用查表与该公式逐项一致。VDP 片段内的多图块按列排列；水平／垂直翻转作用于整个片段。颜色索引 0 透明，其他索引即使颜色为黑也保持不透明。

## 原 CPU 与独立组合器

每个引用图形键执行四种整体翻转与 `0100/00d8` 两种深度，共 9,064 次原 CPU 调用；另有 154 个裁切、缓存复用、跳过 DMA、其他有符号深度、精灵计数上限和 byte 回绕边界，总计 9,218 次。调用关闭中断，从研究存档恢复后只设置已声明私有输入，实际执行 `0x1580` 至外层返回标记 `0x3d8`。不以函数内部的 JSR／RTS 当作完整返回。

[独立 Go 验证器](../../../games/yuyuhakusho/cmd/verify-sprites/main.go) 自行解析帧、选择输入并组合输出，比较完整 256-byte 人物、DMA 缓冲、精灵缓冲、层链缓冲、游标、计数与溢出标志，9,218 次全部一致。它不执行 ROM 或模拟器，属于资源恢复验证工具，不是最终游戏渲染器。测试与真实缺实现失败先于实现；负向校验拒绝缺帧、错误键／返回、损坏输出、重复或缺失场景以及无效缓冲输入。负向测试保留全部断言，并使用值切片副本隔离修改，避免反复序列化整份大夹具。

夹具只保留实际引用的原始图形帧及原 CPU 输出，不携带原 ROM 程序或完整图块库。见 [sprite-oracle-manifest.json](../../../games/yuyuhakusho/data/sprite-oracle-manifest.json)。

## 自然像素验证

另采集 20 个自然帧的 RAM、VRAM、CRAM、VDP 寄存器和视频。该固定内核在 `retro_run` 返回时，VRAM 状态已经准备下一视频帧；应把状态 `f` 与视频 `f+1` 比较。同号视频保留用于负向验证，没有覆盖或删除错位证据。

[独立像素解码](../../../games/yuyuhakusho/cmd/verify-sprites/pixels.go) 从 VDP 精灵链、列序图块、翻转和正常 CRAM 颜色自行生成候选像素，20 帧的 71,646 个高优先级不透明精灵像素全部与原视频一致，其中 53,185 个来自人物图块。41 段人物缓存描述符指向的 ROM 内容也与 VRAM 逐字节一致。参考片段的来源角色只有 ID 9；全角色几何验证不能扩大为全角色调色板或像素验证。

像素测试拒绝同号视频、错误视频帧号、清空图块／调色板、错误 ROM 来源和未支持的影亮模式。它验证选定普通显示模式下的高优先级精灵，不声称完整背景、低优先级合成、扫描线溢出、全部硬件模式或完整游戏呈现。完整结果、来源摘要和原生／Wasm 测试日志在 [sprite-validation.json](../../../games/yuyuhakusho/data/sprite-validation.json)。

## 复跑与后续接入

从 worktree 根目录运行，原版工具与固定 ROM 须已按研究工具说明准备：

```sh
mkdir -p .local/md-yuyuhakusho/tmp
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/trace_sprite_consumers.py \
  --out .local/md-yuyuhakusho/sprite-consumers-replay
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/extract_sprites.py \
  --out .local/md-yuyuhakusho/sprite-replay
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -json ./games/yuyuhakusho/cmd/verify-sprites
GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -json -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" \
  ./games/yuyuhakusho/cmd/verify-sprites
```

测试读取已保留的受摘要约束夹具；原版复跑输出可与同名夹具比较。`go run ./games/yuyuhakusho/cmd/verify-sprites` 仅输出图形组合／DMA 对照报告，像素验证包含在上述测试中。

后续需要恢复各角色和换色的调色板来源，确定最终的索引图块／图集与片段元数据格式，再接入独立游戏渲染。原版图块可以按已确认传输描述符离线提取，正式客户端不需要 MD 内核或 DMA 缓冲。客户端发布素材继续使用平台的按需加载与 OSS/CDN 规则，不进入正式服务器包；本轮没有发布任何资源。
