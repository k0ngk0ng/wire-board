# 在同一仓库中开发游戏

每款游戏在 `games/<id>/` 中维护。仓库继续使用一个 Go module、一个前端依赖清单、一个服务和一套 SQLite 数据库。新增游戏只需要修改自己的目录，以及后端的一个注册入口。

```text
games/<id>/
  server/                 # 独立 Go 包：规则、状态、动作、机器人、存档编解码
  web/
    module.tsx            # 前端自动发现入口
    types.ts              # 本游戏的视图和选项类型
    ...                   # 棋盘、规则说明、选项编辑和样式
  tests/                  # 前端测试、HTTP／浏览器验收和夹具
  assets/public/<id>/     # 可公开、随程序发布的素材
  README.md
```

服务器专属的数据文件和 `go:embed` 资源放在 `server/` 内。公开素材由 Vite 汇集到仓库 `.local/web-public`，构建后继续进入 `web/dist`；同名公开路径会导致构建失败。修改公开素材后重新启动 Vite 或重新构建。商业素材仍遵循现有 CDN 和来源声明约定。

需要浏览器 Go/Wasm 运行时的游戏在自己的 `build/` 目录维护构建器，由应用构建入口显式接入；公共素材汇集器只接收其已经生成的公开目录。热血篮球的运行时构建与验收见 [构建说明](../games/basketball/build/README.md)。运行时代码与对应 Go 支持脚本随应用同源发布，图片和声音继续使用素材发布流程；main 编译、校验并保存应用产物，tag 不重新构建运行时。

## 新游戏接入

先列出失败场景并编写 E2E，再实现游戏。复杂功能以完整链路验收为主；小型纯函数和低风险改动可以针对性验证。禁止先实现代码再补单元测试。

1. 在 `games/<id>/server` 中实现自己的类型和导出的 `Module() gameapi.Module`。接入契约位于 [internal/gameapi/module.go](../internal/gameapi/module.go)，不需要导入 `internal/game` 或 `internal/server`。
2. 普通模块在 [games/registry.go](../games/registry.go) 中导入 Go 包，并将其 `Module()` 加入 `Modules()` 返回值。需要校验原生运行产物、可能初始化失败的模块在 [internal/production/modules.go](../internal/production/modules.go) 中构造；初始化错误必须在数据库创建和服务监听前返回，不得静默省略模块。游戏之间不互相初始化。
3. 在 `games/<id>/web/module.tsx` 中默认导出 `defineGame<本游戏视图, 本游戏选项>({...})`。前端自动发现该路径，ID 必须与后端一致。`legacy` 保持未设置。
4. 在游戏目录内编写自己的测试、玩法说明和素材声明。游戏之间不得直接导入实现，不得修改另一款游戏的源码来完成接入。
5. 执行本页的构建、边界和接入验收；新游戏应另有自己的完整对局 E2E。
6. 新注册的游戏默认上架，玩家大厅和创建列表会立即显示并允许创建；管理员可在「管理后台 → 游戏上架」显式下架，下架后服务端拒绝创建。已有游戏的管理选择和显式下架状态保留，重启不会覆盖设置。测试夹具应验证默认可见，并在需要时显式下架。

可参考仅用于测试的 [后端验收模块](../internal/server/module_e2e_test.go) 和 [前端验收模块](../internal/server/testdata/module-ui.tsx)。它们演示了完全独立的动作字段、私有信息、交替行动、电脑玩家和结算，不会出现在正常生产构建中。

## 后端契约

| 回调或字段 | 责任 |
| --- | --- |
| `ID`、`Name`、人数上下限 | 游戏目录元数据；后端注册会校验重复 ID |
| `Configure(capacity, options)` | 校验并返回规范化选项；创建、修改设置和实际人数开局都会校验 |
| `Start(context, options)` | 返回初始 `Match`；自行创建游戏状态、随机结果和计时上下文 |
| `Apply(context, match, action)` | 解码本游戏动作、校验玩家和响应令牌，返回下一份 `Match` |
| `View(match, viewer)` | 返回观看者视图；`viewer == -1` 是观众；不得返回完整私有快照 |
| `BotAction`、`Reclaim` | 两者同时提供以支持电脑／托管；前者只选择动作（无操作时可返回 `nil, nil`，宿主继续检查其他受控席位且不写入空闲动作），后者恢复已过期的操作窗口 |
| `BotSearch` | 可选的后台选步；`Input` 是快速的席位信息投影，`Think` 在全局锁外执行并遵守取消，`Budget` 默认 1 秒、最多 10 秒；仍须提供快速合法的 `BotAction` 回退 |
| `AcceptStale` | 可选，允许同一响应窗口内的旧版本；平台仍拒绝未来版本、开局前版本和越权请求 |
| `OnTimer` | 可选，处理系统定时事件；`Context.Actor == -1`，不触发玩家托管 |

