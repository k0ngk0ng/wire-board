# 独立 HUD 与窗口：实现前验证范围

本阶段从固定原资源、字体、头像、生命／能量条数据恢复独立窗口素材和事件适配。新对局必须由空白窗口、原始固定资源及明确规则事件建立；捕获后的窗口图仅为原版对照，不能充当新对局初态。

实现前失败场景：将 E600 共享单元直接误当完整窗口；把 E602 人物 HUD 视图与 E600 起点混淆；把窗口地址 D000 与单元／图块局部索引混用；由截图补头像或姓名；资源解压长度、回溯、分块边界、字节序、图块顺序错误；字体字形、行进、透明色、属性高位错误；头像路径误用普通精灵绘制；血条图块与生命值直接重画导致脱离原每帧缓动；遗漏原事件顺序或共享图块覆盖；坏事件部分提交；改变规则世界状态；从参考状态伪造新对局；只比较自身解码而不与真实原版窗口／视频核对。

先恢复资源和写入路径证据，再编写独立适配器测试并保存缺实现失败，最后实现。静态研究使用直接 Ghidra 68000；REA 不支持此 raw 目标，不能将直接 Ghidra 结果写成 REA 反编译。全部媒体与运行证据留在授权 worktree，不上传或发布。

## 原始资源与写入关系

默认资源清单 `21720` 首项把 `27db80` 的压缩流解码至图块起点，独立离线解码得到 16 个基础 4bpp 图块；全部 512 bytes 与原 20 帧实际加载结果一致。条形图块直接来自 `271a00` 的 32 个 pattern。姓名使用 `24b9e0 + code*128` 的 16×16 字形，每字 4 个 8×8 图块，列序排列；11 个角色共需要 20 个去重字形编号，包含空白字形 0。头像由 `dabca` 表项绑定 `260000` 的 11 份 4×4 图块，表项的传输与组合描述符逐项校验，不由地址间隔猜测。

`FDbe→0fe0` 从 `E706 + HUDOffset` 开始写三字姓名，各字宽两单元、高两单元；名称图块从逻辑槽 `20` 开始，逐字增加 4 槽。`FDFc→20dc` 是直接单元绘制，不是普通精灵提交：头像的 4×4 单元写入 `E600` 共享区域，图块占 `300 + slot*10`（十六进制），保留事件中的颜色属性。生命／能量条只消费原规则的 `FighterBarTransfer`，不根据当前生命值重新设计或重画。

`D24` 将 `E600` 起的 257 words 提交至窗口 `D100`；实际 HUD 对应窗口第 2～5 个图块行。最后一个 word 属于第 6 行，位于本场景窗口显示范围以外。场景初始化 `30f4..310e` 向 `D080` 写 40 个 `E00d` 单元，构成窗口第 1 行的固定背景。新窗口其余单元为透明索引；不读取原版完成后的窗口图。

静态路径见 [hud-present.txt](../../../docs/research/md-yuyuhakusho/static/hud-present.txt)、[字体与头像辅助路径](../../../docs/research/md-yuyuhakusho/static/hud-present-helpers.txt)和[场景初始化](../../../docs/research/md-yuyuhakusho/static/hud-present-scene-init.txt)。离线压缩解析沿用已固定的[原加载指令](../../../docs/research/md-yuyuhakusho/static/arena-assets-loader.txt)，不进入运行时。

## API 与单元所有权

```go
catalog, err := presentation.DecodeHUDCatalog(assetJSON)
hud, err := presentation.NewHUD(catalog)
// 初始 NewChuSession 的 reset/resource 事件和之后每一轮事件均须交付。
hud, err = presentation.ApplyHUDWorldEvents(hud, events, world.Cells[:256], catalog)
window, err := presentation.HUDWindow(hud)
// 将 window 传给 ArenaFrame.Window。完整调用须检查每一步错误。
```

`HUDState` 只保存呈现单元、稀疏逻辑图块赋值、姓名游标和单元所有权。`words` 标记原规则拥有的单元，姓名／头像在覆盖时移除这些位置的规则所有权；之后只从 `world.Cells[:256]` 同步规则拥有的位置。这样保留原版头像覆盖条框首单元的顺序，也防止规则状态中尚未物化的姓名／头像被零值覆盖。适配器不修改世界规则，不读取角色当前生命、能量或胜负。保存／恢复需包含这份呈现状态，不能只恢复世界后重置姓名或图块上下文。

