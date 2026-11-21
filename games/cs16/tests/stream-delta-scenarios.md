# 公开增量投影的失败场景

实现前固定以下边界，最终仍以正式 HTTP／WebSocket 浏览器验收记录为准。

- 首次连接、重连、基线窗口外和旁观使用完整 schema 1；schema 2 无完整基线不得解码。
- epoch、baseFrame、baseEventId 任一不符，或新帧／事件游标回退，只软关闭当前 WebSocket，等待完整恢复；不通过 HTTP 发送控制，不重建同对局已就绪画布。
- 同一帧可有新的输入收到确认；事件 ID 可因席位可见性跳号，不能猜补隐私事件。
- 对象字段可递归设置、删除；数组整换。缺失对象、未知实体 patch、冲突字段、重复 ID、非法类型及原型键必须拒绝，不能污染基线或全局对象。
- 玩家生命或 visible 改变使用 full 替换，隐藏时旧坐标、武器和库存必须清除；实体 remove 后不得保留旧模型。
- 增量 events 只包含新增可见事件，恢复标记在正常增量后清除；不能复制历史事件或重复播放声音。
- 先执行既有 room／setupVersion／serverInstance／version 守卫，旧房间、旧服务或重复包不得触发游戏 decoder，更不得断开当前有效连接。
- main 以 actionState 同步解码一次，将同一完整 packet 交给动作作用域及 React 状态；异步 setState 不得二次消费 patch。
- eager catalog 的回调不依赖 Three.js、React 运行时或地图资源，首次游戏模块尚未加载也能保存完整基线。
- CDP 按实际 socket 收到时刻读取 self patch 的 appliedSeq，记录原始 wire 字节；不得用重组后长度或 rAF 才见到的应用序号代替网络测量。

Node 边界、生产多浏览器、断线恢复与快慢端结果统一保存在 `.local/cs16/verification/public-delta/` 及正式浏览器报告目录。

## 晚连接验收观察器

- CDP 晚于游戏连接启用时，未出现在实际增量中的字段保持未知；未知 `reserve` 不能当作已有备弹，也不能跳过换弹用例的补弹准备。
- 当前换弹前置条件与补弹、Ctrl+W、Ctrl+R 的效果均读取完整 raw WebSocket 投影。HTTP 不填充 wire 观察器、不用于 ACK／规则应用时延及字节统计。
- 蹲下换弹必须实际进入 `reloadUntil > frame`，不以收到按键或本地动画替代服务端确认。

## 原始连接基线与受控重连

- 正式 `six-map-release-final` 的 Dust2 慢端发生 RED：`knife` 序号 533 与 `usp` 序号 535 均按序收到 200 ACK，实际规则应用序号达到 575，但晚附着观察器的 `partial=true` 且始终没有 `weapon` 字段。快速切换在投影间隔内回到原武器时，合法增量可以不发送该字段。失败原文、像素样本和原始时序保留在 `.local/cs16/browser/reports/six-map-release-final/`。
- 普通输入测试必须先启用 CDP Network 监听并绑定账号身份，再通过本端测试 TCP 代理受控断开／重连，等待实际 WebSocket 的 `schema=1` 完整基线。基线准备阶段的 HTTP 仅提供身份，不能填充观察器的玩家字段。
- 网络 control 路径必须位于 `.local/cs16/`，文件名限 `network-main.json`、`network-peer.json` 或 `network-watch.json`，拒绝符号链接、无效 JSON、非法 RTT／抖动和已经离线的配置。不得修改其他客户端文件。
- `offline=true` 仅用于等待真实 socket 关闭；随后原样恢复 control 文件字节，包括 RTT、抖动和额外字段。关闭等待失败、基线等待失败或驱动退出时仍须恢复。Python 外层也须在子进程失败时恢复本端文件。
- 完整基线未出现、原画布被替换、epoch 改变或控制未恢复时必须失败，不能放宽原武器和应用确认。记录完整基线原文、字节数、SHA-256 与 socket 生命周期。
- 像素探针仅在重连和控制恢复后启动。此时以实际 raw 完整状态建立 `measurementStartIdentity`；连接准备期间的自然换命不能被误认作采样期间死亡重试的依据。
- 购买滚动、后台释放和只观察连接恢复的专用分支不增加准备性断线。保留所有实际 raw 字节并单独标记准备阶段，不从测得的输入、帧或 ACK 延迟中减去重连时间。
- 首次受控重连定向已取得新连接的完整基线，但在 `knife` 出站约 10.5 ms 后立即读取选择数组，Q 仍可能位于有界发送队列，测试便提前退出。输入通道先更新本地选择再按在途窗口发送；本地画面恢复不等于已经出站。驱动只等待两条真实选择出站；若先观察到第一条选择的 ACK 仍立即失败。随后继续检查两条选择按序接受、Q 出站早于刀的 ACK，以及实际 raw 权威武器和应用序号，不能以本地选择替代。
