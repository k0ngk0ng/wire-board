# 仙水状态通道失败场景

以下场景及 `fighter5_storage_test.go` 先于 source5 world view／merge 实现。

- 三子对象 `+4c` 最高 byte 更新时丢失原低 24 bits，或零值无法覆盖旧 Gravity。
- `+fe` 的 parent owner 与飞影 history bank 在同一临时 view 中重复持有。
- Source5 actor/action 传递丢失三个子对象的 Gravity／Owner；使用先前原始字节掩盖遗漏。
- 按来源合并时误写其他子对象字段，或修改输入 world map。
- 新 view 污染其他九个已开放人物的状态或存档。

WorldObject 保留 `DragonBank` 这一历史存档字段名，它是唯一 physical `+fe.word` owner。来源 4 将它解释为 bank，来源 5 的临时 `Owner` 映射同一个真实 word，不新增第二个存储字段。Source5 的 `ChildExtras.DragonBank` 必须为零，由 `Fighter5ChildExtras.Owner` 单独提供该次回调的语义。
