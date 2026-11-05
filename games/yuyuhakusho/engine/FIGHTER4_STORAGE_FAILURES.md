# 飞影状态通道失败场景

以下清单与 `fighter4_storage_test.go` 先于共享生产字段修改。

- 父体 `+90` 的移动参数来源在 world、actor、action 之间丢失，或写回时保留旧字节掩盖遗漏。
- 三个子对象的 `+48` 完整 long 被截成高 word；低输入 byte 写回覆盖扩展输入 byte。
- 子对象 `+FE` 借用其他字段，或零值无法覆盖旧 bank。
- actor/action 之间丢失全局 `A038`；从先前测试原始值补回，造成假通过。
- 合并 view 改写原 world map，或覆盖无权修改的子对象字段。
- 全对象清除遗漏 `+FE`，或较短前缀清除过早擦除它。

本轮先验证类型和往返通道，保持 source4 的完整 action/input/actor 入口为 unsupported。原 CPU 自然与单初态组合测试由独立 scope 冻结；它们需在开放规则之前产生真实运行失败。
