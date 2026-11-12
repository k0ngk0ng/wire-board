# 赛车联机协议只读审查

当前赛车采用「本地运动预测 + 低频权威运动投影」。本人按键会先进入本地规则和下一次绘制，不等待服务器 ACK。静态赛道、地形与车型资源按内容摘要加载，常规赛车投影不发送完整地图或美术素材。现有代码具备连接级背压、预测上限和断线释放，但未完整满足 [即时游戏开发规范](../../../rules/realtime.md)：仍重复发送整个 24 条历史事件环，也没有分别表达「输入已接受」与「物理步已应用」的序号。

本文审查源码，并纳入最终隔离性能／延迟场景的原生 WebSocket 字节统计、握手和输入到像素测量。最终构建在已测场景内通过预先冻结的体验门槛；上述协议缺口仍保留，不据此声明全部实时规范已经满足。本次仅更新文档，不修改运行时代码。

## 本地响应、输入与权威校正

| 环节 | 已观察的实现 | 具体边界 |
| --- | --- | --- |
| 本地按键 | `TrackCanvas.tsx:202` 的每次 `requestAnimationFrame` 先采样控制器，再用 `predictCar` 推进本人车辆 | 不等待 ACK 或服务器快照；单次绘制步长至多 50 ms |
| 本地规则 | `prediction.ts` 使用赛车运动、地形和碰撞规则 | 不负责权威圈数、胜负或持久破坏状态；其他车辆使用有界外推，观战保留 110 ms 显示缓冲 |
| 预测上限 | 快照校正时的预测年龄最多 200 ms；普通逐帧预测在快照年龄达到 250 ms 后停止 | 不随网络积压无限增加预测或显示缓冲 |
| 输入发送 | `Board.tsx:567` 每 40 ms 尝试取下一条 `LatestDrive` | 连续控制变化至少相隔 100 ms；保持输入每 300 ms 发送；全释放、喇叭释放可绕过 100 ms 限制 |
| 有序窗口 | `LatestDrive` 使用 race epoch 和递增 `seq`；WebSocket 在途上限为 8，HTTP 回退为 1 | 释放无需等待上一条 ACK，但窗口满时仍须等待容量；只保留最新控制意图 |
| 接受确认 | 每条操作有唯一 nonce，`StreamInputs` 按 nonce 完成 Promise；`durable` 区分内存接受和持久化 | 5 秒 ACK 超时；平台待确认操作上限为 32，浏览器 socket 缓冲超过 64 KiB 时拒绝继续提交 |
| 权威确认 | `Car.Seq` 在 `Apply(drive)` 接受操作时立即更新 | 该序号不是已完成物理积分的确认，不能据此删除精确重放所需的历史输入 |
| 当前校正 | 每次数据／时钟变化后，以权威车辆和「当前输入」预测到显示时间，再衰减位置／角度误差 | 小于 150 单位且未复位的误差平滑处理；更大差异或复位直接采用新状态 |

相关源码：`games/racing/web/input.ts:45`、`games/racing/web/Board.tsx:278`、`games/racing/web/TrackCanvas.tsx:88`、`games/racing/web/TrackCanvas.tsx:219`、`web/src/realtime-stream.ts:170`、`web/src/main.tsx:483`。

### 已应用序号与重放缺口

调用链为：服务器 `Apply(drive)` 设置 `Car.Seq` 并追加带接收时间的 `Pending Input`；之后 `advance` 的 20 ms 物理步才消费 Pending 并更新 `Applied`。`Input` 没有 seq，投影还会清空 `Pending` 和 `Applied`，因此客户端看不到单独的 `appliedSeq`。

客户端也没有保留「序号／目标规则帧／输入」历史来回滚后重放。`TrackCanvas.tsx:101` 将当前输入直接用于权威时间之后的整个预测年龄。这使校正无法还原这段时间内先转向、再松开等输入变化；从校正到下一次绘制之间出现的新输入，也没有独立的目标帧约束。已有平滑和 250 ms 上限可限制显示误差，但不等同于规范要求的双确认和按序重放。最终正常／RTT 151 ms 场景的原生输入到像素结果见文末；这些通过结果不能补足缺失的协议语义，也不能仅凭源码缺口认定当前体验不流畅。

