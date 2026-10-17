# 篮球服务端接入

`Module()` 已实现平台房间接口，当前仅供开发验收，尚未加入正式注册表。创建入口开放 1／2 个平台席位，支持单人、双人合作、双人对战；准备阶段可选择已验收的球场 0／1／2／3；默认纽约。分队在平台创建时选择，比赛内不重复显示原版人数菜单。联机 `Board` 已完成真实 HTTP、Go/Wasm 和原生输入的针对性浏览器验收；完整多人比赛、其他球场的平台选项及正式发布仍待接通，不能据这些检查宣称游戏已上架。

## 场地和球员准备

场地与球员图片选择在统一等待房间中显示，创建弹窗只提供人数与合作／对战模式。平台 `Prepare` 命令验证真人席位、等待状态、房主权限和版本，篮球模块校验具体选择；成功后全部真人重新准备。房主可选择场地和阵容，其他玩家只能选择自己控制的角色；同队电脑可交换，不能抢占真人队友的角色。SQLite 保留选择，开始时直接据此创建原版球场和角色配置。

当前球队仍为原 0／1 队，人物槽 0／1；剩余队伍与特殊动作继续恢复。图片已完整导出 8 个球场与 32 人物，但不据此扩大可玩范围。准备验收见 [PREPARATION_SELECTION.md](PREPARATION_SELECTION.md)。

## 人数与分队

| 模式 | 平台席位 | 原版角色编号 | 固定电脑 |
| --- | --- | --- | --- |
| 单人 `solo` | 1 | 第一席 → 0 | 1、2、3 |
| 双人合作 `cooperative` | 2 | 第一席 → 0，第二席 → 1 | 2、3 |
| 双人对战 `versus` | 2 | 第一席 → 0，第二席 → 2 | 1、3 |

原版角色 0／1 属于 A 队，2／3 属于 B 队。默认一人为单人，两人为合作。人数与模式不符时拒绝创建或开局，不能把未坐满的双人房间静默转为单人。四席仅保留在开发夹具和旧开发存档中，用于验证四个独立控制器；正式模块的最大人数为 2。

视图中的 `seatActors` 映射平台席位到角色，`localActor` 是当前观看者的角色编号，观众为 -1。`inputs` 和 `videoInputs` 仍按四个原版角色存储。输入请求的 `seat` 始终是平台鉴权席位；浏览器本地预测及确认序号使用映射后的角色编号。托管、超时、战绩和玩家栏使用平台席位，固定电脑角色不创建虚假用户。

三种模式都设置 `Match.HasComputerPlayers`，因此不计竞技积分。单人／合作使用统一合作结果；双人对战使用竞争队伍结果，只把胜者映射到实际参与的席位。

## 输入和时间

玩家只提交本席逻辑按钮：

```json
{"type":"buttons","epoch":"当前比赛标识","sequence":1,"buttons":1}
```

`buttons` 使用引擎的逻辑位，不是键盘键码。默认向右为 `1`；物理按键、连发和设备映射由用户控制器设置处理。`sequence` 按席位递增，单次跳跃最多 1,024，不能超过 JavaScript 安全整数范围。未知字段、其他比赛标识、过期序号及观众操作均拒绝。并发玩家使用各自序号，不因其他席位先提交而丢失操作。

普通松键仍提交 `buttons:0`，保留此前短按。失焦、设备取消或主动退出输入使用：

```json
{"type":"release","epoch":"当前比赛标识","sequence":2}
```

取消会清除本席未消费的输入队列及锁存，不清其他席位。每席最多保留 64 个按钮变化，记录服务器接收时间；100ms 系统帧内按 60Hz 视频边界消费，避免把多次连发压成一个按下位。客户端不能提供位置、分数、随机数、电脑身份或绝对输入时间。

浏览器联机使用可靠批次，保留等待响应期间的短按与连发：

```json
{"type":"inputs","epoch":"当前比赛标识","seat":0,"basedOn":1000000,"sequence":2,"events":[{"sequence":1,"buttons":64,"offsetMS":0,"release":false},{"sequence":2,"buttons":0,"offsetMS":20,"release":false}]}
```

`seat` 必须等于认证席位，`basedOn` 回显最新快照的 `serverAt`。服务端拒绝未来值和超过 1 秒的陈旧快照。事件序号严格递增，最后一个等于顶层序号；相对间隔非递减、最长 500ms、最多 64 个事件。已确认事件按幂等方式忽略，不重复按下或刷新断线期限；任何校验失败都不改变存档。

批次中的 `release:true` 清理旧队列与锁存，可优先于旧按键请求发送。后续手势携带尚未确认的取消事件，使乱序请求不复活旧输入。可选 `cancelMask` 只取消指定逻辑位：例如 `64` 只取消 B，保留键盘持续按住的方向及其排队边沿。未提供时取消本席全部按钮。只有取消事件能提供该字段，范围为 1～255。

