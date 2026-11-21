# CS 1.6 即时同步审计

本次按 [即时游戏开发规范](../../../rules/realtime.md) 只读复核冻结源码与已有产物，没有运行负载、测试、构建或浏览器。生产实现已冻结；初次审计提出的显示平滑、传输过期、规则身份和十真人容量问题均已有实现与对应验证。最终无过滤四图浏览器 E2E `formal-fifth` 已完整通过，退出码为 0，功能与性能失败列表均为空。结论限于报告记录的环境、样本与预算；Aztec 模拟触屏 P95 恰为 50.0 ms，达到阈值但没有余量。

证据统一位于 `.local/cs16/verification/network-audit/`。`source-manifest.json` 与 `latest-source-manifest.json` 保存早期快照；`frozen-source-manifest.json` 保存本次复核身份。旧诊断轮次与旧负载仅作为历史记录，不计入当前通过结论。

## 当前结论

常规同步使用有序输入、独立接收／应用序号、按席位裁剪的动态投影、新增事件和动态 brush 增量。正常帧及恢复包均不传静态 Map、模型、完整存档或历史环。恢复发送 `mapId` 与 `rulesSHA256`，复用已校验的资源缓存。

可选 `RealtimeDeferredInput` 只允许成功 Apply 的普通排队输入延后房间投影，内存提交、ACK 和回执保持即时；下一规则帧照常发布。购买、取消、关键检查点、结算及公共元数据变化仍即时广播。默认未声明的模块保持原行为，跳过广播仍取消失效电脑搜索。

| 审计项 | 当前状态 | 证据与边界 |
| --- | --- | --- |
| 恢复重复静态 Map | 已修复并验证 | 四图十席初始／恢复投影仅动态数据；`server/recovery-size.json`、`recovery-privacy-green.jsonl` |
| 小位置误差显示平滑 | 已实现并通过针对性验证 | `DisplayCorrection` 仅处理不超过 8 原始单位的小误差，在首个 80 ms 窗口内归零；生命、姿态、大误差、恢复及障碍变化立即服从权威；`web/correction.log`、`correction-trajectory.json` |
| 传输中过期输入与服务器排队混淆 | 已修复并验证整数边界 | 服务器接收时刻与引用权威帧相差最多 500 ms，执行排队另限 200 ms；500／501 ms、200／201 ms、取消清理例外和客户端恢复清队均已覆盖；`server/control-header-green.jsonl`、`web/transport-expiry.log` |
| 权威规则与外部资源身份 | 已实现并通过资源契约验证 | 加载前及每次权威更新检查 `rulesSHA256`；gzip 使用解压后 `decodedSHA256`，不匹配释放输入并停止本端；`web/assets.log`、`cdn/summary.json` |
| 同显示帧的短按边沿 | 已修复并验证 | 预测帧严格递增，连续输入限 10 ms 合并，离散边沿保序；`web/input-pacing.log` |
| 十真人高频输入容量 | 冻结源码通过 | 十个独立认证真人连接，1000 条输入，849 条／s，ACK P95 95.806 ms，最大在途 8；详见下节 |
| 四房间与慢观战隔离 | 冻结源码通过限定负载 | 四房各十实体，每房一真人加九电脑，慢观战不读取；ACK P95 4.156 ms、规则落后 P95 36 ms；不等同四十高频真人 |
| 当前构建完整浏览器认证 | 正式四图用例通过 | `formal-fifth/report.json`；231 个源码及 16 个实际产物在运行前后、审计复核时均匹配；触屏结果仅限模拟环境且无性能余量 |

## 冻结源码的容量与公共兼容验证

最终容量证据为 `.local/cs16/verification/server/capacity-deferred-summary.json`。运行前后 420 个文件清单与聚合 SHA-256 均为 `bf3119ab3a7d7b7037532d1e0afef5cf79c7c0d150460058be2839b2c2f4ba4a`；本次再次逐文件比较，生产文件无变化，仅 `games/cs16/tests/browser_fixture_test.go` 在后续加入夹具运行身份元数据；正式浏览器报告已绑定该最终夹具源码与实际执行文件身份。清单包含 CS16 服务端／引擎及公共服务器、gameapi 等范围，不能用这个服务端容量摘要替代前端构建身份。

