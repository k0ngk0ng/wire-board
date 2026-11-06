# 实际声部入口与终止恢复

`BeginVoice` 接收实际 voice enter 与 Root 当前 score transaction 的 working State，按 NoteWait 复用 held-note／新音符子服务，并处理有限音轨终止。每个真实操作及当时 working State 都交给 `Consume`，实际 voice leave 后执行 `Finish`。全部阶段共用一个临时 clock、refresh 与操作预算。

Root 的 `WorkingScore` hook 提供当前事务值快照，避免从权威 driver 的事务前 Snapshot 读取过期 sibling 数据。跨声部恢复保留结束事件的 position，同时分别验证结束声部和目标声部的 owner／channel／flags；不增加另一套动态 voice 状态。

普通音乐声部停止只完成 key-off 与本声部返回。Primary 终止先执行 key-off，再通过 Root 的唯一 `PriorityClear` effect lease 清除优先级，最后恢复对应音乐声部。FM inactive target 保持原状态；active target 清除 claimed、设置 forced rest，并从原版 seek 中段进入乐器传送，跳过普通 claimed／bank 前缀。PSG target 无论 active 与否都恢复 flags，noise target 在实际意图处重新写入 noise latch。

资源选择按实际席位确定：原版 primary 扫描使用负 domain 标志和 voice-local bank。既有隔离 API 的 route 标签按历史采样条件保留，统一入口显式选择实际分支，不将隔离标签当成席位规则。恢复已经选好音乐 bank，复用同一个 bounded seek、25 次写入和 pan tail。

唯一 queue-replace 控制位于有限音轨终止前。`Consume` 只准备这次效果，Root 的 `QueueReplace` hook 必须调用同一个 transfer 的 `QueueReplace(op, apply)`；priority 同样由 `PriorityClear(op, apply)` 完成。两个 `apply` 都必须在原生 store 时刻同步执行一次。不得在普通操作回调和 effect lease 中各执行一次，也不得保存回调后补执行。

`EndsDriverTick` 为 DAC 终止提供明确语义：原版会跳过整个 driver 的剩余扫描和最终 return。普通声部终止只退出本声部，外层仍需继续。这里仅计固定的跳过成本和 return，不建立原 CPU stack。

当前范围仍不包括 active secondary layer、尚未证明的 owned FM6、完整 driver 外层扫描／Root housekeeping、world IRQ、联合 checkpoint 和平台播放。没有整段计时的路径继续 unavailable，不能据本项移除全局未计时错误。失败后必须丢弃已触及的 detached 外部 owners，接口不自动 flush。

原版、真实 Root effect 与复跑记录见 [score-clock-track-end](../../verification/score-clock-track-end/)。
