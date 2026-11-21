# 公共 WS 完整状态与增量基线失败场景

先编写真实 HTTP／SQLite／WebSocket E2E，再修改公共 streamSocket 的连接游标。测试只注册宿主契约计数模块，不执行其他游戏。普通 View 返回完整数据；可选 StreamView 用空游标返回 full、非空游标返回 delta。

| 场景 | 预期 |
| --- | --- |
| 初始 state 后第一次 game | 以空游标请求 full，建立本连接基线 |
| 连续正常 game，中间读取 HTTP state | 保留本 WS 的增量游标，HTTP 读取不修改它 |
| changed 发送完整 state，例如主动托管与手动接管 | 完整 state 成功写入后清空本连接游标；下一 game 以空游标发 full，再恢复增量 |
| pushFrame 遇到 packet 为 nil，回退完整 state | 与 changed 路径相同地重置游标 |
| 非读取观众阻塞完整 state 后断开，导致写失败 | 失败连接结束，不执行之后排队的投影；不能污染正常玩家或重连的新连接 |
| 其他连接仍在发送正常输入 | ACK 与增量继续推进；慢端自己的完整状态不能清除快端游标 |
| 未声明 StreamView 的旧模块 | 继续使用普通 View，完整 state 与 game 行为保持兼容 |

生产修改只允许在完整 state 成功 write 后清游标，普通 game 仍只在成功 write 后提交 nextCursor。失败连接的局部游标无法由网络继续观测，因此同时通过源码顺序复核和断开／重连／跨连接 E2E 验证不污染后续流。

产物和复跑缓存位于 `.local/cs16/verification/stream-baseline/` 与 `.local/cs16/`。不使用 ACK 证明客户端已经应用业务状态。
