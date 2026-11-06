# 音符收尾与颤音的先行失败场景

在实现时序前覆盖以下失败场景：

- FM／PSG 的 early／final cleanup 重复或缺少 return；tie 分支错误地重置 gate、包络、音高倍率或颤音定义。
- cleanup 的 modulation enable 分支与 held-note modulation 的 delay、rate、steps 分支混用；rate 为 0 时遗漏 8 位回绕，或在 rate 归零且 steps 为 0 时错误地累加 offset。
- 定义加载的字段读取、steps 除二、offset 清零成本遗漏；source 中固定移位与访问成本被整段估算，导致 refresh 边界错误。
- DAC final commit 的休止与有效采样路径混用；提前计入或执行 PCM 总线申请、优先级读取及发布；有效采样必须停在实际 publication 前。
- 使用原 RAM／寄存器／源 token 作为生产参数，或在 helper 新增一份持续计数器、相位和颤音状态。生产接口只消费唯一 score owner 的临时语义投影并返回 clock。
- 未知 family、与 family 冲突的字段、clock overflow、refresh overflow 或预算耗尽被接受，或错误返回部分有效结果。

固定 profile 从原 cleanup／modulation routine 或 DAC commit 声明入口开始，无额外 IRQ，不把私有入口证明当作自然可达性。先冻结原版 oracle 和 Go E2E，再实现；既有 fixture 与测试保持不变。