`Context.Actor` 来自已认证的房间席位，不能相信动作里的玩家编号。`Context.Now` 是本次操作时间，`Players` 是实际席位数。平台先验证身份和幂等键，再在候选快照上调用模块，数据库事务成功后才替换当前房间和广播。

`Context.Automated` 是宿主提供的席位控制状态，长度始终等于 `Players`。未离桌的电脑或托管席位为 `true`，其他席位为 `false`；客户端不能通过动作字段指定该值。宿主每次回调创建独立切片，模块不得修改上下文；直接调用模块的测试或其他调用方传入 `nil` 时，其语义由模块自行定义。

该字段反映回调执行时的事务状态。取消托管时，宿主先调用 `Reclaim`，成功后再清除该席位的 `AutoPlay`；因此 `Reclaim` 仍可能看到该席位为 `true`，下一次 `OnTimer` 才看到 `false`。`Restore` 从已保存的席位状态重新生成该字段，不从游戏快照或客户端输入恢复控制权限。

`Match.Data` 是游戏自己的 JSON 快照；具体类型、版本升级和随机状态由游戏维护。`Match.Schema` 可用于本游戏的格式版本。视图读取和动作执行均需要识别支持的版本，不能只靠更改字段名称升级旧对局。

`Actors` 是当前可行动玩家，`TimeoutActors` 是截止时仍欠操作的玩家，两者不必相同。`Deadline` 为 Unix 毫秒，零表示没有定时窗口。游戏自行保存暂停的时间预算、并发响应令牌等信息，不把复杂规则交给通用回合引擎。若没有提供电脑／托管能力，应使用无自动接管的流程，不依赖截止时间自动推进。

各席位使用独立截止时间的模块可设置 `ReclaimOnCancel: true`。平台在玩家从托管切回手动时调用 `Reclaim`，即使该席位不是全局最早到期的 `TimeoutActors`；模块自行判断该席位是否确实需要恢复操作窗口。回调不能延长尚未过期的时间、重新打开已提交操作、解除暂停或改变其他席位的截止时间。重复取消已经关闭的托管不再次调用回调；保存失败时控制状态与时间一起回滚。默认不启用，现有模块仍按原有最早到期窗口规则调用，实时模块仍在取消托管时清理旧控制。公共 HTTP 验收命令为 `go test -race ./internal/gameapi ./internal/server -run 'Test(ReclaimOnCancelRegistration|ModuleIndependentDeadlineReclaimHTTP|ModuleReclaimCancelPreservesOtherStatesHTTP)' -count=1`，证据保存在 `.local/module-reclaim-cancel/`。

需要跨天保存的游戏可显式设置 `Match.Paused: true`。暂停权限、投票和恢复操作由游戏通过正常 `Apply` 处理；模块须在 `Data` 保存剩余操作时间，并清空 `Deadline`、`SystemDeadline`、`TimeoutActors`，将 `Attention` 设为显式空数组。暂停期间不能有 `Result`，但 `Actors` 可保留供真人提交恢复投票。平台停止普通电脑、后台选步和系统推进；已在计算的后台任务会被取消，迟到结果不能写入。电脑不会自动提交恢复投票，游戏须根据席位控制状态处理电脑同意。

显式暂停的进行中房间连续 30 天无人操作后关闭，普通房间仍为 24 小时。期限以持久化的 `LastActive` 计算；刷新、观战和重启不延长保留期。关闭不计胜负或积分，沿用 `inactive` 原因和历史记录。恢复后立即使用普通闲置策略，并由游戏按已保存的剩余时间重建操作期限，不重新发放资源或推进回合。旧游戏未设置 `Paused` 时行为不变。前端公开 `room.game.paused` 和暂停时的 `pauseExpiresAt`（Unix 毫秒）；公共回合提示保留倒计时占位并显示「对局已暂停」，`BoardProps.clock.paused` 同步为真。

实时模块的暂停和恢复也属于必须先持久化的关键状态转换。帧内一个电脑操作进入暂停后，后续电脑和系统计时器不再执行；WebSocket 输入及系统帧必须在保存成功后才发布暂停或恢复状态，保存失败保留原局面。关闭页面根据持久化的暂停标记说明 30 天保留期，普通历史房间仍显示原有 24 小时说明。

