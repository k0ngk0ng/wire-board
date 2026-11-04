# 独立背景与精灵合成：实现前验证范围

本阶段将已验证的默认场地索引图块和 A／B 单元转换为可复用呈现接口。运行时只接收索引平面、逐行滚动、色盘、背色、显示门槛和可选窗口，不接收 ROM、VRAM、寄存器或参考画面。参考夹具仅用于测试适配和原视频对照。

实现前失败场景：图集身份、图块长度、索引、单元属性或平面尺寸无效；把原硬件图块号当局部图块号；滚动正负方向、环绕或翻转错误；透明索引与不透明黑色混淆；A／B 优先级错误；缺少窗口却静默显示错误背景；逐行色盘、颜色限制或空白门槛遗漏；精灵盖住空白行；低优先级精灵盖住高优先级背景；背景透明像素错误阻挡精灵；合成修改源图；截断数组导致 panic 或部分结果。

先编写原视频对照及负向测试，保存新接口缺实现失败，再实现。测试复用已固定的 20 帧原视频和 4,480 行实际显示观察；窗口由测试将原参考单元转换成独立索引输入，不加入默认场地图集或复刻运行时。精灵优先级另对照固定原内核 `vdp_render.c` 的 `make_lut_bg`／`make_lut_bgobj`，并在现有自然原视频中核对。

## 接口与所有权

`DecodeArenaCatalog` 解码已经绑定原资源清单的默认场地，返回两个 `IndexedPlane` 和初始 16 色。A／B 共享只读索引图块，调用方不得修改已加载目录。`ArenaFrame.Rows` 每行显式提供水平／垂直滚动、当前 compact 色盘、背色、正常显示／空白与窗口选择；可选窗口使用自己的独立索引图块和单元，不读原内存。

`RasterizeArena` 返回 `ArenaRaster`：`Raster.Opaque` 区分选中平面非零索引与背色，`HighPriority` 保留选中平面的优先级，`Sources` 为背色／A／B／窗口，`Visible` 保存逐行显示门槛。`CompositeArenaSprites` 接收已经按原提交顺序生成的精灵平面；低优先级精灵被不透明高优先级背景遮挡，高优先级精灵覆盖背景，空白行只显示背色。合成后为完整不透明画面，可直接使用 `RGBA()`；没有修改两个输入。

## 实际结果

三项聚焦测试在 native race（3.553 秒）和 Node／Wasm（0.753 秒）全部通过，`go vet` 通过。缺新类型／新函数的真实失败保存为 `arena-presentation-red.jsonl`，先于 `arena.go` 实现。

- 原 20 帧、4,480 行：1,354,388 个背景像素与原视频零差异，原精灵遮罩为 79,212 个像素；范围与先前场地验收相同。
- 使用测试中的原精灵单元适配器后，完整合成 1,433,600 个像素全部与原视频一致。这验证自然样本中的普通模式图层合成，不将参考精灵适配器作为复刻运行时。
- 128 个组合核对 A／B／精灵透明度、三个优先级位、逐行显示门槛与不透明黑色。真值关系来自固定原内核普通模式优先级表，源片段保存在 [arena-composition-core-source.txt](../../../docs/research/md-yuyuhakusho/static/arena-composition-core-source.txt)。
- 自然显示实际覆盖水平／垂直翻转及其组合、水平正负滚动环绕、7 个垂直滚动值、逐行色盘、上方窗口与空白行。正常可见行为第 14～207 行，窗口输入共 680 行。
- 8 个坏目录和 5 个坏帧输入、缺少窗口、截断精灵数组均明确失败；错误不修改目录或源图。

默认 A／B 索引目录不含动态 HUD／窗口图块。窗口的头像、姓名、生命条仍需由独立规则和资源构造；不得将本测试参考窗口当作新对局状态。普通模式以外的影亮／隔行、列滚动、局部横向窗口、硬件扫描线精灵溢出及其余七场地未验证；`Limited` 只调用现有颜色转换，本批自然可见行没有该模式，不扩大其证据。世界状态到逐行显示输入、完整平台页面与音频仍由后续集成验证。

## 复跑

从 worktree 根目录运行，不需 ROM 或模拟器进程：

```sh
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -json -run '^TestArena' ./games/yuyuhakusho/presentation

GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -json -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" \
  -run '^TestArena' ./games/yuyuhakusho/presentation
```

来源、fixture／代码摘要与实际日志绑定见 [arena-presentation-validation.json](../data/arena-presentation-validation.json)。本阶段没有增加媒体副本、临时 profile 或私有缓存；共享 Go 缓存仍供整体集成使用，不在子任务中清理。
