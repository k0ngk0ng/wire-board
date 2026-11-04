# 场景背景素材：实现前验证范围

本阶段恢复实际加载战场的图块、平面图、优先级、滚动和调色板绑定，供独立呈现使用。原版内核只用于取证；正式客户端不运行 MD 内核，不把视频截图作为场景素材。

实现前失败场景：由文件名或地址间隔猜场地数量／名称；把当前可见裁片当作完整场地；忽略平面 A、B、窗口平面及硬件优先级；图块行列、翻转、透明色、调色板索引或有效颜色位错误；将行／列滚动误作单个相机偏移；忽略窗口覆盖范围、平面宽高和环绕；把精灵覆盖位置的原视频当作背景本色；忽略前后帧 VRAM／视频时序；遗漏动态调色板或图块；把 VRAM 快照直接当作已恢复的 ROM 资源绑定；只重新解码自洽而没有原视频对照；清空输出后宣称零差异；观察器改变整帧；未经验证的单场景结果扩展到所有场景。

先读取实际场地初始化调用及加载后的原版 VRAM／CRAM／VDP 寄存器和视频，保留固定输入完整 RAM／视频基线。再建立真实参考夹具、编写测试并保存缺实现失败，最后加入独立图块／平面组合代码。像素验收须明确遮罩、优先级、时序和覆盖边界；不能把未支持模式当作已通过。预览保留在 worktree 内 `.local/`，本阶段不上传或发布素材。

## 已确认绑定

战斗管理器通过 `0xf96c` 创建 `FF8180` 场地对象，并通过 `0x216f0` 提交资源清单。`A060.w` 是偶数偏移，两个表分别位于 `0xf94c`（对象回调 long）和 `0x21700`（资源清单 long）。八条静态绑定如下；只有偏移 0 在本批自然像素中验证，没有据表项猜测场地名称或菜单可达范围。

| 场地偏移 | 对象回调 | 资源清单 |
| --- | --- | --- |
| `0000` | `20918` | `21720` |
| `0002` | `20ae0` | `2175c` |
| `0004` | `20cb2` | `21798` |
| `0006` | `20e28` | `217cc` |
| `0008` | `20fb4` | `21800` |
| `000a` | `21126` | `21834` |
| `000c` | `212f8` | `21870` |
| `000e` | `21412` | `218a4` |

自然第 1,512 帧实际调用 `0x800`，入口 A0 为 `0x21720`；观察没有丢失记录。默认清单的六条类型 6 资源指向 `27db80/27dcb4/288714/289888/28a41c/28a7f4`，目标分别为 `0000/1800/8000/a000/c000/e000`。类型 6 在 `0x8c8` 初始化源指针和编码 byte 数，由 `0x988` 分块解码到研究 RAM 后上传。类型 0 在 `0x846` 直接复制指定 byte 数；默认最后一项从 `28acf2` 复制 32 bytes 至 `FFE080`，即初始场景的 16 色。独立素材不携带这些解码程序或在运行时解释资源清单。

静态范围见 [arena-assets-bindings.txt](static/arena-assets-bindings.txt)、[arena-assets-loader.txt](static/arena-assets-loader.txt) 和 [arena-assets-scenes.txt](static/arena-assets-scenes.txt)。当前 REA 不支持 raw 68000，本轮静态记录来自直接 Ghidra，未标成 REA 反编译。

## 独立素材与依赖

[arena-assets-default.json.gz](../../../games/yuyuhakusho/data/arena-assets-default.json.gz) 是默认场地的紧凑 A／B 素材：406 个去重 4bpp 图块，每块 32 bytes；两张 64×32 平面图共 4,096 个单元；每单元保留图块库索引、调色板、翻转和优先级。压缩文件为 10,965 bytes，不含完整 VRAM、视频截图或 MD 运行时。

这批自然样本的 A／B 表和引用的图块在全部 20 帧保持相同。验证器逐单元检查原属性高位和原 VRAM 图块全部 32 bytes，并拒绝额外未引用图块。原 VRAM 图块编号仅用于取证；素材中的编号是去重库的局部索引。

