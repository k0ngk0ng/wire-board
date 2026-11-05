# 《魔强统一战》逐连接增量输入

本次只增加公共 `StreamView` 接口、独立连接游标和本作的输入投影。规则、呈现、控制器、资源身份和 `/sync` opaque checkpoint 保持原有实现；此前 263 文件来源清单及完整 AI 验收单独保留，本次不重跑 32 场 AI 矩阵。

## 参考与边界

只读对照 Main 的 `internal/gameapi/module.go`、`internal/server/realtime_stream.go`、`realtime_cursor_e2e_test.go`，以及篮球、泡泡堂、魂斗罗的同步实现。篮球和魂斗罗支持确定性输入重演；泡泡堂的低频权威投影配本地运动预测用于模型比较。本作沿用自身的确定性重演，不搬入其他游戏的规则或字段。

公共改动限定在当前 worktree 的 `internal/gameapi/module.go` 与 `internal/server/realtime_stream.go`。只移植所需 API 和连接游标；未整体复制 Main 的模块文件，也未写入 Main。当前 worktree 没有 Main 的 `moduleViewData`，因此消息包装 helper 放在本次独占的流文件内，按既有 wrapper 保留公共字段。未采用 `StreamView` 的模块继续走原回调路径。

## 协议

`Module.StreamView(match,viewer,cursor)` 收到脱离 room 的 Match，游戏投影在平台锁外执行。连接保存 `(roomID,viewer,opaqueCursor)`；room 或 viewer 变化时不传旧游标，游戏再用 epoch 检查比赛身份。新连接始终从空游标开始。

本作 cursor 为 `epoch:frame`，帧号使用规范十进制 `uint64` 字符串。常规 WS 仍使用原 View 顶层格式和 `syncwire.InputWindow`，但只编码该连接最后成功发送之后的新增输入，附最多 6 帧已发冗余。相同帧返回规范空窗口。常规发送不含 world、HUD、cache、存档或客户端素材。

无效、非规范、未来、旧 epoch 或已落出 120 帧恢复环的 cursor 返回 `resync:true` 和空输入窗口。窗口恰好左边界可正常追赶。`resync` 要求客户端拥有对应有效权威状态；已经由 `/sync` 验证的相同 epoch／帧／双摘要可视为已履行，不必重复下载。新帧或摘要不符则按需恢复，不能猜测缺失输入。

每条连接只有一个待发送帧通知，实际 Socket 写入有独立超时。只有成功写入完整消息后才推进该连接游标，不等其他玩家的 ACK。120 帧持久恢复环独立保留，普通 HTTP View 和 `/sync` 的行为不因裁剪发送窗口而改变。

Match 在锁内逐字段复制全部 slice 和 Result 引用，保留 nil 与显式空切片的区别；不在锁内 JSON 编码整份 opaque 游戏数据。公共信封与 Match 来自同一 room version。信封快照完成后释放锁，再做规则解码、摘要、投影和 Socket 写入。

## 先行测试与修订来源

`STREAM_CURSOR_FAILURES.md` 先于实现，`platform-stream-cursor-test-first.json` 保存初始测试／失败清单 SHA、Main 只读来源 SHA，以及真实缺少 `StreamView` API 的红灯日志。旧 263 来源清单未覆盖或改写。

测试夹具的修订均保留前版源码、SHA 和失败记录：game 的字符串 `status` 与 ACK 的数字状态需分别解析；ACK 在专用、有界接收队列的实际 Read 时记录，规则重演另行执行；冗余上界相对本连接上一 WS 帧计算，不能用可能经 `/sync` 前移的 client confirmed 代替。

性能与竞态范围按 Root 的明确要求拆分，见 `STREAM_RACE_SCOPE.md`。原 `TestCursorHTTPWSReplaySizesAndReconnect` 的 native 500 ms 阈值不变；该原性能用例在 `-race` 下约 5 秒的失败完整保留，不宣称通过。新增 `TestCursorHTTPWSFunctionalRace` 覆盖相同实际 HTTP／WS、300 帧、60 次动作／确认、完整规则与呈现双摘要、2048 字节视图、独立快慢连接及重连；插桩耗时不作为 SLA。

## 重复执行

在 worktree 根目录运行：

```sh
python3 scripts/yuyuhakusho/verify_stream_cursor.py native
python3 scripts/yuyuhakusho/verify_stream_cursor.py race
python3 scripts/yuyuhakusho/verify_stream_cursor.py wasm
python3 scripts/yuyuhakusho/verify_stream_cursor.py vet
```

原生与 race 选择器分别明确选择性能用例或功能用例。另覆盖公共 cursor 注册、锁外 detached 投影、同版本信封、room／viewer 重置、4 MiB 阻塞慢连接、既有实时流鉴权、普通 View 和 `/sync` 恢复。Wasm 只运行投影、64 位边界及旧普通 View 检查，不运行服务器网络或完整 AI 矩阵。

脚本记录源码依赖清单、冻结测试 SHA、命令、环境、原始输出、网络证据及执行前后源码一致性。所有 Go 缓存、临时文件和遥测目录限定在 worktree 的 `.local/`，不调用全局 Go 配置命令。

## 已测原生结果

真实 WS 房间重演 300 帧，60 次密集输入／60 次确认全部完成，逐个权威边界的完整规则和呈现摘要一致。常规游戏 View 峰值为 997 字节；20 个空闲消息共 27014 字节，80 个密集消息共 111139 字节，消息字节包含平台 JSON 信封。最大观测输入 ACK 为 5.187 ms，满足冻结的 native 500 ms 阈值。它是此本机测试的输入确认观测值，不能替代连接 RTT 或事件到像素的响应测量。

另一条实际 spectator WS 停止读取，快端仍正常输入、重放和恢复。公共 fixture 另以 4 MiB 消息及小 TCP 发送缓冲制造实际写入积压，验证独立连接不会共享推进游标或阻塞快端。重连得到新的 `resync`，不重开加载门槛、不启用托管，原 opaque checkpoint 可恢复。

最终 native 为 12 个顶层测试、3 个人数子场景通过；明确选定的功能 race 同样 12＋3 通过；Wasm 投影和旧 View 边界为 3＋3 通过，`go vet` 通过。四项执行前后源码和冻结文件均一致。功能 race 的 300 帧／60 次确认及双摘要检查通过，其输入确认观测值为 3691.488 ms，不声明满足性能 SLA。原性能用例的 race 红灯没有覆盖或删除。

最终机器可读清单为 `games/yuyuhakusho/verification/platform-stream-cursor-final-validation.json`；原始日志、命令、逐连接网络结果和失败来源均在同目录保留。清理记录为 `platform-stream-cursor-cleanup.json`，只删除本范围临时 profile 可执行文件／原始 profile、未采用的摘要和已归档临时网络结果，共约 47 MB；共享缓存未清理，Main 未写入。

本记录不代替 Root 的正式浏览器正常网络／120 ms 慢端、原生输入到像素和帧率验收，也不代表上传、发布或部署已完成。
