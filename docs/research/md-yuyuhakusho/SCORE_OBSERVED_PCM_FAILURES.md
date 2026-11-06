# 乐谱现场 PCM 接线的先行失败场景

- 把持久声像与自动声像当次 YM 写入值混用；两者的左右声道可以不同。YM 应使用当前调用值，DAC 恢复声像仍取唯一 voice owner 的持久值，AMS／FMS 均不得丢失。
- 只传旧 Stereo 字段，或从上一拍／原 driver 快照猜测正在事务中变化的完整声像；callback 投影必须来自 working voice 的实际阶段，不能新增 State 字段或另一份 pan owner。
- 反馈正确但在回调里提前写入、重放返回的诊断 intents，或把本次未接受的请求发布；每个实际 OpChipWrite 只应用一次。
- 消费错误 voice、event position、intent offset、order、kind 或 payload；复用旧阶段、提前 Finish、重复 Consume 或错误后继续均须拒绝。
- 初始乐器 pan 尾部与普通 voice pan 的值来源混用；instrument tail 不能用外部 argument 覆盖其现场字段读取。
- 外部失败导致 score／mixed 中一方提交；E2E 必须通过 detached branches 共同提交，失败保留原 owner。
- 用局部阶段时刻冒充整拍时刻。接线只覆盖声明的 feedback／write／return 区间，尚未计费的控制参数、包络、world／Root 不得记零或对玩家启用整场声音。

先冻结补充原版对照和实际 score-to-mixed E2E，再修改实现。保留旧 Stereo／intents／State／source fixture，新增只读临时投影兼容已有消费者。
