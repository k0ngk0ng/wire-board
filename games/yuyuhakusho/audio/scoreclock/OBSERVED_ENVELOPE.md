# 当前事务的包络阶段连接

`Catalog.PitchEnvelopeStep`／`VolumeEnvelopeStep` 返回现有逻辑 cursor 对应的语义 tick 与 bound work，不推进 cursor。返回的 offset、level 和 fixed word pointer 均为独立值；调用方不能修改 immutable catalog。未绑定、未知位置和已结束位置沿用既有拒绝语义。

`PriceObservedPitchCalculation` 在实际 pitch resource operation 处使用当前 working voice 的 ID、cursor、scale 与 modulation enable 查询资源，并进入既有 calculator 计时。它不计算第二份状态，不完成 terminal key-off／frequency caller，也不把外层 guard 和整拍计费置零。

`BeginObservedEnvelopeVolume` 在实际 volume body 入口使用当前事务快照启动既有分阶段 volume。只有对应 `OpChipWrite` 到达才调用 Consume，匹配 voice、event position、intent span 和完整 PSG 字节；hold／suppression 不消费写入。Finish 消费唯一 return。调用方仍负责 held-note caller 及其它阶段的成本和 detached 共同提交。

原 score driver 继续执行唯一 `Envelope.Next` 并推进 cursor／scale／note flag。E2E 对全部 374 个 volume 步骤与 206 个 pitch 步骤使用实际 driver／transaction，阶段结束时刻对照已冻结的原版，完整输出与 State 对比 legacy 同输入路径。只读查询额外验证全部 580 步无 pointer 泄漏。

验收与复跑入口见 [score-observed-envelope](../../verification/score-observed-envelope/)。完整 Root／world 时间线和平台整场音频仍待完成。
