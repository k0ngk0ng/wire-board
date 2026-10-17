# Court 6 独立运动与老虎机机关

Court 6 的独立运动、原进球触发和老虎机机关已完成 Native、race、Wasm 验证，随后已接入运行时与开场／转场。正常时钟四种控制配置和新增 16 种准备配置的结果见后文；平台选项及浏览器由根任务另行验收，本子任务不修改入口。

## 原入口与权威状态

原场地编号为 `$058B = 6`，场景／投影布局仍是 Scene 1。地形表有 9 行，纵深排他上限为 72；地面设备分派 `$8458` 在本场地进入空函数 `$84D6`。老虎机由实际进球触发，不由地面碰撞启动。

原篮筐接触入口是 bank 4 `$8E33`；进球时把 `$06DD/$06DE` 中 `scoringSide ^ 1` 对应字节设为 1。动画尾入口 `$9B95` 在非空闲／非暂停且 `$0300` 为奇数时分派到 Court 6 `$A124`，推进滚轮、共享随机数、奖励、图块及调色板。

| 原地址 | 语义 | 当前或拟用的唯一权威字段 |
| --- | --- | --- |
| `$06D7..$06DA` | 前四个滚轮；高位表示停止 | `Preparation.Actions.Actors[0..3].Player.NewYorkPassage` |
| `$06DB/$06DC` | 后两个滚轮 | `Preparation.Actions.Auxiliary[0..1].NewYorkPassage` |
| `$06DD/$06DE` | 左右老虎机进度及进球启动标记 | 已有 `Preparation.CourtScoreFlags` |
| `$06DF` | 奖励状态高位 | `Preparation.CourtOutcomeFlags`，JSON 零值省略 |
| `$0048..$004B` | 共享随机数 | `Preparation.Actions.Random` |
| `$0166..$0169` | 两队比分低／高字节 | `Preparation.Scores` |
| `$0540..$0545` | 篮筐原 owner／安装编号 | `Preparation.Actions.Hoops[].Owner` |
| `$062D..$0632` | 篮筐得分锁 | `Preparation.Actions.Hoops[].ScoreLock` |
| `$0300`、`$0397` | 逻辑帧低字节、已显示相机横坐标 | `World.LogicFrame`、`World.Camera.DisplayedX` |

`CourtAnimationState` 只用于一次调用的投影，不新增第二份持久化滚轮、分数或 RNG。旧 `NewYorkPassage` 字段在其他场地是原共享字节的历史名称，不能据其非零值执行纽约通道逻辑。

## 运动绑定

先记录纵深边界错误、共享滚轮字节被当作通道、错误调用其他场地设备、辅助对象拒绝绑定及其他对象写入丢失等失败场景，再增加 `court_six_binding_test.go`。首失败保存在 `.local/basketball/rea/court-six/binding-before.log`。

`verify_court_six_motion.py` 对全部 13 个对象各执行 160 次原 `$EEFF`，共 2,080 次，覆盖横向边界、纵深 0／1／70／71、持有状态、纵深移动与下落。完整运动状态、共享标记、绑定提交和地形缓存均匹配；设备音效请求为 0，与 `$84D6` 空入口一致。

生产修改仅为 `court_motion_binding.go` 允许独立绑定 Court 6，以及 `auxiliary_rest.go` 在具有完整原运动绑定时允许 Court 6。兼容 API 缺少原场地绑定时仍报错，不能退回纽约地形。

## 原版普通输入与真实奖励

原参考从既有存档开始，仅在第一次 `$D1B5` 选择 Court 6，之后通过控制器输入完成原菜单与中场确认。没有改分数、滚轮、随机数或比赛时钟。26,000 视频帧参考保存在 `.local/nes-basket/runs/court-six-slot-complete-reference/`；它不是从原菜单自然选择 Court 6 的证明。

被动观测 `court-six-slot-observed-v2` 的全部 RAM 和 6 张 PNG 与参考逐字节一致，记录：

- 14 次真实进球，正确设置原 `$06DD/$06DE` 标记。
- 7,244 次老虎机原调用，165 次共享 RNG 变化。
- 一次真实奖励：参考第 22,172 视频帧，比分从 12∶26 变为 3∶26，音效顺序为滚轮停止 14、扣分 9，奖励标记置位。中奖标记持续 36 个原视频帧。

