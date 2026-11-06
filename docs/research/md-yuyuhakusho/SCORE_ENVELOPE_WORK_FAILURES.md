# 包络工作量与计时的先行失败场景

- 已展开的声音结果相同，但原始 restart／repeat／retarget／scale 路径不同；不能仅按声音 tick 猜测工作量。
- prefix 到 cycle、cycle 回绕和允许的 prefix-end 游标别名映射错误，新增原始游标或另一份动态包络状态。
- 把结构性工作量变成可执行程序；资源只保留有限导航类别，不保留原 byte、目标、指针、PC、原索引、RAM 或持续事件历史。
- pitch 负 offset、scale 的 8 位回绕／16 位符号扩展、MULU 动态周期和内部 refresh 顺序错误；terminal keyoff 必须保留已恢复的 131 bypass 语义。
- volume hold／重复 keyoff 与正常 level 混用，遗漏 8 位加法回绕、clamp、note-off／claimed／tie-gate 抑制和当前 noise channel 的端口值。
- 工作资源未绑定准确 catalog，缺项、重复、未知类别、超限或错误 cursor 被接受；五资源 identity／checkpoint 允许混用。
- 原版私有 leaf、资源绑定或局部计时通过便宣称完整世界／Root／平台音频完成；未实现阶段继续明确 unpriced。

先冻结来源／工作资源对照和 Go E2E，再接入生产实现。现有 State、包络 cursor、声音 tick、旧 fixture 与 golden 保持原归属。