十真人测试使用真实 HTTP、WebSocket、SQLite 和十个独立认证账号。各连接先执行 40 条普通操作，再以每人 100 Hz 目标速率提交 100 条视角输入；在途上限固定为 8。1000 条均获确认，耗时 1177 ms，实测吞吐 849 条／s，ACK P95 95806 μs。冻结阈值为至少 800 条／s、ACK P95 不超过 100000 μs。高频窗口采到 23 次规则时钟，落后 P95／最大值均为 65 ms；普通窗口为 45／47 ms。测试同时覆盖重复 nonce、托管清理权限、手动接管和服务重启。ACK 是传输回执，不表示规则已应用或画面已显示。

四房间测试为每房一真人与九电脑，另加接收缓冲 1024 字节且不读取的观战连接；ACK P95 为 4156 μs，规则落后 P95 为 36 ms。它验证多房间规则推进与慢连接隔离，不声称验证了四十名高频真人客户端。

公共契约新增验证与既有回归分别保存在：

| 范围 | 结果 | 产物 |
| --- | --- | --- |
| 普通输入延后投影 | 普通与 race 各 12 个通过事件；十真实 WS 连接、100 个即时 ACK、规则帧前零重复投影、下一帧完整状态、重连和权限通知 | `deferred-input/summary.json` |
| 既有 realtime 兼容性 | 26 个通过事件；批次、排队、检查点、恢复、游标、流、加载和持久化 | `realtime-regression/summary.json` |
| 会话缓存 | 普通与 race 各 9 个通过事件，原有鉴权／管理 24 项通过；40 WS 夹具的重复会话 SQL 从 122 次降至 0 | `auth-cache/summary.json`、`auth-regression/summary.json` |
| 共享与游戏 lint | 最终登记范围内 139 个源码通过；40 个共享 Go 正负例与 12 项 lint 测试通过 | `lint/final-source.log`、`lint/shared-go-summary.json` |

延后投影的十条连接使用同一账号验证通知扇出；它与十个独立真人账号的容量测试是不同夹具。会话缓存仅缓存已验证会话身份，账号、会话与权限相关数据库写入通过提交／回滚钩子失效缓存，动态房间与席位权限仍逐次检查。

## 源码逐项核对