公共暂停契约通过 `go test -race ./internal/gameapi ./internal/server -run 'Test(PausedEnvelopeValidation|ModulePaused)' -count=1` 和 `node --test web/tests/module-pause.test.mjs web/tests/closed-room.test.mjs` 验证，结果保存在 `.local/module-pause/`。HTTP／WebSocket 场景覆盖暂停与恢复、权限、幂等、事务回滚、电脑静默、运行中后台选步取消、实时帧与输入的暂停落库、跨天重启和到期关闭；组件渲染验证不代替游戏自己的桌面与窄屏交互验收。

`Attention` 单独指定需要提示「等待行动」的玩家，必须属于 `Actors`。未设置（`nil`）时沿用 `Actors`；显式空数组表示无人欠操作，持久化后仍保留区别。可选操作不应因此让全桌显示为必须回应。

`SystemDeadline` 为独立的 Unix 毫秒时间，零表示没有系统事件。它要求模块提供 `OnTimer`，但不要求支持电脑／托管。宿主在处理玩家超时前调用它，回调须清零截止时间或设到 `Context.Now` 之后，避免过期事件反复执行。快照、日志、战绩及系统动作记录在一个事务内提交；回调或事务失败时保留原状态并在后续调度重试。因此回调必须保持无外部副作用。玩家在临界时间提交的操作仍须由模块校验窗口和截止时间。

回合制游戏若需要出膛、命中或落地等短间隔系统事件，可显式设置 `Module.PreciseTimers: true`，使用 50 毫秒系统时钟。该设置只影响 `OnTimer` 的到期检查，不执行实时控制帧，也不改变玩家超时和电脑行动的调度频率。未启用的普通模块仍使用一秒系统时钟。

需要连续驾驶等高频操作时，可显式声明 `Module.Realtime: true`。宿主以 50 毫秒唤醒检查模块的 `SystemDeadline`，在同一事务内为每个合法受控席位调用 `BotAction → Apply`，再调用 `OnTimer`；模块决定物理子步和下次截止时间。回调须保持有界且不能配置异步 `BotSearch`。取消托管总会调用该模块的 `Reclaim`，模块只清除本席旧控制并保留赛程。进行中的实时帧和输入只通知同桌玩家／观众，结束等生命周期仍全局通知；普通模块保持原有计时节奏。

`Eliminated` 表示已经完成自身全部待决策事项和遗留责任的出局席位，不能再出现在 `Actors`。这些玩家可离桌加入新桌，但原局保留其席位编号、身份和最终名次；房主离桌时转移给仍在桌的真人。最后一名真人离桌后，进行中的模块对局仍保留名册，剩余电脑继续行动，原房主权限清除。模块不得把已离桌玩家重新加入行动者；未出局玩家仍遵守原有保留席位规则。

`Match.HasComputerPlayers` 表示包含不占平台席位的固定电脑角色。此类对局从开局起不计竞技积分，结算和历史归档再次核对该标记；不可把双人对战伪装成合作对局来跳过计分。角色编号属于游戏内部，`Actors`、`TimeoutActors`、结果及输入身份仍使用平台席位编号。

`Match.Result != nil` 表示完局，同时清空行动者和截止时间。结果包括最终名次、获胜席位、可选分数及公开详情。`Team` 选择现有团队计分政策；游戏决定胜负和名次，平台保存历史并执行积分政策。合作游戏设置 `Cooperative: true`，所有席位统一成功或失败；成功时全部席位列为赢家，失败时赢家列表为空，名次均为 1。合作局保存战绩但不计竞争积分。`Cooperative` 与竞争队伍计分的 `Team` 互斥。练习或休闲对局可设置 `Unrated: true`：平台保存成绩和历史，但不写积分流水、不在历史中标记胜者。详情不可包含需要继续保密的数据。

竞争和棋使用 `Draw: true`，至少两名玩家，`Winners` 为空、所有 `Ranks` 为 1；不能用“双赢家”代替和棋，也不能同时设置 `Cooperative`。纯真人计分局记录每人 0 分，不计胜场；含电脑的对局仍不计分。和棋原因放入 `Details`，公共结算、历史和刷新会保留和棋标志。下架游戏已有的历史仍可查看，但没有历史的未上架游戏不出现在玩家的空白统计卡中。