目录只保存初始场景色盘，不把它误作整局固定颜色。第 2,605 帧起，`FFE080` 首 16 色已有自然变化；场地附加对象 `150ca/1525e` 的色盘推进仍须另外恢复。运行时必须提供当前色盘、A／B 水平滚动行、垂直滚动、可见／空白行及窗口／HUD 数据。窗口包含当时角色头像、姓名和生命条，不进入本 A／B 目录，也不能作为全部角色的通用 HUD。

## 像素与时序验证

选择固定输入的 20 个自然帧。原版参考保留帧末状态 `f`、视频 `f/f+1`，另通过观察原背景绘制函数入口记录视频 `f+1` 实际使用的 4,480 行显示状态。观察 wrapper 每次只调用原函数一次；CPU 观察维护函数指针重绑定、空白重画及 VDP 数据端口写后色盘。未改游戏 RAM、ROM 或原指令流程。全部 3,600 帧完整 RAM／视频与未挂钩原版基线一致。

早期仅使用帧末寄存器时，背景与视频不一致。实际默认画面只有第 14～207 行绘制；第 16 行还有 CRAM0 更新。这个过程与精灵准备状态 `f` 对视频 `f+1` 的规律不能混为一个静态寄存器快照。独立比较保留真实空白行、滚动、调色板及窗口输入，没有扩大精灵遮罩来消除差异。

[独立 Go 解码器](../../../games/yuyuhakusho/cmd/verify-arena-assets/main.go) 自行解析 8×8 4bpp 图块、水平／垂直翻转、调色板、平面 A／B 优先级和上方窗口。20 帧共 1,354,388 个未被精灵遮罩排除的像素与原视频零差异，其中非黑像素 1,125,356 个。来源计数分别为 A 548,773、B 415,428、窗口 91,756、背色／空白 298,431。

另有 79,212 个原不透明精灵像素被保守排除，包括低优先级精灵。它验证背景像素，不证明精灵与平面的完整优先级合成。静态背景 VRAM 检查覆盖 `0000..3fff`、`8000..efff`；窗口所用其他图块仍由最终像素对照约束。本批没有检测到所查区域在有效绘制期间发生变化。

当前支持已观察的 H40、64×32、逐行水平滚动、全局垂直滚动、无横向局部窗口及正常颜色模式。列滚动、局部窗口硬件边界、影亮、隔行、扫描线溢出和其余场地不在本批验收范围。负向测试拒绝错误时序、图块／色盘／滚动损坏、截断／不支持配置，以及紧凑素材的图块、单元和绑定损坏。三轮实际缺实现失败先于相应解码、行状态和目录实现；4 项顶层测试在原生竞态和 Node/Wasm 均通过。

完整来源、日志、参考和边界见 [arena-assets-validation.json](../../../games/yuyuhakusho/data/arena-assets-validation.json)。独立背景和原图对照保留于 `.local/md-yuyuhakusho/arena-assets-preview/`，源状态 1,855 对原视频 1,856；预览不含人物、阴影和「1 回战」精灵。

## 复跑

从 worktree 根目录运行，固定 ROM、原版内核与加钩子内核须已按[研究工具说明](../../../scripts/yuyuhakusho/README.md)准备。

```sh
mkdir -p .local/md-yuyuhakusho/tmp
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/extract_arena_assets.py \
  --out .local/md-yuyuhakusho/arena-assets-replay
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go run ./games/yuyuhakusho/cmd/verify-arena-assets \
  -reference .local/md-yuyuhakusho/arena-assets-replay/arena-assets-reference.json.gz \
  -catalog-out .local/md-yuyuhakusho/arena-assets-replay/arena-assets-default.json.gz \
  -preview .local/md-yuyuhakusho/arena-assets-replay/background.png
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -json ./games/yuyuhakusho/cmd/verify-arena-assets
GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -json -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" \
  ./games/yuyuhakusho/cmd/verify-arena-assets
```

尚未恢复的默认场景更新、相机／水波滚动和动态色盘不能由这些静态素材替代。正式背景分包、其他七个场地、全部 HUD、独立平台渲染与发布仍未完成。本阶段没有上传素材，也没有把客户端素材嵌入服务器构建。