所有事件按原顺序应用。未知字形、头像、条形 pattern、目标槽、资源及越界单元明确失败，返回原状态；晚期错误也不泄露前面的图块赋值。窗口构造将当前逻辑槽转换为局部索引图块，缺少实际引用的图块时失败，不擅自补透明。

## 默认显示行

`DefaultArenaRows` 接收当前 compact 色盘和两组逐行水平／垂直滚动，构造 224 行默认战斗显示输入：第 14～207 行可见，`y<48` 使用顶部窗口；颜色 0 在 `y<16` 或 `y>=208` 时为黑，其余行使用当前色盘首色。其余 63 色不变。20 个原帧中 `F140` 与 `E000` 首色一致，4,480 行实际色盘、显示门槛和窗口选择逐项相同。

早期把全部 224 行垂直滚动假设为同一值，发现第 221 个空白行可提前接收下一帧 VSRAM。接口因此保留逐行垂直输入，不把这个实际差异裁掉。独立世界可在本帧各行提供其已完成的滚动值；空白行不会采样平面。`FFF0f2` 的 `74020003` 是 VRAM 写命令，不是窗口寄存器切换。

## 实际结果

五项测试在 native race（5.557 秒）及 Node／Wasm（1.578 秒）通过，`go vet` 通过；测试的真实缺接口失败保留为 `hud-red.jsonl`。行显示另保留缺接口失败与逐行垂直输入契约失败。

- 两次完整 3,600 帧原 CPU 采集，完整 RAM／视频均与固定基线一致；两次图集、fixture 和清单逐字节相同。
- 10 次自然窗口写入、55 次原 CPU 隔离调用（11 个姓名、44 个头像）完整 512-byte 单元输出一致。
- 从空 HUD、ROM 原始图集、1,134 条有序原事件及声明的规则单元输入重建 20 个窗口。可见窗口的 4,800 个单元及其引用图块逐项一致，91,756 个原窗口像素零差异。
- 全新独立 2／3／4 人对局各推进 500 ticks，姓名、头像、条形图块和窗口可连续构造。该验证不载入原窗口或参考对局初态，不等同于平台浏览器 E2E。
- 坏事件、晚期错误和截断规则输入原子拒绝；规则同步保留姓名／头像，同时更新原条形单元。缺图块及错误图集身份明确失败。

`hud-assets.json.gz` 为 5,395 bytes，只含原始索引素材与固定条栏描述；`hud-reference.json.gz` 为 9,039 bytes，仅供开发测试。来源、日志、SHA 和复跑结果见 [hud-validation.json](../data/hud-validation.json)。

## 覆盖边界

当前头像坐标范围覆盖默认两人／三人布局。四人布局沿用原规则省略头像。原三人布局应使用槽 `0/1/3`；若另选包含槽 2 的三人组合，人物图块 `300` 会与头像槽别名，需要按原选槽规则或共享图块赋值恢复，不能把任意三个槽视为等价。此模块的稀疏 HUD 图块没有模拟人物图块对头像槽的覆盖。

自然窗口证据为默认场地及来源角色 9；全 11 个姓名与头像的单元布局另由原 CPU 隔离验证。其余角色完整玩法、其余场地、单人故事布局的头像越界行为、其他字体／菜单、训练模式动态文本及硬件显示模式不在本批范围。完整音频、平台房间、网络和浏览器输入验收另行完成。

## 复跑

从 worktree 根目录运行。重新采集需要已固定的 ROM、原研究内核及观察过滤器；运行 Go／Wasm 验证只读取声明的索引素材和开发 fixture，不执行模拟器。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/extract_hud_assets.py \
  --out .local/md-yuyuhakusho/hud-replay

GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -json -run '^Test(HUD|DefaultArenaRows)' \
  ./games/yuyuhakusho/presentation

GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -json -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" \
  -run '^Test(HUD|DefaultArenaRows)' ./games/yuyuhakusho/presentation
```
