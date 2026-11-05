# 树的跨人物停顿通道失败场景

以下清单和 `fighter7_storage_test.go` 先于跨人物临时通道实现。

- Actor→Action 丢失 peer `+69`，或使用 `+84` 的 StunTicks 代替。
- 停顿与能量共用 peer 身份却覆盖另一资源，或自身在 map 和 Fighter 重复拥有停顿。
- 空席位／已死亡的 peer 被过滤，导致原版 Team 遍历写入丢失。
- Action 返回后保留旧停顿或在输入 map 上原地写入，破坏错误原子性。
- World 未把 actor 的 peer 字节立即合回对应对象，导致同轮后续角色错误推进。
- 编码器依赖 previous raw bytes 隐藏 `+69` 遗漏，或改变其他三父体的未拥有字段。

持久化仍由每个 WorldObject.HitStopTicks 单独拥有。新增 ActorWorld.OtherHitStopTicks 和 TailFighterEnergy.HitStopTicks 仅是另三个固定父体的临时视图，自身只用原 Fighter/Action.HitStopTicks。