`web/network-input.ts` 实现账号／房间／席位／比赛绑定、序号确认、批次合并、250ms 持续按键心跳、优先取消和过期清理。超过 500ms 未确认的可玩输入会清除并要求新手势；普通空闲零值不发送心跳。失焦和设备取消仍由输入组件触发，服务器 750ms 释放作为断线兜底。该类已接入 `web/Board.tsx`，范围和限制见 [联机画面验收](../web/NETWORK_PRESENTATION_VERIFICATION.md)。

服务端另保留最多 120 个已执行视频帧的输入记录 `videoInputs`，用于浏览器 Wasm 重放快照间动画。记录包含真实的托管标志、短按锁存和取消状态。它是只读回放数据，不能作为玩家操作提交；只有原逻辑按键协议可改变比赛。

750ms 未收到输入时释放持续按钮；15 秒无有效操作由平台超时机制转为托管。`Context.Automated` 是宿主提供的真实席位状态，游戏用它选择原版 AI 输入路径。`BotAction` 不创建第二套策略：原电脑决策仍在每次原世界输入阶段执行，规则和物理与真人共用。

取消托管只重置该席物理按钮／输入组合缓存，保留角色动作、AI 记忆、位置、比分、随机数及其他席位期限。暂停期间原世界物理和比赛钟停止，但原主循环随机数与投影仍推进；恢复验收按实际视频 tick 对齐，不能把整个暂停世界假定为静止字节串。

## 恢复与结果

音乐单独保存原曲目、代次、分发文件摘要和服务器媒体时间线。快照提供当前音乐位置，初次连接／恢复只定位当前曲目，不重放历史效果。原 253／255 决定音乐暂停，游戏 START 暂停不自行更改 BGM。位置按 1 倍速媒体墙钟推进，不由逻辑帧推算 PCM 时间；服务器恢复保留音乐起点。详见 [音乐恢复与验收](../web/AUDIO_SCENE_STATE.md)。

服务端保存完整运行状态和输入序号。SQLite 恢复保持比赛标识、规则状态与已有期限，清掉陈旧网络按钮，并重新建立实时调度基准；长时间停顿最多追赶 250ms，不瞬间模拟离线期间整场比赛。

结算来自原 `Outcome`。确认后提交已有的最后结果画面、停止单场推进，按所选模式将原赢家映射到实际平台席位。原奖励／菜单去向保留在结果详情中，不在此单场宿主执行。

`RuntimeAdvanceError` 为失败规则保留精确运行前态和四个控制器输入，普通日志只输出帧号和原因。HTTP 验收观察器另存首次失败状态；它不改输入、时钟或返回值，出现过规则错误就不能被后续偶然继续运行掩盖为成功。

## 验收

```sh
source scripts/env.sh
go test -race ./games/basketball/server -count=1 -v
go test -race ./games/basketball/engine -run '^TestRuntimeAutomation' -count=1
```

初次服务端适配的 8 项验收通过：5 项明确的模块边界验证，以及 3 项真实 HTTP／身份／系统时钟／SQLite 验证。覆盖默认下架、独立并发输入、观众与注入拒绝、两次连发、取消隔离、暂停／接管／恢复、真实超时托管、三名电脑参与、长停顿和团队结果。HTTP 房间使用随机种子；两次实际运行暴露的道具撞人和持球普通动作缺口均已保留失败前态，并从原代码恢复后重放验证，没有换种子绕过。

最终日志：`.local/basketball/engine/platform-race-verified.log`；HTTP 快照：`.local/basketball/server-http/`。定时规则失败前态另存为 `*-timer-failure.json`，空输入 HTTP 快照可用 `scripts/basketball/replay_http_runtime.py` 在下一个平台控制期限前复现。此范围不是完整 HTTP 对局、多人浏览器或网络预测验收。

可靠批次新增验证：

```sh
source scripts/env.sh
node --experimental-strip-types --test games/basketball/tests/network-input.test.mjs
go test -race ./games/basketball/server -run '^TestHost|^TestBasketballHTTPJavaScriptTransport$|^TestBasketballHTTPReliableInputBatches$' -count=1 -v
```

批次接入阶段的记录为 10 项 JavaScript 隔离测试、13 项 Go 竞态检查通过；此后新增的逐帧历史检查见联机画面报告。真实 JavaScript 发送器通过认证 HTTP 路径验证 9 个条件，覆盖延迟投递、响应丢失、取消抢先到达、过期清理和部分设备取消。测试仅显式延迟请求／响应，未替换游戏动作、注入世界状态或缩短时钟；先用真实 Start 暂停比赛，再检查网络边界。

记录为 `.local/basketball/engine/network-client-after.log`、`network-final-race.log` 和 `.local/basketball/server-http/javascript-transport.jsonl`；实现前失败记录保留为 `network-batch-before.log`、`network-client-before.log`、`network-partial-before.log`、`network-client-partial-before.log`。该发送器使用 Node 执行以跨越真实 HTTP 边界，不能当作真实浏览器的预测、渲染或多端完整对局验收。


人数模式本次验收与复现命令见 [PLAYER_MODES.md](PLAYER_MODES.md)。既有四席开发验收继续保留，不能将它误认为正式入口人数。