长捕获的首版提取器在原函数返回后读取图块队列，遇到 NMI 已把已提交队列的高地址字节改为 `FF`，因此报错；失败保留在 `slot-observed-source.log`。第二版被动记录实际队列头写入，593 个被 NMI 清除的队列头从该次已观察写入恢复，仅用于提取图块命令。原 RAM 不被写入，完整轨迹和截图仍一致。没有根据期望图像猜地址或改写原图块。

源夹具为 `court-six-slot-observed.json.gz`、`court-six-slot-goals.ram.gz`，各自 SHA-256 与参考身份见 `court-six-slot-observed-source.json`。

## 从进球到老虎机结束的连续原函数链

`verify_court_six_slot_chain.py` 分别使用上述 14 个真实进球入口的原 RAM，一次执行 `$8E33`，然后连续执行 `$9B95`，仅推进外部逻辑计数器 `$0300`。不在调用之间改写 RNG、滚轮、进度或分数。14 条链共执行 367 次老虎机调用，其中一条再次得到 12∶26 → 3∶26 的减 9 奖励，最终均清除进度和奖励标记。

这是隔离机关链，省略了世界其他行动，不能称为完整原比赛回放。测试比较每次调用的滚轮、RNG、分数、图块、调色板及音效，并在每个边界比较 JSON 恢复后的结果。原目标状态从首个进球状态持续推进，不向 Go 注入中间期望状态。

初次独立驱动错误地把真实调用者的返回位置 `$FCE9` 当作人为调用栈的停止地址，保留失败 `slot-chain-source.log`；改为对应本驱动压栈的 `$7000` 返回哨兵后完成，未忽略未知设备写入。源夹具及 SHA-256 见 `court-six-slot-chains-source.json`。

## 最小运行时接线方案

1. 新增一个保存 `$06DF` 的 `Preparation.CourtOutcomeFlags` 字段。开场从 `Transition.Passages[8]` 导入，转场写回该槽；跨节以实际重新初始化结果覆盖。已有 `CourtScoreFlags` 同样从 `Passages[6:8]` 导入，不能隐含永远为 0。
2. 在 `observeRuntimeCourtAnimation` 为 Court 6 读取上述六个共享滚轮、进度及奖励字节，并提供当前篮筐 owner／得分锁、RNG 和比分。成功后一次性写回这些相同字段，保留图块／调色板队列的下一视频提交顺序。
3. 动画位于原 D352 尾部。奖励改变权威比分后，由下一轮原 HUD 阶段显示，不能额外提前刷新 HUD。音效保持原 13／14／9／46，调色板为 7 与 11～15；已存在 Court 6 调色板验证范围。
4. 由根任务协调开场、运行时、准备、世界动作等门禁，并在正常种子和时钟下完成四种控制配置及准备阶段增量配置。实际 Go 完整赛须观察至少一次进球启动及奖励；无触发完赛不能替代机关验收。
5. 最后验证浏览器滚轮、奖励比分、音效、调色板提交、暂停、刷新与恢复，再决定平台准备选项范围。独立图像素材或本文件不构成开放依据。

## 复跑与结果

```sh
source scripts/env.sh
python3 -B scripts/basketball/verify_court_six_motion.py
.local/board-art-env/bin/python -B scripts/basketball/capture_court_six_slots.py
.local/board-art-env/bin/python -B scripts/basketball/verify_court_six_slot_chain.py
go test -race ./games/basketball/engine -run '^(TestCourtSix.*|TestCourtAnimationInactiveVideoAndInvalidCourt)$' -count=1 -v
GOOS=js GOARCH=wasm go test ./games/basketball/engine -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" -run '^(TestCourtSix.*|TestCourtAnimationInactiveVideoAndInvalidCourt)$' -count=1 -v
```

终态日志位于 `.local/basketball/rea/court-six/independent-{native,race,wasm}.log`，三者均 `PASS`。未重跑 Court 5 完整比赛或其他游戏。


## 运行时接线与正常比赛增量验收

根任务已按前述映射完成接线：唯一的 `CourtOutcomeFlags` 保存 `$06DF`；六个滚轮仍复用人物／道具字节；进球触发、动画进度复用 `CourtScoreFlags`。开场、转场及跨节分别导入、导出和采用原重置结果。原 7,244 次老虎机调用经过实际运行时适配器逐项验证，包含图块队列、调色板下一视频提交、HUD 延后更新及 JSON 恢复；Court 7 仍被门禁拒绝。