`LatestDrive` 对方向、油门、手刹、喇叭统一保留最新持有状态，没有独立的短按边沿队列。其现有测试覆盖「已发送按下后，释放不等旧 ACK」；完全发生在两次发送之间的短按是否需要可靠传达，仍须按动作语义另行约定，不能将现有窗口测试描述为所有短按边沿都已保留。

## 推进、发送和绘制是三个节拍

- 物理规则以 20 ms 子步推进，即 50 Hz；`State.ServerAt` 随子步前进。
- 赛车 `frameMS=100`，`SystemDeadline=ServerAt+100`。常规权威推进约为 10 Hz，每次可能补多个物理步；平台的 50 ms 唤醒不会将赛车规则自动改为 20 Hz。
- 每次成功接受 drive，以及每次 timer 推进，都会调用 `broadcastGameChange`。因此网络变更并不只来自 100 ms 的 timer。
- 每个 WebSocket 连接使用 50 ms 的推送闸，合并为当前最新 game 投影；常规 game 通道至多约 20 次／秒。ACK、network 和全量 state 是不同消息类型，全量生命周期刷新也不受这一 game 闸口限定。
- 本地绘制使用独立的 `requestAnimationFrame`，不会等待这 50／100 ms 的网络节拍。

以上是源码定义的节拍与上限，不是已测得的实际消息频率。引用位置：`games/racing/server/state.go:14`、`games/racing/server/state.go:210`、`games/racing/server/physics.go:273`、`internal/server/module_realtime.go:31`、`internal/server/server.go:2088`、`internal/server/realtime_stream.go:257`。

## 常规投影中发送的内容

`project` 发送当前车辆、圈数／名次、当前破坏位图、控制权限和时间信息。`compactTrack` 明确移除 `Points`、`Road`、`Barriers`、`Props`；车型、地形和美术数据从版本化资源加载，不重复进入普通赛车投影。赛道标识／几何摘要、尺寸、比赛选项等小型元数据仍每次重复。

存档专用的待执行输入、已应用输入对象、AI 驾驶内部状态和圈数拓扑跟踪器也从 View 中移除。不过投影复用整个 `Car` 类型，仍带有客户端驾驶与绘制未使用的 `idleAt`、`gate`、`checkpoint`、`lastReset` 等字段。宿主的 game 信封还会发送 `log` 和 `systemDeadline`。因此不能将它描述为只包含每帧必需字段的最小协议。

### 历史事件环仍重复发送

`State.emit` 将事件保存为最近 24 条；`project` 每次将 `s.Events` 全部放入 View。赛车注册没有 `StreamView`，所以宿主走普通 `View` 分支，没有为赛车保存连接级事件游标。这与「可靠连接只发送新增事件或少量有界冗余，不反复发送整个历史环」的要求存在明确偏差。

客户端使用事件 ID 去重，原生物件破坏位图和已开始的动画也各自保存状态，因此重复事件通常不会反复播放音效或重新结算破坏。直接后果主要是重复的网络字节、JSON 解码和扫描；不能据此推断音效必定重播。客户端去重不等同于网络上已采用事件增量。

已有证据可量化事件字段在投影中的比例：

| 已保留样本 | 事件数 | View JSON 字节 | `"events":…` 字段字节 | 字段比例 |
| --- | ---: | ---: | ---: | ---: |
| 六车碰撞压力测试的最终投影 | 24 | 7,061 | 2,267 | 32.11% |
| 六车物件碰撞压力测试的最终投影 | 24 | 7,473 | 2,315 | 30.98% |

上述值从 `.local/racing-nfs-backend/module/TestClassicCollisionLoadKeepsAllEventsWithinBudget.json` 与 `TestClassicPropSixCarDrivingPacketBudget.json` 的 `final` 对象，用 Node `JSON.stringify`／`Buffer.byteLength` 重新计算。它们是压力轨迹的最终投影，不是轨迹中最大消息。后者轨迹另有最大单条 View 7,595 字节的记录。

普通两车真实捕获中，Porsche 样本的 369 条 View、Corvette 独立样本的 379 条 View 保留 1～2 条事件；事件字段比例中位数约 4.1%，最大约 8.3%。来源分别为 `final-continuous-five-01/audio-continuous-output-p911.json` 和 `final-ccor-isolated-01/audio-continuous-output-ccor.json`。这些样本支持字段组成分析，不替代隔离网络性能场景。

