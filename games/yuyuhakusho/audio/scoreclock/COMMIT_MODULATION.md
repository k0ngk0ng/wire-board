# 音符收尾与颤音计时

`PriceNoteCommit` 消费唯一 score voice 的临时语义投影，覆盖 FM／PSG 共用的 early／final cleanup。它包含逻辑读位置保存、note wait 重载、tie 判断、gate 重载、两类包络游标与 pitch scale 清零，以及启用时的颤音定义重载和最终 return。tie 分支保留已有 gate、包络和颤音状态，计时接口不修改这些字段。

DAC commit 使用相同接口的独立 family。休止包含最终 return；有效采样完成不可变 descriptor 解析后停在 saved-SR／总线申请之前，并返回 `NeedsDACPublication=true`。该标志不表示总线申请、当前 marker／priority 读取、元数据发布或后续 return 已计费。

`PriceModulationWork` 覆盖 held-note 的完整颤音 leaf：disabled、delay 递减、rate 回绕／等待、rate 到期后 reload，以及 steps 为零时反转 delta 或非零时递减 steps 并累加 offset。只有 rate 的当前值为 1 才到期；0 会按 8 位计数器回绕。反转分支不会在同次调用累加 offset。

两者逐个固定操作更新同一临时 clock、refresh deadline 和预算。它们不读取源 token、PC 或 RAM，不保存定义副本、持续计数器、程序或事件历史；现有 score driver 继续唯一持有并推进所有规则状态。调用方在实际阶段传入当前投影，不能从上一拍缓存。调用方的 JSR／分支、后续 pitch 写入和世界 IRQ 不属于这两个 leaf。

先行失败场景、冻结测试和原版证据见 [score-clock-commit-modulation](../../verification/score-clock-commit-modulation/)。原版矩阵包含 188 个无额外 IRQ 的声明 profile、两份既有 pinned core、752 次执行；被动记录前后的完整 native 状态、RAM 和步骤一致，两份 core 的步骤、寄存器、RAM 及结束时刻一致。该证据不替代完整 driver／世界时序与平台整场音频验收。
