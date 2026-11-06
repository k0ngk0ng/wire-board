# 完整新音符声部的先行失败场景

- 解析 caller、flow handler、参数读取、duration lookahead 和 early／final cleanup 的边界重叠，重复或漏算 return／backedge。
- 用已经推进的 cursor 选择本次资源，或按固定音高成本替代实际转调、除法和位移的成本。
- 新音符的 FM key-off／key-on、PSG rest key-off、volume／pitch terminal key-off 错序或重复，FM／PSG 后处理仍走 held-note 路径。
- 参数前缀、乐器查找、25 次写入、level carrier 更新或 pan 尾部重置预算，或在实际 chip intent 之前提前写入。
- QueueReplace 不在 Root 的唯一 effect lease 中执行，或 DAC feedback／publication 使用过期反馈、重复发布或错误声像。
- 当前 voice、owner、position、sequence、parser work、counter 和 intent 不匹配，阶段遗漏／重复后仍被接受。
- 外部错误或预算耗尽后提交 score，或只回滚局部 clock；联合事务失败必须丢弃已触及的外部 owners。
- 把新音符 body 的成功扩大为 track-end／ownership、完整 Root／world IRQ 或平台播放已完成；未实现入口继续显式报错。

本计划、原版整段调用 oracle 和 Go E2E 先于本项生产改动。ROM、原始 flow context 与 observer 仅用于开发对照，生产继续使用既有 typed score 状态与数据。
