# 原版规则核心

本目录包含固定版本 SDLPAL 独立重实现规则代码和平台适配。原版参考仓库保持只读；上游提交、文件大小与摘要见 `upstream-manifest.json`。上游许可和作者声明保留在 `upstream/`，许可正文为 `upstream/LICENSE`。

当前使用 WASI SDK 34 与 Binaryen 133 编译服务端规则核心，原版脚本、场景移动、资源加载、地形碰撞、调色板演出计时和战斗规则源码保持独立。适配层提供纯内存只读文件、确定性随机数、逻辑时钟和脚本续行；未接入的宿主函数明确报错。

原版 RNG 播放源另固定在 `reference/rngplay.c`，来源和摘要见 `reference/manifest.json`；与主上游文件使用同一提交，原始文件保持不变。构建派生代码只加入场景捕获、动画帧和等待期限观测。规则包保留帧索引及空结束记录，画面指令通过客户端无损素材加载。原版逐帧像素、计时、续档和浏览器验收的完整准备命令见[原版 RNG 演出与续档](../README.md#原版-rng-演出与续档)；运行引擎完整测试前需先生成对应 RNG 夹具及 C 参考。

## 构建与复现

先准备 `games/pal/assets/fetch.py` 固定的原版资源。工具链位于仓库 `.local/pal/toolchains/`；也可通过 `PAL_WASI_SDK` 指定 SDK 目录或 clang，通过 `PAL_WASM_OPT` 指定 Binaryen 133 的 wasm-opt。所有构建产物都位于仓库 `.local/pal/`。

```sh
. scripts/env.sh
python3 -B games/pal/build/native.py
python3 -B games/pal/build/native.py --battle-fixture 1
go run ./games/pal/build/cmd/rules
node games/pal/tests/native-opening.mjs
go test -race -v ./games/pal/engine ./games/pal/resources -count=1
```

固定上游文件已保存在 `upstream/`。更新来源时，显式运行 `python3 -B games/pal/build/native.py --vendor-from /本地/参考仓库`，该命令只读取源仓库并拒绝将脏工作区冒充为固定提交。不得省略来源校验来修复构建。

构建器对原版触发脚本入口添加调用深度观测，抽取原版视频效果与对话函数，并编译原版 `ui.c`。对话中只插入绘制观测与共同阅读的确认边界，不替换剧情指令、分页逻辑、控制码或战斗公式。原文件摘要在构建前检查，编译命令和结果摘要记录于 `.local/pal/native/build.json`。

## 对话与字体

对话使用固定来源的 `PAL_ShowDialogText`、`TEXT_DisplayText`、标题／头像位置和等待函数。逐字、颜色、四行分页、姓名标题、定时文字及原菜单选择均由原版 C 处理；共同确认对应原版等待按键的位置。开场与客栈共 72 条消息形成 23 次阅读停顿，原版自动播放的梦境文字不会额外等待全员确认。

绘制观测包含字形、调色板颜色、位置、头像和界面图块。服务端只编译字体宽度，不携带字形像素。`build/fonts.py` 按原版的 ISO、简体字形加载顺序导出客户端字体，记录大小、摘要和许可；浏览器使用原始位图和索引色绘制。

`--reference-font` 另行生成包含原版 C 字体光栅函数的验证构建，输出到 `.local/pal/native-reference/`。此构建只用于对照，不作为服务端或客户端发布载荷。以下命令在已有工具链和原版资源后执行：

```sh
. scripts/env.sh
python3 -B games/pal/build/native.py
go run ./games/pal/build/cmd/rules
go test -v ./games/pal/resources -run OriginalVisualExport -count=1
python3 -B games/pal/build/fonts.py
python3 -B games/pal/build/native.py --reference-font
node games/pal/tests/dialogue-native.mjs
PAL_REFERENCE_FONT=1 node games/pal/tests/scene-oracle.mjs
python3 -B games/pal/tests/visual-browser.py --dialogue
```

23 个阅读停顿的完整画面均与原版 C 光栅输出逐像素核对。桌面和 320px 截图、哈希记录位于 `.local/pal/dialogue-browser/`；C 参考及 Go 续行记录位于 `.local/pal/verification/`。这不代表菜单、战斗、转场中间帧或正式平台页面已经全部还原。

## 规则与图像分离

`resources.BuildRuleBundle` 保留原始脚本、角色属性、物品、法术、战斗表，以及地图的碰撞和高度。所有地图逐单元核对；人物、敌人和法术图集保留原动画帧数与尺寸。

服务端图像存储使用透明尺寸元数据，不保存原版像素；调色板保留结构和原版演出循环，实际颜色另由客户端素材提供。此数据只用于无画面的权威规则，不能用作客户端美术。客户端仍须加载并绘制原版图块、人物、界面和演出。

规则快照第 2 版同时保存线性内存和暂停时的 C 栈指针。仅复制内存不能恢复所有局部数组；必须恢复对应栈指针，且宿主读取界面或写入输入时使用独立服务栈，避免覆盖暂停的菜单数组。状态摘要覆盖内存和栈指针，恢复时另检查栈边界。旧的第 1 版仅用于早期本地探针，不能作为正式菜单存档。

Go 运行器每次在独立实例中应用候选状态，保留 C 局部变量、调用续行、资源游标、随机数和逻辑时钟。读取原始文件仅发生在初始化；回调没有宿主文件或网络接口。快照另有规则构建、资源身份、压缩大小和解压边界校验。

压缩容器使用 Zstandard，解压后为 `PALMEM01` 二进制封装：8 字节标识、两个小端 32 位长度、JSON 元数据和原始线性内存。元数据不再重复携带内存的 Base64 文本；规则快照仍为第 2 版。读取兼容同一原生构建的旧 gzip／JSON 和 gzip 二进制封装，保留压缩校验、内存与栈摘要、长度上限和多余数据拒绝。这不提供跨 Wasm 构建的存档迁移。

平台规则回调使用 `engine.Continuation` 持有一次事务的私有状态。持久化字节进入时完整解码和校验；后续推进、输入释放、公开投影与编码共用这份私有状态，避免对同一内存重复计算摘要。观测返回独立副本，不暴露原生内存。每次执行仍创建独立 Wasm 实例；失败或取消不替换事务中的旧状态，数据库事务失败也不会污染其他房间。公开的 `Snapshot` 接口仍完整校验调用方提供的内存。

```sh
. scripts/env.sh
go test -race -v ./games/pal/engine -run 'Test(OriginalOwnedContinuation|NativeSnapshotBinaryEnvelopeAndLegacyContinuation)$' -count=1 -timeout=4m
go test -race -v ./games/pal/tests -run '^TestOriginalDetachedStreamViews$' -count=1 -timeout=4m
```

`owned-continuation.json` 和 `detached-stream.json` 记录原版客栈／菜单续行、调用方隔离、失败回滚及并发实例验证。实时连接的公开投影在平台公共锁外计算，席位确认序号仍分别返回。这些正确性结果不代表[性能预算](../PERFORMANCE.md)已经达标。

`engine.Runtime.ReleaseInputs` 仅清除方向、按键和排队输入，不执行下一条规则，不增加逻辑时间，也不关闭原菜单或对话。已验证重复调用不改变结果；平台续档／重连接入时须调用此接口，避免保存前的长按在恢复后继续生效。

全员已确认的阅读停顿另存逻辑确认状态。保存、重连或接管清理实体输入时不会清除此状态，避免在原版按键等待的最后一次延迟中存档后卡住。

## 游戏菜单

`uigame.c`、`itemmenu.c`、`magicmenu.c` 和 `uibattle.c` 已联结原版核心，屏幕布局使用 `palcfg.c` 的原始默认坐标。当前实际主循环验收覆盖主菜单、人物状态、行囊子菜单和单人队伍的仙术列表；菜单中途压缩存档、全新实例恢复和输入释放保留原菜单及光标。

系统菜单交接给平台操作边界，不调用原版本地存档槽、音量配置或进程退出。当前领队可关闭此边界；保存仍走平台全体原玩家确认。公共菜单输入使用有界短按队列，按顺序消费，普通松键不覆盖已收到的短按；子菜单切换和续档建立新的输入 epoch。

```sh
. scripts/env.sh
go test -v ./games/pal/engine -run OriginalFieldMenus -count=1
go test -v ./games/pal/tests -run OriginalMenusShared -count=1 -timeout=3m
node games/pal/tests/field-menus-native.mjs
python3 -B games/pal/build/native.py --reference-font
PAL_REFERENCE_FONT=1 PAL_ORIGINAL_PIXELS=1 node games/pal/tests/menus-oracle.mjs
python3 -B games/pal/tests/visual-browser.py --menus
```

8 个场景／菜单／关闭还原画面与原版 C 光栅输出一致，结果位于 `.local/pal/menus-browser/`。这不覆盖实际装备更换、道具消耗、商店交易、已有仙术施放或战斗流程。

浏览器绘制页和平台 `Board` 复用 `web/Stage.tsx` 的居中舞台，保持 320×200 比例，在 320px 竖屏、820×320 横屏和桌面检查位置与画布身份。调整尺寸不会重建 Canvas。按用户明确要求，手机按键半透明覆盖在画面上，PC 不显示虚拟方向／确认键；控件不得占用游戏区域高度，也不得缩小画面。完整平台操作验收仍在推进。

## 方向短按与持续移动

客户端先合并键盘、触屏与手柄来源，相反方向同时按住时归零，恢复剩余方向时立即发送一次边沿。斜向输入只选择一个原版方向，不在同一边沿中混入竞争方向。短按只发送一步脉冲，不建立服务器长按状态；实际按住至少一个探索帧后，才通过有期限的方向状态续期。这样即使 HTTP 确认排队导致松开请求晚到，也不会把短按变成持续走路。

平台按顺序保留有界短按队列。原版探索移动和战斗主选项原先只读取当前长按方向，短按在下一个逻辑帧前松开时可能丢失。适配层在这两个入口使用 `wb_input_direction()`，优先消费当前原版帧的方向边沿，再回到持续方向。原版步长、碰撞、帧间隔和战斗计算保持原行为。过期队列、存档及重新接管会释放输入，不补走旧步子。

```sh
. scripts/env.sh
python3 -B games/pal/build/native.py
python3 -B games/pal/build/native.py --battle-fixture 1
node --test games/pal/tests/input-directions.test.mjs
go test -v ./games/pal/tests -run 'Original(FieldShortDirectionsReleaseAndSave|BattleShortDirectionReleasedBeforeFrame)$' -count=1 -timeout=5m
```

`field-short-directions.json` 和 `battle-short-directions.json` 位于 `.local/pal/verification/`，覆盖 1～3 席、帧前按下并松开、计时批次内只移动一步、方向排队、过期释放、存档及重新接管。平台浏览器的键盘和手机触控验证沿用[页面验收](../README.md#平台页面与覆盖式按键验收)。

服务器还记录输入的接收时间，并在每个原版续行点按当时有效的方向推进。新按下的方向不能回填之前的帧；松开也不能抹掉之前有效的移动。持续方向按有效期分段，续期不填补已经失效的间隙。离散短按超过有效期后仍会丢弃，宿主积压最多推进 250 毫秒，不补播更早的离线时间。

`AdvanceScheduled` 每批只恢复、捕获一次原生内存，在原版等待到期时提供相应输入。派生源码通过 `WB_DelayUntil` 执行原版等待宏，并在宿主暂停返回后再次轮询；普通菜单的 `SDL_Delay` 保持原有清键和轮询顺序。场景、脚本、队伍、菜单或战斗控制阶段变化后，该批次不再消费旧控制输入。接收序号与规则已应用序号分别保留，等待下一帧的输入不会提前标记为已执行。

```sh
. scripts/env.sh
go test -v ./games/pal/engine -run NativeScheduledInput -count=1 -timeout=3m
go test -v ./games/pal/tests -run 'Original(FieldInputArrivalTimeline|InputTimelineOverflowReleaseAndLegacyRestore)$' -count=1 -timeout=5m
```

结果位于 `scheduled-input-native.json`、`input-arrival-timeline.json` 和 `input-timeline-lifecycle.json`。1～3 席的 18 个场景验证晚到的按下／松开、有效期、失效后的续期、短按到达和积压上限；对照时钟、随机数、位置及剩余等待。另验证有界方向历史、队列满时释放和恢复正常移动。新增控制字段可以缺省，相同原生构建的旧格式控制状态可恢复并清除旧输入；原生二进制变化后的跨构建存档迁移仍未完成。

## 控制包有效期与网络恢复

公开投影中的 `controlAt` 是房间当前的服务端逻辑时间，单位为毫秒；它不是原生游戏时钟，也不是客户端本机时间。客户端创建 `input` 操作时复制该值，排队、重试或切换传输方式时不得重新盖上较新的时间。服务端在权限、epoch 和序号检查后，将标记缺失、来自未来或超过 500 毫秒的非中立输入登记为丢弃，不推进原版状态，也不重新授予移动有效期。`discardedSeq` 只向对应席位返回，用于清理旧按键和预测；传输 ACK 不表示按键已在规则中执行。

中立松开操作仍使用原有的权限、epoch 和递增序号检查，但不因旧时间标记而丢弃。客户端在失焦、后台、控制器取消、服务器丢弃反馈或短暂同步失败时清除未发送的物理按键，保留必要的中立松开；服务端控制时钟超过 500 毫秒未更新时也停止采样。恢复前已经清除的键盘重复事件不会重新建立长按。原名册、角色权限、剧情确认、存档和电脑策略不因连接状态而改变。

```sh
. scripts/env.sh
go test -v ./games/pal/tests -run '^TestOriginal(InputFreshnessAndUnconditionalRelease|BattleInputFreshness)$' -count=1 -timeout=4m
npm --prefix web test -- pal
```

规则结果为 `.local/pal/verification/input-freshness.json` 和 `battle-input-freshness.json`，覆盖 1～3 席、过期／缺失／未来标记、有效延迟、旧标记松开及战斗中的旧确认／方向／道具键。真实页面验证沿用 `PAL_BOARD_ENTRY=field` 房间夹具，运行 `python3 -B games/pal/tests/network-lifecycle-browser.py`。结果为 `.local/pal/network-lifecycle/`，覆盖单端上传中断后的旧包丢弃、快端操作、真实 TCP／WebSocket 断开与自动重连、真实后台标签页、持续按键及重复事件、恢复后的 320px 原生触屏，以及权威画面的逐像素收敛。上述结果不替代随机丢包、所有阶段和完整性能验收。

## 探索移动预测

原生观测导出当前帧、相机、队伍偏移、走路阶段、队员／跟随者轨迹和附近可绘制对象。碰撞投影包含半径为 8 的 289 个原版格点答案，以及旧轨迹周围 45 个额外答案；剧情脚本留下的轨迹可能不在标准格点上，不能把这些位置一律当作障碍。投影不携带脚本、随机数或原生内存。观测沿用独立服务栈，不覆盖暂停的 C 局部数组。

客户端 `field-rules.ts` 按原版 `PAL_UpdateParty`、`PAL_UpdatePartyGestures` 和 `PAL_CalcCoverTiles` 推进与绘制。当前领队的短按立即显示；连续移动保留原版 100 毫秒节奏。预测最多领先 3 步，权威状态超过 300 毫秒未推进时清理旧预测。短按以规则已应用序号确认，持续方向还核对目标帧；普通 ACK 不充当规则确认。小幅位置差在 80 毫秒内校正，剧情边界、领队和 epoch 变化清除旧预测。剧情、NPC 决策、资源与战斗结果仍由服务器决定。

```sh
. scripts/env.sh
node games/pal/tests/field-prediction-native.mjs
node games/pal/tests/snapshot-detachment-native.mjs
go test -v ./games/pal/engine -run '^TestOriginalFieldPredictionProjection$' -count=1 -timeout=3m
npm --prefix web test -- pal
```

`field-prediction-native.json` 记录实际开场到达客栈后的原版 C 移动对照。540 步覆盖正常移动、障碍、1～3 名队员、跟随者、0／3／4 帧走路方案和非格点轨迹，同时逐项核对人物与遮挡绘制顺序。这些额外队伍是明确的移动夹具，不代表剧情已经走到相应队伍。`field-prediction-go.json` 验证公开投影隔离、畸形元数据拒绝和原版存档续行；`snapshot-detachment-js.json` 验证 JS 对照夹具保存的是独立内存副本。

原生观测导出增加后，构建摘要已变化。存档仍为第 2 版，只有相同原生构建与资源身份的存档可恢复，跨构建迁移仍未实现。真实页面的探索响应及剩余范围见 [性能验收](../PERFORMANCE.md)。

## 独立战斗

独立战斗验证构建使用 `--battle-fixture 1`，输出到 `.local/pal/battle-fixtures/team-1-party-3/`，正常构建仍运行实际 `PAL_GameMain`。夹具仅选择原版敌队和原版队员，不修改血量、仙术、伤害、道具或奖励，不作为剧情进度和正式发布构建。`--fixture-party` 可限定原版队伍人数，`--fixture-roles 0,2` 可明确选择原版队员并确定队伍人数；这与平台的 1～3 个控制席位分开。

```sh
. scripts/env.sh
python3 -B games/pal/build/native.py --battle-fixture 1
python3 -B games/pal/build/native.py --battle-fixture 1 --fixture-roles 0,2
PAL_NATIVE_WASM=.local/pal/battle-fixtures/team-1-party-3/rules.wasm PAL_TEST_PLAYERS=1 node games/pal/tests/native-battle.mjs
PAL_NATIVE_WASM=.local/pal/battle-fixtures/team-1-party-3/rules.wasm PAL_TEST_PLAYERS=2 node games/pal/tests/native-battle.mjs
PAL_NATIVE_WASM=.local/pal/battle-fixtures/team-1-party-3/rules.wasm PAL_TEST_PLAYERS=3 node games/pal/tests/native-battle.mjs
go test -v ./games/pal/engine -run OriginalBattle -count=1 -timeout=3m
go test -v ./games/pal/tests -run OriginalBattleRole -count=1 -timeout=5m
PAL_NATIVE_WASM=.local/pal/battle-fixtures/team-1-party-3/rules.wasm node games/pal/tests/native-coop.mjs
go test -v ./games/pal/tests -run OriginalCooperativeMagic -count=1 -timeout=4m
go test -race -v ./internal/server -run PALOriginalCooperativeConsentHTTP -count=1 -timeout=4m
```

验证使用真实 `PAL_StartBattle`，覆盖攻击、防御、原版快捷操作、原始角色／敌人属性、40 毫秒战斗时钟、实际胜利奖励和新实例续档。多人权限以当前队员的控制者为准；角色／选招阶段变化清除旧方向、短按队列和 epoch。同一计时批次内切换角色也会释放旧输入。原版重复、强制选招、逃跑快捷键仅作用于当前角色；多人全队自动攻击及跨队员撤回被拦截，单人保留原操作。敌人血量仅用于权威诊断，不发到公共游戏投影。

合击通过原版 `PAL_BattleCommitAction` 的共同确认边界，尚未同意前不提交指令、不消耗体力、不推进原版时钟。发起人及已同意者仍可取消；所有受影响队员的控制者均可操作，只有未同意者收到行动提醒。受影响队员包含原版会跳过其本轮选招的非体力贡献者；没有在队角色的席位不参与确认。单人保留原版操作，不增加多人确认。

部分确认、选中目标与暂停中的 C 提交调用随存档保存。批准后在全新原生实例执行，与原版单人合击逐步核对血量、伤害、随机数、时钟及奖励。真实 HTTP／SQLite 验证还覆盖失败回滚、发起人取消、保存退出、全新服务器恢复、原名册及旧页面拒绝。

记录位于 `.local/pal/verification/native-battle-*.json`、`battle-cooperation-*.json`、`native-coop-js.json`、`coop-consent-*.json` 和 `coop-unassigned-seat.json`。合击玩家界面另有 `.local/pal/battle-board-browser/` 的真实平台验收。战斗图像验证见下节；战斗内道具／其他施法、头目及完整剧情尚须验收。

## 战斗绘制与缓冲

`raster.c` 将原版离屏绘制保留为不可变的缓冲依赖图，节点只包含资源编号、绘制参数及其他缓冲版本，不包含像素。复制后修改源缓冲不会改变旧备份；有限的节点池会回收所有活跃缓冲不再引用的节点。导出只遍历当前画面的依赖，不发送私有地址、战斗随机数或整段原生内存。

客户端使用原图集与字体重放调色板索引操作，包括普通、阴影、色阶移动、单色、背景色阶、原版逐行波动和交错渐变。人物、敌人、FIRE 法术与 DATA#10 物理特效均以原版实际绘制调用的顺序、位置及帧号呈现。图像缓冲释放时同步清除原生资源指针登记，已保存的绘制节点继续使用不可变资源编号。

```sh
. scripts/env.sh
python3 -B games/pal/build/native.py --battle-fixture 0 --reference-font
python3 -B games/pal/build/native.py --battle-fixture 1 --reference-font
python3 -B games/pal/build/native.py --battle-fixture 0
python3 -B games/pal/build/native.py --battle-fixture 1
python3 -B games/pal/build/native.py --battle-fixture 0 --fixture-field 18 --reference-font
python3 -B games/pal/build/native.py --battle-fixture 0 --fixture-field 18
PAL_NATIVE_WASM=.local/pal/battle-fixtures/team-0-party-3-reference/rules.wasm PAL_REFERENCE_FONT=1 PAL_ORIGINAL_PIXELS=1 PAL_BATTLE_CASE=attack node games/pal/tests/battle-oracle.mjs
PAL_NATIVE_WASM=.local/pal/battle-fixtures/team-1-party-3-reference/rules.wasm PAL_REFERENCE_FONT=1 PAL_ORIGINAL_PIXELS=1 PAL_BATTLE_CASE=coop node games/pal/tests/battle-oracle.mjs
PAL_NATIVE_WASM=.local/pal/battle-fixtures/team-0-party-3-field-18-reference/rules.wasm PAL_REFERENCE_FONT=1 PAL_ORIGINAL_PIXELS=1 PAL_BATTLE_CASE=wave node games/pal/tests/battle-oracle.mjs
PAL_NATIVE_WASM=.local/pal/battle-fixtures/team-0-party-3/rules.wasm PAL_BATTLE_CASE=attack node games/pal/tests/battle-oracle.mjs
PAL_NATIVE_WASM=.local/pal/battle-fixtures/team-1-party-3/rules.wasm PAL_BATTLE_CASE=coop node games/pal/tests/battle-oracle.mjs
PAL_NATIVE_WASM=.local/pal/battle-fixtures/team-0-party-3-field-18/rules.wasm PAL_BATTLE_CASE=wave node games/pal/tests/battle-oracle.mjs
PAL_NATIVE_WASM=.local/pal/battle-fixtures/team-1-party-3/rules.wasm node games/pal/tests/raster-continuation.mjs
go test -v ./games/pal/engine -run OriginalCombatRaster -count=1 -timeout=3m
python3 -B games/pal/tests/visual-browser.py --battle attack
python3 -B games/pal/tests/visual-browser.py --battle coop
python3 -B games/pal/tests/visual-browser.py --battle wave
```

三组独立原版战斗共 278 个变化画面在浏览器与原版 C 之间逐像素一致。普通攻击与合击沿用初始背景编号 0；波动验证显式选择原版脚本使用的第 18 号背景，保留该背景原有波动及元素效果。固定像素哈希保存在 `resources/testdata/battle-*-raster.json`，添加绘制观测不会改变原版 C 的画面序列。`PAL_RECORD_BASELINE=1` 只用于首次记录经核对的原版参考，日常验收不得用它覆盖差异。

当前画面基于原版逻辑屏幕。正式页面、入场物理屏幕的调色板／交错切换、其他法术、头目和完整剧情还须验收；完整绘制图投影也须改为正式传输中的增量更新。独立战斗像素验证不代表上述工作完成。

## 验收边界

当前验证了原版开场消息、梦境转场、完整起床对话、恢复正常人物形象、客栈自由移动、内存存档和独立实例续行。结果位于 `.local/pal/verification/native-opening.json`、`native-go-opening.json`、`native-inn.json` 和 `rule-bundle.json`。独立战斗及绘制的记录见上节。

尚未完成装备／道具／商店的完整操作验收、完整战斗和多人操作界面、音视频输出、剧情聚散、电脑完整流程和实际平台游戏入口。对话、部分菜单和独立战斗的验收不能替代这些功能，也不能视为完整游戏通关。