## 单条预算、消息流量与实际传输带宽

现有 8 KiB 门槛约束 Go `Module.View` 的 JSON 序列化大小。7,595 字节是当前六车／24 事件压力轨迹的最大单条赛车 View，未包含宿主 game 信封、ACK、network 消息或连接恢复数据，不能直接视为单条完整 WebSocket 包，更不能视为 bytes/s。

旧浏览器工具 `states[].bytes` 的定义是 `Buffer.byteLength(JSON.stringify(view))`，也只是重新序列化 `game.data`。离线验收日志包含很多时刻的轨迹和诊断字段，其文件大小同样不等于游戏的一次同步或持续网络负载。

最终场景按以下互不替代的口径保存结果：

1. 原始 CDP `payloadData` 按 UTF-8 长度统计，上行／下行分开，按 input、ACK、game、state、network 等类型汇总。
2. 将初次连接／握手／bootstrap 与正式测量窗口分开，报告测量时长、总应用 payload 字节、每秒字节及峰值时间桶。
3. 同时报告实际状态到达间隔、RTT、ACK、原生输入到像素、显示排队和帧间隔；不以 ACK 或 FPS 代替操作响应。
4. raw application payload 不含 WebSocket 帧头／掩码、ping／pong、TLS、TCP／IP 和重传，不能命名为总传输带宽。

### WebSocket 压缩

实时入口 `internal/server/realtime_stream.go:181` 使用空 `websocket.AcceptOptions{}`。`go.mod` 固定 `github.com/coder/websocket v1.8.14`；实际本地依赖 `accept.go:55` 标明默认 `CompressionDisabled`，`compress.go:20` 将该值定义为零值，`accept.go:278` 在此模式直接跳过 permessage-deflate 选择。浏览器端使用原生 `new WebSocket`，没有应用层压缩封装。

因此当前源码路径未启用 permessage-deflate，即使浏览器握手提议该扩展，服务端默认也不协商。最终六车和双人 fixture 的实际 101 握手响应均只保留了 Upgrade／Connection 白名单字段，`extensions=null`、`perMessageDeflateNegotiated=false`，与源码结论一致。该实测不冒称为现网握手；启用压缩的取舍需要另测 CPU、内存和延迟，本次不变更模式。

## 慢端隔离、背压与恢复

| 已观察的边界 | 作用与限制 |
| --- | --- |
| 每连接 changed／frame／network 通知各有容量 1 | 积压时保留「有当前更新待发」的状态，不排队保存无限旧快照 |
| 每连接 ACK 队列容量 32 | 慢连接可阻塞自己的读／ACK 写循环，不需要等待其他玩家确认 |
| 每次 socket write 有 5 秒期限，网络写出位于平台锁外 | 慢读网络不会在写出期间持有平台锁；超时后取消并移除该连接订阅 |
| 每房间输入队列最多 64 条 outstanding | 超出返回 429；该上限按房间共享，不等于每个玩家均有独立输入额度 |
| 浏览器待确认与发送缓冲有上限 | 堵塞／超时会触发该端同步失败和输入释放，不自动补发旧按键 |
| 快照过期超过 1 秒 | 本地暂停控制，要求连接恢复后重新操作；此前 250 ms 后已经停止本人运动预测 |
| 失焦／隐藏／旋转 | 清空本地触点与持有输入，并尝试发送释放 |
| 服务端 750 ms 未取得有效应用输入 | 按赛车规则将该车转为保护制动、关闭喇叭；不把真人席位交给电脑 |
| 断流与重连 | 平台拒绝旧 pending Promise，改变 presentation epoch，重新取得权威 state；新连接不重播旧持有输入 |

慢端隔离还存在一个计算层面的边界：赛车没有 `StreamView`，普通 `moduleView` 的解码／投影仍在平台 `s.mu` 内执行。socket 网络写出虽然在锁外，但某次昂贵投影或规则推进仍可能延迟其他连接获取锁。平台已有慢 HTTP／WebSocket 读者 E2E，但合成模块的网络隔离测试不等同于当前六车赛车规则在负载下已完成验收。