| 要求 | 当前实现 | 结论 |
| --- | --- | --- |
| 本地先响应，再服务器校正 | Board 的准备回调立即进入 `MotionPrediction.input`；鼠标立即更新 facing；服务器决定伤害与资源 | 源码、预测验证与正式原生像素样本符合 |
| 接收与应用确认分开 | `ReceivedSeq` 在接收入队时增加；`AppliedSeq` 在规则消费或丢弃过期输入时增加 | 未把 ACK 当作规则应用 |
| 在途与预测有界 | 游戏最多 8 个在途、64 个队列项；公共层最多 32 个待 ACK、64 KiB socket 缓冲；预测最多 15 个 10 ms 步 | 源码及限定容量验证符合 |
| 连续输入合并，短按保序 | 连续值按 10 ms 节流／替换；射击、跳跃、换枪等边沿单独保留并排入递增预测帧 | 针对性验证通过 |
| 传输与执行时限分开 | `InputReceivedAt` 由宿主记录；引用权威帧限制传输时限，当前服务器时钟限制队列时限 | 边界验证通过；取消仅允许全释放 |
| 只传必要动态投影 | 显式裁剪玩家、弹药、目标、投掷物、掉落物及 brush；不含 History、Pending、Seed、AI 路径或完整 Match.Data | 源码与投影测试符合 |
| 静态规则身份闭环 | 服务端嵌入规则摘要、公开 `mapId`／`rulesSHA256`，前端比对清单身份并缓存 | 不再有 Map 兜底通道；正式 CDN 发布另行验收 |
| 每连接新增事件与 brush | 游标绑定 epoch、frame、eventId、round、brush、控制权限、追赶状态和 viewer | 只发新增事件；未变 brush 省略 |
| 成功写入后推进游标 | `pushFrame` 先 write 再更新 cursor；`Unchanged` 保留旧游标 | 公共 WS 验证通过 |
| 重复 game 不回退完整 state | 私有跳过信号先于 nil 回退处理；公共指纹只排除 version／serverNow | 公共 WS 验证通过 |
| 公共状态不能漏通知 | loading、clockOffset、setup、actors、attention、paused、finished 等均参与公共指纹；需要时空游标重取 | 公共变化、混合操作与权限验证通过 |
| 新规则帧继续增量 | 首先使用现有 cursor 调用 StreamView；正常 deadline 变化不会强制补发全部 brush | 公共回归通过 |
| 游标缺口显式恢复 | 无效游标、跨 epoch、事件环缺口或 viewer 不符返回恢复投影 | 恢复只含当前动态状态，无历史 Events／Map |
| 事件和声音不重播 | EventCursor 首次接入从最新 ID 开始，重复 ID 过滤，恢复推进到当前事件 | 源码符合 |
| 显示时钟与校正分离 | 普通确认保留显示时钟；有界重放后仅显示层 `present` 平滑小误差 | 不修改权威或预测规则结果 |
| 规则、网络、显示时钟分离 | 规则 100 Hz，网络按规则截止批次投影，绘制使用独立 rAF；远端约 50 ms 插值且最多六份快照 | 正式样本 game 约 33.18～33.49 次／s、显示约 60 FPS |
| 断线禁止 HTTP 输入回退 | `streamInputOnly`；WS state 首包后才 attach，断线清 pending 且 inputWindow 为 0 | 源码与公共回归符合 |
| 失焦、隐藏与取消释放 | 清键鼠／触屏、待预测操作并发送 cancel；服务端清 pending、动量和投掷蓄力，无新输入 300 ms 后释放保持态 | 源码与输入恢复验证符合 |
| 同局重连复用资源 | 同一 room／epoch／map／viewer 及同一 serverInstance 保留 Board 和资源 effect | 服务实例变化会重建；正式同实例后台／重连验证通过 |
| 初次加载门槛 | 解码、规则初始化及首次绘制后 Ready；后续重连不重建全员门槛 | 公共加载回归通过 |
| 慢端不阻塞其他人 | 每连接独立有界帧信号、游标、ACK 队列与写超时；投影和写入在公共锁外 | 冻结四房慢观战负载通过 |
| 截止计算隔离 | 最多四个工作器，同房只算一帧，提交核对房间身份及版本，冲突有界重试 | 源码与容量证据符合 |
| 不自动托管真人 | 慢、闲置、隐藏或断线只停输入；平台显式托管与手动接管保留鉴权 | 权限与恢复验证通过 |
| 权威和私有状态隔离 | 输入不接受位置／伤害／结果，投影按席位裁剪；恢复保持鉴权 | 无原生内存检查点外泄 |
| 不跳过规则步 | Advance 完整推进移动、战斗、目标和历史；显示只绘制最终展示帧 | 源码符合 |

## 正式浏览器与实际构建绑定

最终报告为 `.local/cs16/browser/reports/formal-fifth/report.json`，小型汇总为同目录 `verification-summary.json`。`diagnostic=false`、`partial=false`，未筛选地图或跳过自然结束，四图各有十席，使用真实 HTTP／SQLite／WebSocket、两个真人页面、八个电脑及观战页面，没有注入规则状态。Assault 在两名真人显式开启平台托管后自然完成两局，最终比分 2∶0。它与上节十独立真人的高频容量测试分别覆盖浏览器和服务端负载。

报告绑定 231 个源码文件，聚合身份为 `aff311ec3e9e7ad40f3d1580c6c7fb68213dd98f7f5f25b446f9cba7d6e0a9f0`，范围含游戏、公共传输、鉴权、控制器和构建入口；16 个实际前端文件包括 HTML、JS、CSS、清单、WASM 与运行脚本。运行期间 `sourceChangedDuringRun`、`artifactsChangedDuringRun`、`runtimeSourceMismatches` 均为空，本次再次逐文件比对当前内容也全部一致。

