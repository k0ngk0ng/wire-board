# 实际乐谱包络接线的先行失败场景

- 从原 driver 的未提交快照读取旧 cursor／scale／flags，或在 consumer 再推进一次包络；必须读取当前 working transaction，原 driver 唯一推进状态。
- 只读 tick 查询返回可修改的共享 pointer；修改返回的 offset／level／fixed word 不得改变 catalog。
- 错误的 family、envelope ID、cursor、未绑定 work 或不匹配的 operation／voice 被接受。
- volume Begin 在 actual OpChipWrite 前输出；keyoff／hold／claimed 的 intent 数量错误，错误 owner／position／span／payload 或重复 intent 被消费。
- pitch terminal 被当成已完成 key-off 或 frequency write；calculator 的结束点必须保留现有声明边界。
- 局部阶段 clock 被当作完整 voice／tick，或清除其它未计费阶段的 unpriced 错误。

先用冻结 580 个正式逻辑步骤和原版 profile 编写实际 driver／transaction E2E，再实现查询和阶段接线。旧 State、cursor、声音 tick、fixture 与 golden 不改写。