真人断流后不立即在 socket teardown 中直接更改赛车输入，而是依赖客户端释放或 750 ms 的失联输入保护。平台对 realtime 清空回合超时和 TimeoutActors，不会因空闲／断线自动接管。手动托管和显式电脑仍按既有入口运行。

## 可采用的最小改进路径

这次用户接受混合模型，重点是实际联网体验。没有场景数据证明现有体验不流畅时，不应为日志体积误会立即迁移全量 input-only 协议。后续若需降低重复负载，可保留必要的权威车辆投影，单独采用平台现成的流式能力：

- `games/escape/server/module.go:183` 提供部分运动预测游戏的 `StreamView`／`SyncSnapshot=project` 示例；相同内容修订时省略静态集合。
- `games/contra/server/stream_view.go:125` 和 `games/yuyuhakusho/server/stream_view.go:16` 展示每连接游标、epoch／窗口缺口、明确 resync 及少量有界冗余。
- `games/pvz/server/module.go:286` 同时具备权威投影与可重演模式，可参考按能力选择数据形态。泡泡堂当前同样没有 `StreamView`，不能把它当成游标要求已经完整满足的例子。
- 宿主会在成功 socket write 后推进各连接游标；`StreamView` 在脱离房间对象后、平台锁外执行。注册它必须同时提供 `SyncSnapshot`，现有认证 `/api/rooms/{id}/sync` 可作为显式恢复入口。

赛车最小方案应分别处理事件游标、epoch／缺口恢复、客户端事件消费和权威运动投影。仅在服务器过滤旧事件还不够：React 可能将连续可靠消息合并到一次渲染，新事件必须在协议合并阶段保留，或采用经过测试的少量冗余，避免音效／撞物事件在渲染前丢失。该改进无需丢弃存档中的完整历史环，也无需改变胜负或物理模型。

已应用 seq／输入时序重放是另一个独立问题，需要先冻结失败场景和测试，再决定其实现范围。本轮只记录缺口，运行时代码和最终构建继续保持冻结。

## 最终隔离实测

最终 UI／联网构建的 `index.html` SHA-256 为 `4bac7f3b039b253cc9373f33d9bf65e6b359dd443aa78efab4ec07ddf2a36012`，构建身份文件 SHA-256 为 `267d769e5214854b5fe4cdd1c198c70781dee75e47820b0e0ee05d1be37fadc9`。全部长圈和其他浏览器验收结束后，`final-feedback-perf-03`、`final-latency-04` 顺序隔离执行，两份报告均通过并绑定该构建；没有将较早 27ddd412 或中间 87b510de 的性能数字改标为最终构建。

### 应用消息流量

下表直接统计原始 CDP WebSocket `payloadData`，KB 按 1000 bytes 计算。握手、首次资源加载和 bootstrap 在正式测量窗口之外；窗口、逐消息时间戳、字节数与类型保留在原生 JSON 的 `wsPayloadSummary`、`wsPayloadFrames` 中，可据此复算每秒时间桶。

| 实际场景 | 统计窗口 | 上行消息／字节 | 下行消息／字节 | 上行 B/s | 下行 KB/s | game 到达间隔 P95 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 两人正常连接，8 次原生转向试次 | 9.335 s | 20／5,708 | 124／262,981 | 611.46 | 28.17 | 117 ms |
| 两人 RTT 151 ms，8 次原生转向试次 | 9.245 s | 19／5,432 | 130／281,336 | 587.56 | 30.43 | 118 ms |
| 六车托管，PC | 6.012 s | 0／0 | 61／349,170 | 0 | 58.08 | 101 ms |
| 六车托管，320 px | 6.018 s | 0／0 | 61／348,773 | 0 | 57.95 | 101 ms |
| 健康玩家与 RTT 151 ms 慢连接并存 | 6.006 s | 0／0 | 62／161,292 | 0 | 26.86 | 102 ms |

正常连接上行全部为 20 条 input；下行为 100 条 game、20 条 ACK、4 条 network。慢连接上行全部为 19 条 input；下行为 107 条 game、19 条 ACK、4 条 network。六车 PC／320 px 窗口各为 60 条 game 加 1 条 network，实际约 10 Hz game 更新，符合这一场景的 100 ms 权威节拍。它们不代表前述 50 ms 推送闸已经改为固定 100 ms。