普通回调不执行数据库、网络或文件操作，也不修改输入。`BotSearch.Think` 是唯一的计算通道例外，可在平台锁外运行带取消的本地引擎；它不能访问宿主房间或数据库，只返回候选动作。机器人只根据公开信息与自己的秘密决策；选择动作和读取视图不能改变对局。随机结果应存入游戏状态，E2E 如需精确重放，应保留初始快照和随机结果，或在游戏内提供测试随机源。

后台选步同时最多占用 4 个工作槽，按房间只保留一份有效请求；未遵守取消的任务在实际返回前继续占用槽，不能因超时不断创建新工作线程。超时、异常、非法动作或工作槽满时走模块的快速合法回退。`Input` 必须先剔除该席位不可见的信息，不能把完整私有快照传给引擎。输入上限 1 MiB，候选动作上限 64 KiB。

平台将结果绑定到房间、对局编号、房间版本及席位身份。取消托管、关闭、重开和任何版本更新都会让旧任务失效；结果回收后仍经过 `Apply`、快照验证和数据库事务。自动超时接管会先保存控制状态，玩家可以在计算期间取消托管；正常结果、回退和重启恢复都不能重复记分或写入未提交状态。没有行动的成功结果保持静默，不产生空动作日志。

## 前端契约

[web/src/game-api/module.ts](../web/src/game-api/module.ts) 定义 `defineGame`。模块提供名称、介绍、人数范围、棋盘及可选的配置编辑、规则和封面组件。人数不是连续范围时提供 `playerCounts`（例如 `[2, 4, 6]`），首页据此准确显示可选人数。分类通过 `category` 声明为 `board`、`poker` 或 `realtime`，大厅、创建和后台共用分类下拉。必须满桌开局时声明 `requiresFullTable: true`，后端仍须严格校验实际人数和模式。支持机器人时声明 `supportsBots: true`，并与后端能力保持一致。自行播放音效的游戏声明 `ownsSounds: true`，宿主停止重复播放通用动作音；事件到音效的映射保留在游戏目录；通过 `BoardProps.audio.play([音效 URL])` 交给平台播放，退出时调用 `audio.stop()` 清理本游戏队列。播放器、用户交互解锁、静音、停止和失败处理由平台统一维护。游戏不得另建播放器或绕过全局静音。

新游戏复用宿主的创建卡片、等待房间、玩家栏和回合提示。`Cover` 提供与既有游戏一致的 SVG 插画；`PlayerSummary` 可在公共玩家卡中补充本游戏的公开资产摘要。不得通过游戏 CSS 隐藏或另行重排宿主布局；文字与背景须成对定义并验证最终对比度，详见 [AGENTS.md](../AGENTS.md)。

棋盘收到本游戏的 `data`、房间席位、当前观看者、`busy`、素材地址、全局音效开关 `sound`、服务器时间基准 `clock`，以及 `act`／`command`。一般游戏行动使用 `act({ ...本游戏动作 })`；平台负责加入当前版本和幂等键。`room.game.actors` 可用于提示当前行动者，但所有合法性仍由服务端确认。

新游戏的配置通过 `options` 传递。公共等待界面提供准备、开始、电脑玩家和设置保存，不需要扩展旧游戏的创建表单。个性化选项由模块的 `Options` 组件维护；保存配置会清除真人准备状态。

实时模块不使用操作倒计时自动托管。宿主在保存或发布快照前清零其 `Deadline`、清空 `TimeoutActors`；游戏自己的开局、选角与比赛期限通过 `SystemDeadline`／`OnTimer` 保留。恢复旧存档时，只有标为超时接管的真人席位恢复手动；所有手动操作席位清理旧控制，电脑和手动托管继续有效，系统期限不因清理控制而重置。输入不得延后已预约的系统帧，以免持续输入使电脑决策饥饿。

等待房间内需要按席位选择角色、队伍等内容时，前端可提供 `Preparation` 组件。宿主将它放在配置区域之后、座位区域之前；创建房间不渲染此组件。组件收到 `room`、服务端已保存的 `options`、`host`、`busy`、`assets` 和 `prepare(action)`。这里的 `options` 不包含 `Options` 组件尚未保存的草稿；提交使用平台 `command("prepare", { action })`，沿用房间版本和幂等键。

后端通过可选的 `Module.Prepare(PreparationContext, options, action)` 返回新配置。平台只允许等待房间中已入座、未离开的真人调用；`Actor`、`Host`、`Capacity` 均来自服务端房间，`Bots` 仅包含现有座位的 `Seat.Bot`，不补齐到房间容量，也不以托管状态代替。上下文中的切片、配置和操作输入均与宿主数据分离。模块负责本游戏的选择权限和合法性；平台再调用 `Configure` 校验返回配置，并在同一命令事务中保存配置、清除所有真人的准备状态、保留电脑准备状态。回调、校验或持久化失败均不提交房间变更。

