# 正常控制参数与真实队列写入

`PriceControlWork` 使用临时 `ControlParameterWork`，覆盖正式普通控制形状和声明的 continuation：pan、detune、pan automation、level-add、tie、gate、tempo、queue replacement、instrument selection、modulation、noise、envelope selection、transpose 和 modulation enable。它不执行源 opcode、原指针／RAM 或另一个计数器 owner。

返回 `Next=complete` 时已包含 handler return；`pan`、`level`、`instrument` 停在对应已有 leaf 之前，不重复其 guard、寄存器写入或 return。`psg-noise` 停在实际端口 store 之前，`queue` 停在共享 Root queue store 之前。parser caller／dispatch／backedge 不包含在本接口中。未知控制和 track-end 等其它 body 仍明确拒绝，正式 parser resource 的 unpriced 标记保持不变。

`PriceObservedControl` 直接消费当前实际 `OpParameter/RoutineControl`。新 `ControlProjection` 只携带 immutable typed event 的 ImmediateFM、pan disabled、enable 和 noise latch；唯一 voice／State 与旧 intents 不变。正式数据中的 FM level-add 进入即时 level leaf，PSG level-add 仅更新保存值；不根据 action 名推测原控制编码。

`ServeNoiseControl` 在同一 clock／refresh／预算内完成 noise handler。claimed 只返回，未 claimed 才在实际写入点调用唯一 PSG owner；该 store 包含 operand 读取，为 24 CPU cycles，没有 YM／ZRAM 额外访问。它与 prefix-only 的价格是同一内部实现，调用方不能先完整计费又重复调用完整服务。

共享队列写入由 `ServeQueueControl` 的 effect 回调包住：原 16-cycle store 的 refresh 后访问点调用一次实际 owner，随后消费唯一 return。`RuleHooks.QueueReplace` 允许 Root 把该 canonical effect 交给计时回调，默认消费者仍沿用原写入顺序。Root 检查 effect 同步调用一次；遗漏、重复、外部失败和过期调用均拒绝，不能在外层回调和 Root 各写一遍。

这些回调运行于 detached branches。任何失败后调用方须丢弃所有相关 owner；本项不承诺自动回滚外部字段，不实现世界 IRQ 或联合 checkpoint，也不能将 handler 边界当作整拍已计费。实际平台整场音频仍有后续工作。

原版 208 个声明 profile、两份既有 core、832 次执行覆盖完整 pure handler、精确 continuation 和 noise／queue effect。实际 E2E 另覆盖正式 25 种控制形状的现场投影、真实 Root queue 写入及错误生命周期。冻结证据、API 命名修正和复跑入口见 [score-clock-control](../../verification/score-clock-control/)。