| 实际执行产物 | SHA-256 |
| --- | --- |
| index.html | `0fc996ab83af416d159e24008385ccc0e5570c63d5a5667ae1ddcf8e059f2bbe` |
| motion.wasm | `77fc242bbdf702fd51ff070dc3aa6788908ef986091364452beeda585d71edc7` |
| 运行中的 Go 测试夹具二进制 | `200e008b1c9118f390eb6de979d35b3cf0bbfc5aed2564f1975d6841d8f2410b` |

WASM 的规则源码身份为 `745e1b4b2016afc7939f79e3657b3eccc17ee5088c253da29a1ca223cfd5ec9c`。JS／CSS 和其余实际产物的逐文件身份保存在 `frozen-source-manifest.json`；Go 夹具使用生产服务器包，执行文件为 darwin／arm64、Go 1.26.2，摘要由夹具运行时记录。`go test` 退出后临时执行文件已删除，保留的是运行时身份记录与可复跑命令，不声称保留了该二进制副本。

最终本地构建资源清点见 `.local/cs16/verification/cdn/final-built-inventory.json`：CS16 共 997 项、60798906 字节，含当前运行时；index 和清单身份与正式报告一致。检查外部请求为 0，仅验证本地构建载荷与清单，不表示已经上传、发布或完成 CDN 回读。

以下是正式原生事件到首次 WebGL 像素变化的样本，不包含实体屏幕扫描输出。桌面每图测 12 次键盘和 8 次鼠标响应，Aztec 测 24 次模拟触屏响应；显示频率按正式探针的可见更新定义计算。

| 地图 | 原生响应 P95 | 显示帧间隔 P95 | 显示频率 | game 包 P50／P95／最大值 | game 频率 | 收包字节率 |
| --- | --- | --- | --- | --- | --- | --- |
| de_dust2 | 键盘 16.0 ms，鼠标 12.9 ms | 16.8 ms | 约 60.00 FPS | 4807.5／4833／5012 B | 33.49 次／s | 约 163996 B／s |
| de_inferno | 键盘 16.4 ms，鼠标 14.0 ms | 16.7 ms | 约 60.00 FPS | 4803／4809／4999 B | 33.31 次／s | 约 162854 B／s |
| de_aztec | 模拟触屏 50.0 ms | 16.7 ms | 约 60.00 FPS | 4810.5／4900／4941 B | 33.20 次／s | 约 162741 B／s |
| cs_assault | 键盘 17.0 ms，鼠标 15.3 ms | 16.7 ms | 约 60.00 FPS | 4602／4611／4741 B | 33.18 次／s | 约 155290 B／s |

包大小与字节率均为实际 WS 应用消息正文，不含 TLS／TCP／IP 头。P50 从保留的 `gameMessageSizes` 重算，P95 与最大值逐项匹配正式报告；完整结果见 `formal-browser-metrics.json`。正常样本 WebSocket RTT P95 为 40～48 ms，输入 ACK P95 为 53.116～66.771 ms，规则应用 P95 为 74.863～77.970 ms；这些值与本地原生像素响应分别报告。

**触屏只在本次样本达到阈值。** 正式 Aztec P95 精确为 50.0 ms，冻结预算也是 50 ms，余量为零；同一实现第四次正式运行曾为 50.2 ms 而失败，短诊断为 49.6 ms。短诊断不替代完整正式轮次，最终通过也不抹去这段波动。本次原生派发排队 P95 为 45.4 ms。测试使用 Chromium 触屏模拟，不能将其推广为实体手机或任意设备都能稳定满足 50 ms。竖屏 320×760、横屏 760×360 的原生布局与截图尺寸一致，控制按钮和 HUD 无重叠；原生全屏与页面全屏分别保留证据。

快慢端共存中，慢端 RTT P95 269 ms、ACK P95 287.069 ms，仍有本地键盘像素响应 P95 18.4 ms；同时快端键盘 P95 16.9 ms、约 60 FPS。断开另一真人连接时快端继续推进 57 帧，真人未被自动托管且恢复成功。可信 window blur 后 0.2 ms 发送全释放输入，后台恢复保留同一 canvas；显式托管／接管保持同一对局。