公共准备扩展使用独立夹具验收，不运行具体游戏规则。从仓库根目录执行：

```sh
. scripts/env.sh
go test -race ./internal/server -run '^(TestModulePreparationHTTP|TestModuleRegistrationAndLifecycleE2E)$' -count=1 -v > .local/module-preparation-race.log 2>&1
node --test web/tests/module-preparation.test.mjs > .local/module-preparation-ui-after.log 2>&1
```

HTTP 验收记录保存在 `.local/module-preparation/http.json`，覆盖身份和元数据伪造、配置校验、版本与幂等键、SQLite 保存失败回滚、重启及禁止状态。前端契约记录保存在 `.local/module-preparation/ui-render.json`，验证公共等待框架内的组件位置、权威配置和命令传递；这是实际 React 组件的服务端渲染检查，不能替代具体游戏在浏览器中的交互和视觉验收。

实时前端声明 `realtime: true` 后，平台 `act` 沿用相同鉴权、nonce 和服务端规则接口，但不占用全局 busy 锁。模块必须合并持续输入、保留单调序号，并在松手、取消、失焦、托管及重连时清理旧意图；不能另建绕过平台权限的网络入口。预测仅改善显示，不能决定进度或结果。

实时通道通过 `/api/ws?stream=1` 发送初始完整状态及生命周期变化；高频 `game` 消息只包含当前房间的游戏快照和版本。每条输入经平台鉴权、席位权限、规则校验及有界 nonce 去重后确认。普通移动和物理帧在内存中推进，`ack.durable: false` 仅表示服务器已处理；不逐条写动作表或房间快照。平台每秒保存有变化的实时房间，正常关闭时保存剩余变化。开局、结算、托管、关闭等关键操作保持事务保存；游戏使用 `Match.CheckpointKey` 标记阶段／回合边界，键值变化也须先保存后发布，成功确认包含 `durable: true`。断线时清理未确认输入，重新连接后取得完整快照；过期房间或旧版本消息不得覆盖当前状态。完整状态和游戏消息包含进程级 `serverInstance`，新客户端输入携带同一标识；重启时拒绝旧实例控制，客户端接受检查点的较低版本并重建本地控制。正常重连读取内存最新状态；异常崩溃在存储健康时可能回退约一个保存周期。检查点失败保留待保存状态并记录最后成功版本，后续重试，失败期间不能保证一秒恢复窗口。`BoardProps.inputWindow` 表示有序流可用的输入窗口，连续输入最多 8 条，HTTP 回退为单请求；离散按键仍保留各游戏自己的批次上限。

引擎等连续音效使用 `audio.loop(key, sources, {volume, rate})` 与 `audio.stopLoop(key)`。sources 是同一声音的 CDN／本地回退；重复调用更新音量和音高速率，不从头重播。平台负责原生媒体、平滑更新、静音、离桌／隐藏清理和过期播放取消，游戏只维护事件映射与参数。

样式使用游戏根类名或 CSS Modules，避免为游戏向全局 `style.css` 增加规则。游戏可自行维护颜色、组件、动画和日志含义；只有不含游戏知识且确实稳定的工具才进入公共 UI。

## 开发和验证

从仓库根目录执行。所有缓存、临时数据库、浏览器配置和产物均保存在仓库内。公共平台修改先执行公共检查；按下文补充实际改动游戏的验收，不默认运行其他游戏测试。

```sh
. scripts/env.sh
python3 -B scripts/check_game_boundaries.py
npm --prefix web run build
python3 -B scripts/ci/scope.py run --game shared
go test -run '^$' ./...
go vet ./...
```

按游戏运行已有测试，例如：

```sh
go test -count=1 ./games/carcassonne/server
npm --prefix web test -- catan
```

前端测试入口不带游戏参数时会收集 `web/tests/*.test.mjs` 和全部 `games/*/tests/*.test.mjs`，这是主动全量入口，不用于普通单游戏修改；指定游戏时只运行该目录，无用例会报错。main 的检查范围由 `scripts/ci/scope.py` 选择：公共平台始终验证，游戏前端／素材只验证对应模块，规则、规则测试及夹具改动才启动对应 Go 套件。遗留在 `internal/server` 的游戏测试按文件与测试名称归属分配；旧多游戏完整对局使用准确的游戏子测试过滤。`go test ./...` 与既有全量分片脚本同样保留为主动全量验收入口。