两人上行数字对应八次交替转向、释放及回正等待的实际操作模式，不是通用上行容量上限。六车窗口使用托管推进，没有手动上行，不能把上行 0 解读为不支持真人输入；另有同构建 `feedback-320-healthy-operation.json` 的真实触屏操作、上行消息及无同步告警的完整截图。

这些数字是应用 payload，**不是 TLS／TCP 总传输带宽**。即使实际握手未协商压缩，它们仍不含 WebSocket 帧头／掩码、ping／pong、TLS、TCP／IP 和重传开销；不能再将单条 7,595 B View 或验收日志文件大小代入计算场景流量。

### 可见响应与慢端隔离

| 验收项 | 最终实测 |
| --- | --- |
| 六车 PC 绘制 | 59.985 FPS，帧间隔 P95 18.1 ms |
| 六车 320 px 绘制 | 60.008 FPS，帧间隔 P95 18.4 ms |
| 正常连接，原生输入到首次真实车辆像素变化 | 8／8 可见，P95 78.11 ms |
| RTT 151 ms，原生输入到首次真实车辆像素变化 | 8／8 可见，P95 49.40 ms |
| 正常／慢连接输入 ACK | P95 27／173 ms，分别保留，不替代像素响应 |
| 健康玩家与慢连接并存 | 60.002 FPS，帧间隔 P95 18.1 ms，game 到达间隔 P95 102 ms |
| 实际 socket 中断与恢复 | 比赛 epoch、原期限保持；旧油门／喇叭释放，重复键不会恢复旧输入 |

最终像素 P95 均满足预先冻结的 80 ms 门槛。慢连接下 ACK 明显增加，而本地像素仍先响应，支持当前混合模型确实没有等待服务器 ACK 才呈现本人操作。慢连接场景的像素 P95 比正常场景小，是这两组有限试次的结果，不证明增加 RTT 能改善响应。网络延迟为有序 TCP 延迟和真实 socket 中断，没有将其描述为 IP 丢包验收。

历史红报告保留：`final-latency-01` 的旧 27 构建正常输入 P95 为 83.64 ms；`final-latency-02` 的中间 87 构建慢连接 P95 为 142.16 ms，其中一次发输入前已有 179 个变化像素。后续先确认真实渲染车轮回零并稳定至少 250 ms、6 次绘制，再发下一次输入；等待有界 5 秒，完整保留八次已发试次，像素判定公式和 80 ms 门槛均未改变。回正等待发生在输入之前，不计作输入响应时间；原失败没有删除或改写为通过。

### 报告与摘要

以下路径以仓库根目录为基准；原生 JSON 保留实际握手、消息逐条记录、状态到达与输入／像素工件。

| 证据 | SHA-256 |
| --- | --- |
| `.local/racing-nfs-browser/final-feedback-perf-03/report.json` | `cb89bd8fe6103352463dfcd057820f9be7f9a0cc63a2d7671a8150d3fdd0888b` |
| `.local/racing-nfs-browser/final-latency-04/report.json` | `897823a39cf533d554b240dab3c29a6b94147be087d11904cbb96a270e1a462f` |
| `docs/research/racing-nfs/driving-feedback-browser-validation.json` | `059906d82d073188a93d201915209f095f6ffc8e17d62daa0d90809417c308ab` |

性能原生文件为 `feedback-desktop-six-car-performance.json`、`feedback-320-six-car-performance.json`；延迟原生文件为 `pixel-response-normal.json`、`pixel-response-rtt150.json`、`healthy-peer-alongside-rtt150.json`、`reconnect-releases-stale-input.json`，分别位于上述两个运行目录。公开结果与历史限制见 [浏览器验收记录](DRIVING-FEEDBACK-BROWSER-RESULTS.md) 和 [机器摘要](driving-feedback-browser-validation.json)。

实测支持已覆盖场景的联网响应与慢端隔离通过，不消除本文记录的完整事件环重发、缺少 applied seq／输入历史重放、锁内投影等缺口，也不外推为所有网络条件、实体手机或现网链路均已验证。
