# 酎世界对象轮次

组合原 `0x3592` 的已就绪对局对象轮次。调度从 `8000` 至 `A000`，调用前保存当前 header 的 stride；空 header 跳过 64 bytes。某回调在未来地址建立的新对象必须在当前轮次读取并执行，不能预先固定整轮活动对象列表。碰撞管理器先消费上一轮形状，再由人物／子对象提交下一轮形状。

失败场景：按修改后 stride 推进、忘记未来槽本轮执行、按类型分组批量更新破坏地址顺序；共享 RNG／暂停／慢动作／相机／色盘视图冻结在轮次开始；每人物重置世界缓冲；父对象／子对象／FX 各存独立字段后失步；HUD、行滚动和色盘／统计别名拆成多份；清槽不应用完整拥有字段；将 Draw=false 当作删除；未恢复回调静默略过；把期望原中间态或音频中断队列写回 Go；把旧菜单外层或硬件精灵编码误当纯对局规则。

先保留原版整个对象轮次的实际执行顺序、完整输入／输出与原版基线；列出所有实际回调及边界，再写组合测试和实际缺实现失败，最后实现独立 typed 轮次。连续验证只初始化一次，外部控制输入经受信采样器提供，前后驱动由已恢复规则推进，不逐对象或逐帧回灌内部状态。硬件精灵独占缓存／DMA 与独立音频消费的排除必须逐项说明，不能扩大成未解释字段屏蔽。

本轮 API 不执行原菜单；主对象 `9ad0` 只在已经证明的等待状态作为边界，独立对局清场后停止。场景／色盘／输入等帧驱动属于外层明确组合，不以预期完成标志代替驱动。

## API 与唯一状态

`StepChuWorldObjects(ChuWorldState, ChuWorldView, ChuWorldLibrary)` 返回新状态、有序语义事件和错误。`Objects` 是唯一对象集合；共享输入、随机数、暂停／慢动作、相机、色盘、形状／投掷缓冲、FX 游标、统计和对局配置分别只有一个所有者。每个回调从当前世界构建临时组件视图，并立即回写。没有持久化每个人物自己的 `ChuActorWorld`，也没有生产 RAM 镜像或 tick 内的字节编码适配。

`Cells[768]` 对应共享 `E600..EC00`：人物 HUD 使用 `[1:513]`，场景前缀使用 `[:256]`，两组行滚动使用 `[256:512]` 和 `[512:768]`。记录结束标记使用 `LossesByCharacter[60:64]`，没有第二份数组。人物反馈请求直接使用 `Clock.DurationRequest/AmplitudeRequest`，标签闪烁直接取 `uint8(Clock.Frame)`。生命周期的 ResultTicks 与人物 BattleState 同属 `Match.ResultTicks`。

开局 manager 的 `FE68` 模板写入使用 `E600` 基址，按 byte 偏移物化到同一 `Cells`，不能沿用人物 HUD 的偏移一 word 视图。6 次自然与 121 次隔离原 CPU 写入、非法边界原子拒绝及六种新鲜配置的首帧集成，见[开局共享单元补充验证](CHU_WORLD_START_CELLS_VERIFICATION.md)。名字和硬件字形仍是明确的呈现事件边界。

活动回调的 stride 在调用前保存，空槽只前进 64 bytes。对齐／身份错误、缺少规则数据、未恢复回调和源角色 9 以外的活动人物均返回错误；已完成的前置更新不会泄露到调用者。删除使用组件报告的完整 `ClearedBytes`，不会用 `Draw=false` 推断删除。未来槽在同一轮运行，碰撞先消费旧形状，场景先消费旧坐标样本，人物再提交新形状和样本，影子最后消费登记。

事件按地址顺序返回：`callback` 保存入口 header 与 stride；`draw`、`hud`、`start` 描述呈现操作；`clear`、`clear-world` 描述实际删除。HUD 只生成 Pattern／TargetTile，不模拟硬件 DMA。主等待对象只接受 `8000/9ad0/stage20`；生命周期清理 `8040..A000` 后设置 `MatchComplete` 并停止，不进入原菜单。

## 原版整轮证据

| 原版输入流 | 更新轮次 | 实际回调 | 边界 |
| --- | ---: | ---: | --- |
| 默认完整对局 | 2,957 | 38,448 | 第 1,800 个视频标签的已就绪状态至第 4,757 个标签的清场 |
| 特殊动作输入 | 1,801 | 23,885 | 第 1,800～3,600 个视频标签，截窗结束时未清场 |

两条输入都从冷启动运行。默认流与既有 5,500 帧未观察内核基线一致；特殊流另外运行未挂钩原内核，与观察内核的 3,600 帧完整 RAM／视频基线一致。没有修改原 CPU 人物状态或注入每帧预期值。采样保存每次 `3592` 的实际调用顺序、调用前 stride、A5 前后值和完整返回栈边界，全部调用完整返回。初次整轮采样缺少形状缓冲载荷，已补采 `A200..A800`；最终夹具不沿用缺载荷版本。

默认流覆盖 18 类实际回调。特殊流自然执行 `1313e` 弹体回调 3 次、`132de` 蓄力附件回调 164 次、`1361e` 释放附件回调 13 次。两条流合计覆盖 21 类回调。这些数值是回调次数，不是独立招式或弹体的数量。

既比较每个完整轮次，也从每条流的第一份就绪状态只初始化一次，组合 `FinishWorldPalette` 与下一次 `BeginWorldFrame` 连续推进。后续只提供受信的归一化控制器输入；不把原版的对象、RNG、相机、色盘、碰撞缓冲或完成标志送回 Go。