本地执行 `python3 -B scripts/ci/scope.py run --game <id>` 验证单个游戏的前端和素材；服务端改动追加 `--backend`。`--game shared` 只跑公共平台 HTTP、存档与竞态检查。main 工作流摘要和 `verification-plan` 记录基线、改动文件与选择范围，`.local/ci/<id>/` 保存命令清单、结果和日志。基线采用已验证祖先构建；旧工作流迁移时可沿用已正式发布版本的匹配构建，不以尚未通过的上一条 main 提交作为基线。

独立接入的服务器 E2E：

```sh
mkdir -p .local/module-refactor
go test -json -count=1 -timeout=2m \
  -run '^TestModule(RegistrationAndLifecycle|BotAndTimeout)E2E$' \
  ./internal/server > .local/module-refactor/module-e2e.jsonl
```

真实浏览器接入验收需要已安装 `agent-browser`：

```sh
python3 -B scripts/accept_game_modules.py
```

该命令临时增加一个测试游戏，构建真实页面并启动测试服务，验证创建、设置、玩家操作、电脑推进、完局和刷新。随后检查现有六款游戏的桌面／手机牌桌。因此它用于确实影响这些公共界面的改动，不作为单游戏修改的默认验收。退出时移除临时游戏，并重新构建正常前端。不要与其他前端构建同时运行。

结果位于 `.local/module-refactor/browser/result.json`，同目录保留截图、页面快照和构建／服务日志。服务器 E2E 另保留 `module-lifecycle.json` 和 Go JSON 输出。CI 的 `shared-platform-acceptance` 产物保存服务器接入验收日志；浏览器验收当前由上述本地命令执行。

每款新游戏还应覆盖观战隐私、非法动作、重复请求、多人并发、超时／取消托管、重启恢复、结算与再来一局。产物记录运行命令和场景，不能只保留一个没有上下文的「通过」。

## 现有六款游戏的兼容范围

六款游戏的规则、局面、动作、机器人、前端专属类型与组件，以及规则测试已按游戏拆包。`internal/game` 是旧 API 适配层，只转发到这些包，以保持现有 JSON 和房间逻辑兼容。

已有复杂房间配置、计时、战绩集成仍在 `internal/server`，旧前端房间配置／展示集中在 `web/src/legacy/room-ui.tsx`。这些是尚未迁移到新契约的兼容逻辑，不是新游戏的接入模板。新游戏不添加 `game.State` 指针、全局动作字段、旧创建分支或旧牌桌分支。

现有游戏的公共 Go 类型／方法变化后，可执行 `go run scripts/legacy-game-api.go` 更新旧 API 转发声明；涉及旧动作协议时还需核对 `internal/game/game.go` 的动作映射。这一步只适用于六款旧游戏，新游戏不需要运行这个生成器。

保留兼容层意味着旧游戏的房间配置调整仍可能需要协调；本次没有宣称这些旧集成已全部迁完。下一款新游戏将直接使用新接口，用实际开发验证契约后，再按游戏替换旧适配逻辑，避免一次重写已有扩展规则。

## 屏幕操作按键

平台统一禁止 `button` 和 `[role="button"]` 的文字选择及 WebKit 长按菜单。方向盘、方向键组、蓄力等屏幕操作区使用 `data-game-controls`，将同一保护应用到按键空隙与说明；游戏保留各自的 `touch-action`、指针捕获和取消处理。不得将禁选范围扩展到整页、聊天或普通正文。


## 等待房间保存并准备

模块等待房间的房主修改地图或规则后，可通过“保存并准备”一次提交 `ready {options}`。宿主先验证权限与模块 Configure，再在同一事务保存选项、清除其他真人对旧规则的准备并确认房主准备；电脑保持准备。未改变的设置不会重置其他真人准备。普通 ready 不带 options 时继续切换本人准备状态，其他人不能借 ready 修改设置。

## 开局视区定位

平台在进入对局后统一定位到 `.game-main`，初始布局稳定后执行一次，给顶部保留 8px。用户已滚动、触摸或操作键盘时取消待执行定位；普通快照不重定位。游戏不再自行重复执行开局滚动；选择浮层后的阶段定位仍按该游戏的明确需要处理。游戏需为主要画面、关键 HUD 与操作区预留手机视区高度，不能靠隐藏公共玩家栏解决。