真实页面恢复观察 12.179 s，规则从 4366 帧推进到 5595 帧，出现 65 次射击；Map 包数为 0，game 包 P95 为 4386 B、最大 14708 B。Assault 自然战斗观察 14.038 s，三名射手共 10 次射击，并出现门和命中事件；game 包 P95 为 9246 B、最大 9323 B，约 33.42 次／s，峰值一秒收包为 239335 B。它验证的是该自然战斗样本，不扩展为任意投掷物密度的压力保证。两项观察均没有内部字段泄漏。

四图原生探针均支持 Long Tasks API，样本内未记录超过 50 ms 的长任务；探针自身 GPU 像素读回 P95 分别为 2.3、2.5、2.1、2.6 ms。这些是明确测到的观测开销，不等同主线程、WASM、状态复制、校验与绘制各自的独占 CPU 时间。正式记录分别保留 RTT、ACK、规则应用、帧间隔、原生派发和像素时间，但尚未形成同一时基的权威包到达→显示排队全链路分解；不由稳定 FPS 或零长任务推断未测分项。

## 历史诊断记录

以下保留真实 Chromium WS 诊断记录，均为旧版本且整轮未通过；不参与当前版本认证。包大小为 UTF-8 WS 消息正文，不含 TLS／TCP 头；频率由留存消息首尾时间计算。

| 历史样本 | game 包中位数／P95 | game 频率 | 边界 |
| --- | --- | --- | --- |
| diagnostic-third，aztec 触屏 | 9783／9793 B | 约 78.37 次／s | 整轮失败且源码已变 |
| diagnostic-fifth，dust2 桌面 | 5509／5533 B | 约 87.67 次／s | 曾测 20 次像素响应，整轮失败 |
| diagnostic-seventh，dust2 桌面 | 4741／4765 B | 约 33.02 次／s | 6.51 s 采样，原生选枪等用例失败 |
| diagnostic-seventh，aztec 触屏 | 4719／4735 B | 约 31.01 次／s | 仅 0.35 s、一次响应，不能作为延迟统计 |

历史来源比较见 `browser-evidence-review.json`，wire 重算见 `historical-wire-metrics.json`。初次恢复曾重复携带约 1943799 B 的 Map，此值来自旧投影重编码，不是新的 wire 测量。旧四房负载记录已由上述 `capacity-deferred` 冻结证据替代，不再列为当前容量缺口。

## 源码入口

| 范围 | 入口 |
| --- | --- |
| 动态投影、游标与恢复 | [server/view.go](../../../games/cs16/server/view.go)、[server/rules_identity.go](../../../games/cs16/server/rules_identity.go) |
| 输入时限、普通排队判定与规则时钟 | [server/module.go](../../../games/cs16/server/module.go)、[server/deferred_input.go](../../../games/cs16/server/deferred_input.go) |
| 接收／应用与释放 | [engine/state.go](../../../games/cs16/engine/state.go) |
| 客户端队列、预测与显示平滑 | [input.ts](../../../games/cs16/web/input.ts)、[prediction.ts](../../../games/cs16/web/prediction.ts)、[presentation.ts](../../../games/cs16/web/presentation.ts) |
| 资源缓存与规则身份 | [assets.ts](../../../games/cs16/web/assets.ts)、[asset-contract.ts](../../../games/cs16/web/asset-contract.ts)、[brush-cache.ts](../../../games/cs16/web/brush-cache.ts) |
| 生命周期与绘制 | [Board.tsx](../../../games/cs16/web/Board.tsx)、[renderer.ts](../../../games/cs16/web/renderer.ts) |
| 公共输入提交与游标 | [realtime_batch.go](../../../internal/server/realtime_batch.go)、[realtime_stream.go](../../../internal/server/realtime_stream.go) |
| 会话身份与失效 | [session_cache.go](../../../internal/server/session_cache.go) |
| WS 首包、断线及房间动作范围 | [main.tsx](../../../web/src/main.tsx)、[action-scope.ts](../../../web/src/game-api/action-scope.ts) |
| 原生像素、wire 与构建绑定 | [native-input.mjs](../../../games/cs16/tests/native-input.mjs)、[accept_browser.py](../../../games/cs16/tests/accept_browser.py) |