四种默认控制配置保留种子 `0x12345678`、两节原时钟及中场确认，每配置冷启动两遍：

| 配置 | 视频帧 | 逻辑更新 | 每次恢复 | 比分 |
| --- | ---: | ---: | ---: | --- |
| 默认单人 | 22,409 | 7,441 | 84 | 45∶30 |
| 默认全电脑 | 22,517 | 7,477 | 84 | 46∶31 |
| 双人合作 | 22,796 | 7,570 | 85 | 0∶73 |
| 双人对战 | 22,430 | 7,448 | 84 | 66∶9 |

Native 日志为 `.local/basketball/engine/court-six-runtime-full.log`，race／Wasm 日志为 `.local/basketball/rea/court-six/runtime-{race,wasm}.log`。后两者还覆盖旧开场、原转场、比分保留、显示提交与恢复、1,998 次连续世界更新及事务回滚，均已取得终态 `PASS`：race 371.911 秒、Wasm 109.936 秒。

准备矩阵仅扩展 Court 6 的四种球员排序与四种控制配置，`preparation-incremental-native.log` 中全部 16 个配置首轮通过，每个重复两遍。没有重跑前 96 个已验收配置、Court 5 完整赛或其他游戏。准备矩阵固定 Mode 128；上表默认单人／全电脑仍用原默认 Mode 0，不能混淆这两个配置范围。

## Go 整局的真实老虎机奖励

只读程序 `.local/basketball/rea/court-six/observe-runtime.go` 从同样的冷启动配置推进完整比赛，只输入原中场 Start；不改变 RNG、比分、道具、滚轮、状态或时钟。`natural-{solo,all-computer,cooperative,versus}.json` 及各自 `events.jsonl` 保存实际老虎机进度、滚轮、奖励标记、比分、音效、HUD 与调色板事件。它证明 Go 整局行为，不替代前述原版证据。

| 配置 | 观察到的首次奖励视频帧 | 奖励前后比分 | 整局奖励次数 |
| --- | ---: | --- | ---: |
| 默认单人 | 795 | 3∶0 → 9∶0 | 3 |
| 默认全电脑 | 12,672 | 28∶20 → 36∶20 | 1 |
| 双人合作 | 8,613 | 0∶23 → 0∶24 | 1 |
| 双人对战 | 513 | 3∶0 → 9∶0 | 2 |

四个配置都自然启动老虎机、获得奖励，并实际提交奖励调色板 11／12。适合快速浏览器验证的默认单人配置是 `Court=6, Mode=0, Controllers=[0,-1,-1,-1], Seed=0x12345678`：第 795 视频帧权威分数从 3∶0 变为 9∶0，音效为 `[14,46]`；该帧 HUD 仍为 3∶0，已显示调色板仍为 14、待提交调色板为 12，第 796 视频帧才提交 12，原 HUD 分阶段调度在第 810 视频帧显示 9∶0。服务器固定使用 Mode 128，因此额外以同一阵容、种子冷跑 `solo platform`：完整赛及上述 795／796／810 帧事件也已实测一致，独立保存为 `natural-solo-platform.json` 与事件流。浏览器验收应观察同一自然过程，不能载入人为中奖状态替代。

复跑新增准备配置与只读观察：

```sh
source scripts/env.sh
BASKETBALL_PREPARATION_MATRIX=1 go test ./games/basketball/engine -run '^TestRuntimePreparedConfigurationsCompleteMatchAndRestore$/^court6-' -parallel 4 -count=1 -v
go run .local/basketball/rea/court-six/observe-runtime.go solo
go run .local/basketball/rea/court-six/observe-runtime.go all-computer
go run .local/basketball/rea/court-six/observe-runtime.go cooperative
go run .local/basketball/rea/court-six/observe-runtime.go versus
go run .local/basketball/rea/court-six/observe-runtime.go solo platform
```

验收汇总为 `.local/basketball/rea/court-six/runtime-summary.json`，同目录 `summarize-runtime.py` 从现有日志和观察产物重新生成，保留各日志／事件文件 SHA-256、完整赛轨迹哈希及具体配置。
