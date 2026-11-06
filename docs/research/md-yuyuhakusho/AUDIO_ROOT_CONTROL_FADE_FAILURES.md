# Fade counter 的先行失败场景

本清单先于本轮 fade 实现与测试。生产只增加 `FadeState{Steps,Remaining}` 与纯决策，不增加完整 Root.State、不复制 score、queue、pause、Scheduler、PCM、voice、duck 或 pending TL payload，不执行新的原 CPU 捕获。

- 将 01／05 的初始 remaining 混同：01 为 3、05 为 32；颠倒原 remaining→steps 两笔写入或漏掉随后 StopDACFlags action。
- 将未知命令、04、音乐或 PCM 请求静默当作 fade-start；这些输入必须明确 Unsupported。
- Steps 为 0 时仍递减或清 Remaining；原函数先判 Steps，必须保留非零 Remaining。
- Remaining 从 1 递减为 0 的同一次调用提前 fade／reset；原函数这次只减 Remaining 后返回。
- Remaining 原为 0 时漏减 Steps；非末步遗漏 Remaining=3，或再次错误使用初始 32。
- Steps 从 1 变 0 时仍重装 Remaining 或发 FadeMusicStep；应只写 Steps=0 后请求 full clear，不自行执行 04。
- 从完成 04 后的 frame 全清结果生成 terminal 期望，误把 Priority、Selected、ChoiceBits、queue／pause／voices 的归零纳入 counter helper。
- 把临时 Writes／Action 持久保存成 replay history，或在 FadeState 之外再保存另一份 Steps／Remaining。
- 使用 Go／Python 规则模型计算 golden；本轮的 before／after、start 值与 branch route 必须来自 canonical-v2 实际事件与 frame。
- 原多 tick 流被拆成彼此无关的单步，漏检 40／50 步倒数、跨 frame carry、pause-hold 不进入 fade，以及序列化恢复后的延续。
- 试图在本 helper 中调用／重建 score 或清除 DAC 参数；它只输出 StopDACFlags／FadeMusicStep／RequestFullClear，真实 owner 动作由 orchestrator 在有序 writes 后执行。

原 native 数据为 canonical-v2 的 5 条流、785 个原／观察完全相同的 frame；本次只读提炼。测试内容先冻结，通过 Go overlay 得到真实缺 API red，再实现并运行原生竞态／Wasm 与可重复检查。
