# 原版追逐搜索的辅助对象别名

## 实现前失败场景

- `$B100` 排除不可搜索动作后，Y 仍是动作编号；下一个候选为该动作编号加一，不能擅自改为下一个对手席位。
- 自然主动场景中对手动作 `0x1E` 导致候选 31。其读取不是第 31 个物理对象：动作来自辅助对象 5 的下一动作，深度来自该辅助对象高度的整数低字节，X 来自其深度固定点的两个字节。
- 动作进入标志和下一动作无效值必须按原字节还原；无下一动作对应 `0xFF`，不能作为待机 0 处理。
- 先判断该动作的可搜索位，再判断坐标窗口；若坐标窗口不符，继续原共享球追逐，不生成新的替代策略。
- 别名上下文缺失或其他未恢复索引应明确报错，不能忽略未知候选，也不能越界访问 `Players`。
- 用完整原输入阶段 before／after 比较状态及共享随机流，保留全部前后状态。连续场景通过此处后出现的新首差另行报告。

## 证据

原主动场景 `world-rider-natural` 的输入阶段第 22 条，frame 3574。复用 REA `$B100` 完整指令证据 `ev_7920cb1ab0e365ea26eb1a2a9c199765e279f2a6fe27395c1e2c17b2b2007aae`，文件 `.local/basketball/rea/ai-batch/function-0xb100.json`。只读原 RAM 核对 `$0531/$0094/$00A1/$00BB` 的别名值；不引入内存解释器或运行时 ROM 访问。

## 结果与限制

完整原输入阶段精确匹配，别名读取也与原 RAM 字节独立对照。`AIControlView` 使用可选辅助对象上下文，统一世界输入传入实时辅助对象；兼容单人物入口同样可显式提供。未提供上下文时仍报错，其他未恢复索引没有被忽略。

原 1,142 条持球交接／追逐记录、全部 1,998 条输入阶段，以及单初态连续 1,998 次核心更新仍通过；原生竞态与 Wasm 记录位于 `.local/basketball/rea/pursuit-alias/{native-tests.json,wasm-tests.log}`。原始来源、完整字段与未压缩／压缩 SHA-256 保存于 `pursuit-alias-input-source.json`。

当时主动连续场景通过此输入分支后，在同一 update 22 的后续动作阶段停止：人物 0 动作为 `0x1E` 且依附标志为 `0x81`，现动作入口尚未恢复该附着接球处理。已保留首差 `next-continuous-boundary.log` 并交主线程；本修改没有绕过该动作，也未宣称主动场景完整通过。

## 索引 74：实现前失败场景

- 原完整输入第 54 条 / frame 3670，人物 3 排除动作 0x49 后实际读取索引 74，不能跳过。
- 动作来自 $055C，深度来自 $00E6；两字节尚无确定游戏语义，使用显式可选 PursuitAlias74，缺失报错，与已知零值分开。
- X 的两个整数位来自 $00BF/$00CC，即篮筐对象 9 的高度整数；必须使用当前篮筐高度而非初始常数。
- 非零上下文、动作高位掩码、深度差无符号回绕、X 有符号窗口均要验证，不能把观察到的零值硬编码。
- 循环配置只从初始 RAM 读取一次，不从后续期望快照注入。写观察覆盖 6,000 帧 CPU 与 180 帧主动场景，没有观察到 $055C/$00E6 写入；这不是全游戏不变证明。
- 原完整输入阶段 before/after 必须匹配；57 次连续更新如出现其他首差，保留并报告。

## 索引 74 的证据与验收

沿用同一 REA `$B100` 完整指令证据 `ev_7920cb1ab0e365ea26eb1a2a9c199765e279f2a6fe27395c1e2c17b2b2007aae`。`$B10F/$B111` 将屏蔽动作进入位后的动作留在 Y，`$B13F` 直接递增，故拒绝 `0x49` 后读取 74。原地址分别为 `$0512+74=$055C`、`$0075+74=$00BF`、`$0082+74=$00CC`、`$009C+74=$00E6`。

只读运行观察脚本为 `scripts/basketball/capture_pursuit_alias74.py`，监听写入及 `$B10A` 执行，不修改 RAM；完整 RAM 与逐帧 PNG 和既有原运行一致。结果在 `.local/basketball/rea/pursuit-alias74/write-observation.json`：6,000 帧 CPU 场景无这两字节写入，180 帧主动场景也无写入；主动场景实际在 frame 3670、3673、3676、3679 探测索引 74。未知字段不被命名为其他游戏资源，也不声称所有未观察路径都不写它们。

`PursuitAlias74` 是可选的显式兼容上下文，循环配置只读首个 RAM 状态一次，缺失时遇到该候选仍报错。当前 X 每次从篮筐对象 9 的高度整数读取。完整输入第 54 条的 before/after 和原字节 oracle 保存于 `pursuit-alias74-input.json.gz`；来源、原始 trace 和压缩／未压缩摘要在相邻 `-source.json`。

