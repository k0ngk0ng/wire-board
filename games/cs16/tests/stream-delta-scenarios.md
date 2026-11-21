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
- 换弹前置条件通过正式鉴权 HTTP 读取；补弹、Ctrl+W、Ctrl+R 的效果仍等待真实 WebSocket 投影确认。HTTP 不填充 wire 观察器、不用于 ACK／规则应用时延及字节统计。
- 蹲下换弹必须实际进入 `reloadUntil > frame`，不以收到按键或本地动画替代服务端确认。
