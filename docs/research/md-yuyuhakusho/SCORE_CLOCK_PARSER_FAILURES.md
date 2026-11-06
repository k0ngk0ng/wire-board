# 正常音序语法时基的先行失败场景

本项补正常音序解析中已展开的纯语法工作：token 分类、控制分派入口、有限 loop／jump／call／return、时长倍率及返回 parser 的固定回边。复用既有两份原版 core 做有界原指令对照，不新增 core、调试器或硬件诊断分支；生产只消费已有的语义 Work，不执行原 PC／opcode／ROM／栈。

实现前覆盖以下失败场景：

- 将每个 Work 当成一条指令或一个固定总成本；漏掉 refresh 的操作边界、位移寻址、条件分支 taken／not-taken 或移位次数成本。
- DAC 和 FM／PSG token 路径分类不同却套用错误的读取成本；控制 token 的分类跳转与随后 JSR 重复或漏计。
- 控制分派未包含原 caller 与中央分派，或把实际 handler body／返回一起误算进去。
- loop 的 first／repeat、taken／exit 路径合并；call／return 使用原保存栈驱动生产规则；flow handler 返回后 parser 回边重复或遗漏。
- duration 倍率为 1 时错误零成本，倍率为 2 时少一次循环，或接受未证明的其它倍率。
- 参数服务、音高解析／note commit 的现场 body 或资源寻址尚未定价，却成功返回零值成本；不支持的 Work 必须显式失败。
- 输入含未知语义字段、原地址、未知 action、预算越界或时钟溢出；失败泄露部分输出、修改调用方 Work 或持久保存另一份时间状态。
- 原版私有对照被记录器改变，或两份固定 core 的时钟／RAM／寄存器结果不一致。所有原运行必须有限步骤结束；超界须失败，不猜测下一 PC。

私有原对照只使用原固定代码、隔离 RAM 参数与显式起点／refresh profile。它证明被选择语法阶段的时钟，不替代完整 driver、异步世界／IRQ、参数 body、独立反馈或最终混音验收。Go 测试及原 oracle 均在新增实现之前冻结。