测试先于实现，初次编译失败记录为 `test-before.log`。完成后原生竞态和 Wasm 均通过原追逐／持球测试、1,998 个完整输入阶段、1,998 次单初态连续更新及 57 次主动完整更新；非零上下文字节、动作掩码、无符号深度／有符号 X 回绕另外验证。记录位于 `.local/basketball/rea/pursuit-alias74/{native-tests.json,wasm-tests.log,continuous-tests.log}`。此前 update 22 的附着动作边界已由对应动作恢复工作解决，本次只增加 alias74 兼容字段和接线，不改动作行为。

复跑：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/extract_pursuit_alias.py --index 54
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/extract_world_loop.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/extract_world_loop.py --source-run world-rider-natural --output world-loop-rider --complete-prefix
source scripts/env.sh
go test -race ./games/basketball/engine -run 'TestOriginal(Pursuit|AIPossession|Possession)|TestWorld(Inputs|Loop)' -count=1
GOOS=js GOARCH=wasm go test -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/basketball/engine -run 'TestOriginal(Pursuit|AIPossession|Possession)|TestWorld(Inputs|Loop)' -count=1
```

57 次完成更新之外的捕获尾部完整保留，未计为完成更新；通过本段也不代表整个游戏已经复刻完成。

## 冷配置索引 73 与惰性读取：实现前失败场景

- 固定冷配置 update411 的原搜索拒绝人物动作 0x48 后查询索引73；不得修正为下一玩家或跳过别名。
- 原 `$B100` 先读取并判断动作 flags，再读深度，深度窗口通过后才读 X。对于已拒绝动作，不应提前索取原本未读的未知坐标字节。
- 动作73来自055B、深度73来自00E5；自然样本另出现索引75，其动作来自SelectedHoop，深度来自00E7。未知业务语义的三个字节由显式可选初始上下文保存，不硬编码0。
- 其他拒绝动作+1候选的动作来源统一对应已恢复的请求、下一动作、控制席位、篮筐所有者、闪避X、选中篮筐、投篮起点、排除接球者和骑乘者字段；不能把全部候选位置都当Players数组。
- 79／87的动作常为FF屏蔽后127，必须可在flags拒绝时完成，不要求未使用坐标；若给非零可搜索动作而需未知坐标，仍明确报错。
- 采样所有B100调用及实际depth/X分支，保留完整前态和成功目标，不筛掉新增别名。新字段只从原初态读取一次，未观察到写不等于证明全游戏不变。

## 惰性读取的 canonical 来源表

继续使用完整 REA `$B100` 证据 `ev_7920cb1ab0e365ea26eb1a2a9c199765e279f2a6fe27395c1e2c17b2b2007aae`。搜索现在按原指令顺序依次读取动作、深度、X，各层都可以在拒绝后停止。运行时没有地址解释器，也不对未知位置填零。

| 候选索引 | 原动作读取地址 | 已恢复的类型字段 |
|---|---|---|
| 7 | 0519 | Hoops[0].Action |
| 14 | 0520 | Actors[1].Player.Request.Action |
| 23 | 0529 | Hoops[3].Request.Action |
| 31 | 0531 | Auxiliary[1].Animation.NextAction；无下一动作对应 FF |
| 39／40 | 0539／053A | Controls[0／1] |
| 48 | 0542 | Hoops[2].Owner |
| 55 | 0549 | Actors[1].Dodge.InitialXHigh |
| 73 | 055B | PursuitReserved.Action73，显式未知用途字节 |
| 74 | 055C | 既有 PursuitAlias74.ActionByte |
| 75 | 055D | SelectedHoop |
| 76／77 | 055E／055F | ShotOriginX 整数低／高字节 |
| 79 | 0561 | ExcludedCatcher |
| 87 | 0569 | CarriedPlayer |
| 88 | 056A | PoseSource（真实共享姿态／跟随源） |

索引 73 的深度来自 E5，75 来自 E7，分别由显式 `PursuitReserved.Depth73/Depth75` 提供；其 X 分别是 Hoops[1]/Hoops[3] 高度整数。74 沿用既有显式字节。索引 14／23、39／40、48 的位置从对应对象的 X 高位、Depth 和 Height 分量按原字节关系构造，而非当作玩家编号。索引 55 的 X 来自玩家 3 高度固定点字节，但深度 D3 尚未恢复；其他尚未明确的地址同样保持错误。

例如 79 和 87 的原动作字节为 FF 时，屏蔽后 127 的 flags 已拒绝候选，因此不读取其未知坐标；若给可搜索动作，真的需要未知深度时仍明确报错。71、78、80 等尚无完整语义来源的候选不增加常量或默认跳过。

## 原自然调用与结果

只读脚本 `scripts/basketball/capture_pursuit_search.py` 录制三个原运行（6000／18000／180 视频帧）中的所有完整 `$B100` 调用，保留入口完整世界状态、每次候选以及是否实际走到读深度／读 X 分支。1112 次调用全部参与对照，候选计数为 `0:1112、1:1025、31:9、73:20、74:4、75:5`，目标结果和真正读取过的值逐项匹配。三个运行的所有 RAM／PNG 与既有参考完全一致；055B／E5／E7 写观察均为 0 次。这仅界定当前观察范围，不证明任何未观察路径都不写它们。

`AIControlView` 使用临时只读 canonical 世界／控制配置指针，不新增一份持久状态。新的 `WorldLoopContext.PursuitReserved` 可选字段从首次 RAM 提取一次，并断言保留阶段的前后值未变；冷开球按原 C9B6/F8BD/FA8C 清零路径显式初始化。后续期望快照不作为输入。该阶段两份 loop fixture 的 `config.pursuitReserved` 含 `action73/depth73/depth75`；索引88恢复后另增加显式 `depth88/xHigh88`。

测试先于实现，原生竞态与 Wasm 均通过新 1112 个搜索调用、原 1142 个持球／追逐调用、1998 个完整输入阶段、1998 次连续世界更新及 57 次主动完整更新。日志位于 `.local/basketball/rea/pursuit-lazy/{test-before.log,native-tests.json,wasm-tests.log}`。固定冷配置推进至 update467，下一边界为人物 1 的 dodge 请求转换；完整前态保存为同目录 `cold-next.json`，已交动作恢复工作，6000 目标未降低。

```sh
PYTHONDONTWRITEBYTECODE=1 .local/monopoly/venv/bin/python scripts/basketball/capture_pursuit_search.py
source scripts/env.sh
go test -race ./games/basketball/engine -run 'TestPursuit|TestOriginal(Pursuit|AIPossession)|TestWorld(Loop|Inputs)' -count=1
GOOS=js GOARCH=wasm go test -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/basketball/engine -run 'TestPursuit|TestOriginal(Pursuit|AIPossession)|TestWorld(Loop|Inputs)' -count=1
```

## 全电脑视频索引88：实现前失败场景

- 新独立全电脑完整视频测试在video557／logic183查询88。动作读取056A，必须使用真实共享姿态/跟随源PoseSource；当前Holder和LastTouch不能替代。
- 深度来自F4；X低字节来自CD（Hoops[3]高度整数高字节），X高字节来自DA（D3..E2列表槽7）。仍按动作flags→深度→X顺序读取，不为已拒绝动作提前索取这些字节。
- F4由C9B6→FA8C冷清零；DA由原bank3 A0D3..A0DF列表初始化填FF。通过隔离原初始化段验证，不把观察到的常量直接写死到搜索函数。
- depth88/xHigh88是可选字节指针：旧JSON未提供时保持未知，真正读取才报错；冷初始化显式提供0/255，原trace fixture从首RAM提供并断言稳定。
- 原B100隔离矩阵覆盖非零PoseSource、掩码、深度拒绝、X读取、可选字段缺失及可能命中；不得直接默认跳过索引88。
- 6000、18000、24000帧原参考均监听DA/F4写入且保留完整RAM/PNG一致性；没有写入只证明这个范围，不声称任何菜单或存档都不变。

## 索引88完成结果

动作源读取已经对接由原动作路径维护的 `WorldActionState.PoseSource`；AI不推测或回写它。两份原loop夹具已由负责PoseSource的恢复工作从原RAM重生成，并通过7份fixture旧字段不变审计（`.local/basketball/rea/pose-source/fixture-migration-audit.json`）。

`Depth88`／`XHigh88` 使用可选 `*uint8`，JSON仍是数字。旧上下文未提供时保持nil，只有原路径真的读取才报错。冷初始化的0／255经原FA8C和bank3 A0D3..A0DF初始化段隔离执行确认；fixture配置从首RAM提取，后续原记录只作稳定断言。verify-world的严格配置已经直接使用engine类型，无需另写字段镜像。

14112组原B100隔离执行全部匹配，包含非零源字节、被拒绝的动作、不同深度、X高字节、合法命中和实际读取门槛；相应缺字段负例证明没有提前索要无用数据。另对6000、18000、24000帧参考做只读写观察：DA/F4未写，初值均0/255；1774次完整B100调用中没有自然命中索引88，因此不把这部分观察称为alias88自然调用验证。完整RAM／PNG一致，原始记录保留。

最终全电脑视频native、Wasm各两次完整比赛通过，race覆盖别名、原混合首节/完整及全电脑完整；1998原连续核心和57主动核心通过严格verify-world CLI及Wasm。日志位于`.local/basketball/rea/pursuit-alias88/`，首次全电脑视频失败已从并行恢复的真实测试日志提取到`.local/basketball/video-runtime-all-computer/first-alias88-failure.json`，附原日志SHA。

```sh
PYTHONDONTWRITEBYTECODE=1 .local/monopoly/venv/bin/python scripts/basketball/capture_pursuit_alias88.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_pursuit_alias88.py
source scripts/env.sh
go test ./games/basketball/engine -run TestPursuitAlias88 -count=1
GOOS=js GOARCH=wasm go test -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/basketball/engine -run 'TestPursuitAlias88|TestWorldLoop' -count=1
```
