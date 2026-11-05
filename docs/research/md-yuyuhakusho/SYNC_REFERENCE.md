# 三款即时游戏的同步机制对照

本次只读对照主工作区提交 `bc9d0d49708b0b8b735cd8034bc98c26d55fcc0b` 的源码。文件当时均无本地修改，来源 SHA 见 [source-manifest.json](../../../games/yuyuhakusho/verification/sync-reference/source-manifest.json)。没有重新运行其他游戏的整局测试，也不把用户感受到的流畅度改写成新的性能测量。

| 游戏 | 实际同步方式 | 可复用经验 |
| --- | --- | --- |
| 热血篮球 | 普通视图不序列化完整 Runtime；发送有界编码输入、帧号与摘要，显式 `/sync` 提供 checkpoint。客户端维护权威与预测分支，并从未确认输入恢复预测 | 网络存档分离；Wasm 紧凑回复只在 `Verify` 时生成摘要；重放可以不捕获无需呈现的中间画面 |
| 泡泡堂 | 约 100 ms 的权威投影含玩家、格子、炸弹及物品；客户端按渲染时钟预测本人的运动，误差逐渐收敛，部分特殊地形采用权威位置 | 低网络频率不等于低显示帧率；方向与放置炸弹的短按分开处理；预测时长有界。它不是只同步按键的实现 |
| 魂斗罗 | 有界规则记录保存帧／修订、双人按键和摘要。普通连接按自己的修订游标选取新增记录，并附六条冗余；缺口要求恢复。完整 checkpoint 与普通消息分离 | 逐连接游标减少重复发送；帧号使用十进制字符串；预测分支不改权威、声音与结算；运行中慢端不重新阻塞其他玩家 |

主要来源为篮球的 `server/wire_view.go`、`web/network-scene.ts` 与 `runtimebridge/compact.go`，泡泡堂的 `server/state.go`、`web/input.ts` 与 `web/prediction.ts`，魂斗罗的 `server/stream_view.go`、`web/runtime-prediction.ts` 与 `web/runtime-room.ts`。这些文件位于主工作区；当前《魔强统一战》worktree 的基线早于魂斗罗接入，不能声称直接运行了该 worktree 中不存在的游戏。

## 《魔强统一战》的取舍

本作规则能在 Go 与 Wasm 确定性重演，因此普通消息使用 12-bit 按键、帧号、序号、托管标记与摘要，完整世界和共享呈现状态仅由显式恢复接口提供。当前 120 帧 RLE 输入窗口有界；本轮没有把魂斗罗较新的公共 `StreamView` 游标协议整批搬入旧平台。逐连接增量仍可进一步减少密集输入时的重复负载，须在独立协议变更中先保留失败证据并验证丢帧恢复，不能把这个优化写成已经实现。

真实双人页面的首轮约 9 帧／秒，并非逐帧传输完整世界。问题来自客户端同步处理：每次分叉做完整 JSON 往返，权威重放绘制未显示的中间像素，并为不消费摘要的本地操作重复计算摘要。Wasm 基准中 JSON 分叉约 13.9 ms，直接复制状态后约 0.095 ms；共享规则每步仍完整运行，权威摘要校验与回调时刻的呈现语义保留。

实际页面的 [performance-red01](../../../games/yuyuhakusho/verification/platform-app-browser/performance-red01/result.json) 固定了 8.80 次可见更新／秒和 119.1 ms 帧间隔 p95。每端至少 50 次更新／秒、p95 不超过 40 ms 的预算先于优化冻结；正式双人结果独立验收，不以单页面夹具替代。

随后还复现了慢端用绝对消息 age 填满预测上限的问题；对照篮球改为本地固定步进、确认后恢复旧预测目标，同时修复同一次 RAF 前的新按键不能回填旧显示帧的边界。最终 [native-clock-network04](../../../games/yuyuhakusho/verification/platform-app-browser/native-clock-network04/result.json) 正常两端 59.94／59.93 次／秒，120 ms TCP 写入延迟的慢端 55.91 次／秒、p95 31.7 ms；慢端原生按键 27.8 ms 响应，先于 226.4 ms ACK。预测上限仍为 12 帧，未通过扩大缓冲掩盖积压。

## 回馈到规范

已把模型选择、时钟分离、输入边沿、连接游标、高频状态复制、按需绘图、双加载门槛和性能测量口径补入主工作区的 `rules/realtime.md`。原文件字节保留在 [rules-realtime.before.md.txt](../../../games/yuyuhakusho/verification/sync-reference/rules-realtime.before.md.txt)。规则变更只覆盖即时游戏专题，不修改三款参考游戏或主工作区的其他工作。