完整比较包含 `8000..A000` 对象区、`A000..A200` 全局区、`A200..A800` 形状／投掷缓冲、`F550..F590` 时钟窗口与统计原始字节，以及全部已恢复语义状态、共享 HUD／滚动单元、相机样本、登记、色盘和事件顺序。验证适配器从原输入的未知字节构造输出，真正的清槽事件清除对应前缀；未恢复字段如果在原 CPU 中出现新写入，会使字节比较失败。

最终 13 项顶层聚焦测试包含 5 项世界组合测试和既有 8 项统一对象测试。原生 `-race` 通过，114.163 秒；Node/Wasm 通过，30.326 秒；完整 engine 包的 `go vet` 通过。两条流的 4,758 轮、62,333 次回调在逐轮及单初态连续验证中全部吻合。世界实现前的真实构建失败保存在 `chu-world-red.jsonl`，并绑定当时的测试源码 SHA；后续额外自然流与原始字节核对作为同一 E2E 验证扩展记录，没有新增生产规则。

## 视频标签与规则调用

视频标签不是规则 tick。默认记录没有视频标签 4,581 的 `3592` 调用；相邻两次对象轮次发生在标签 4,580 与 4,582，而规则 Frame、逻辑计数和帧前 RNG 只前进一步。新增 `3a2e` 入口观察确认：这两轮分别对应标签 4,579 与 4,581 的 driver 入口，中间没有标签 4,580 的入口。

默认流后续的 2,956 轮各有一次 driver 入口，其中 2,357 次在对象轮次前一视频标签，599 次在同一视频标签；第一轮 driver 在采样窗前。跨标签执行和忙门槛均属于原硬件调度，不能按视频标签差额补调 Go 规则。静态 [battle IRQ](../../../docs/research/md-yuyuhakusho/static/world-frame-battle.txt) 的 `32d6..3312` 忙分支可在到达 `335e→3a2e` 前返回。连续验证按实际 driver 调用推进一次，独立平台的 tick 频率由外层调度。

## 呈现与音频比较边界

- 对象 `2c..3b` 是精灵绘制器独占缓存，规则比较只排除这 16 bytes；`28/2a` 图形偏移仍完整比较，因为它们参与碰撞。
- 时钟采样窗口内只另排除 `F578/F579` 的精灵计数和 `F57C/F57D` 的精灵输出游标。原 [sprite-submit](../../../docs/research/md-yuyuhakusho/static/sprite-submit.txt) 在 `15f2/15f6` 读取、`16ac/16b0` 写回，独立呈现模块另有原版验证。没有排除任何规则时钟、暂停、慢动作、反馈或 RNG。
- `F518` 及 DMA／硬件精灵队列属于呈现阶段。世界规则使用每轮逻辑 HUD 游标，不读取或回灌原硬件游标。
- 默认流有 85 轮、特殊流有 84 轮受到 `37c56/37cac` 独立音频队列消费打断。其余 4,589 个完整轮次精确比较音频请求队列。连续规则流不回灌中断后的队列；队列一致性不计入连续状态相等，但 PendingSound、暂停音频标志及全部游戏规则仍比较。此项不证明原音频 CPU 的消费时序或完整声音重放一致。

## 语义规则包

`data/chu-world-rules.json.gz` 解压为 JSON，使用 `DecodeChuWorldRules` 在模拟前解码一次。内容为 11 份人物参数、16 份命令程序、4,746 份父人物／弹体动画记录、8 份蓄力序列、21 份释放附件动画、37 种 FX 初始化／21 份 FX 序列、3 份附着效果序列，以及场景与开局语义表。解压 1,879,860 bytes，压缩 86,596 bytes。

导出器只取已恢复库的语义字段；来源文件包含的 CPU 调用样本、RAM 字节和媒体没有进入规则包。测试将运行时包与逐个独立解码的来源库比较。来源摘要和包摘要见 `data/chu-world-rules-manifest.json`。

本模块的活动人物动作仍限源角色 9（酎）。开局 0／2 阶段、训练和更多标签分支已连接其独立恢复组件，但本轮整轮 CPU 轨迹从已就绪状态开始；新世界构造、资源就绪交付、平台席位、联机、呈现像素和整套音频属于外层验收。规则包包含 11 个角色的数据不代表 11 个角色的完整动作都已可玩。

## 复跑

在已有固定 ROM、未观察内核和观察内核的当前 worktree 中运行，输出全部留在 `.local`：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_chu_world.py \
  --out .local/md-yuyuhakusho/chu-world-replay
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_chu_world.py \
  --inputs games/yuyuhakusho/engine/testdata/chu-special-motion-inputs.json \
  --end 3600 --allow-open-end --name chu-world-special \
  --out .local/md-yuyuhakusho/chu-world-special-replay
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/export_chu_world_rules.py
mkdir -p .local/md-yuyuhakusho/tmp
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -run '^TestChuWorld' ./games/yuyuhakusho/engine
GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -run '^TestChuWorld' \
  -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine
```

原版 CPU 来源、最终测试日志、实际缺实现红灯、测试先行摘要、当前来源摘要及排除项汇总在 `testdata/chu-world-validation.json`。任何摘要或原版基线不一致都应停止核验；不得用重写预期结果绕过。

最终清理只删除本任务 6 个重复／初步采样输出目录和 2 份临时验证日志，共 13,722,307 bytes；保留规范夹具、未观察基线、manifest、实现前红灯、呈现边界失败及最终日志。共享 Go 缓存、固定 ROM／工具和活动 worktree 均保留，清单见 `testdata/chu-world-cleanup.json`。
