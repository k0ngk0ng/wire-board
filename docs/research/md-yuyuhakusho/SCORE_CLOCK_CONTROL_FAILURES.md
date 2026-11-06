# 正常控制参数计时的先行失败场景

- 将 FM 即时 level-add 与 PSG 延后更新混用；根据正式事件的 ImmediateFM 选择，不从 action 名猜测源控制码。
- pan 忽略 PSG 的分支、pan automation disabled／enabled 的 1／6 次实际读取、instrument selection 前缀与后续 transfer 的 claimed guard／return 重复或遗漏。
- modulation setup 漏掉定义位置、三参数、steps 除二及 offset 清零；enable／disable 使用不同或重复成本。
- PSG noise 的 claimed 分支误写硬件，或将带源 operand 读取的 24-cycle store 错套普通 16-cycle PSG store。各阶段须共享预算、保留原访问点。
- queue replacement 在错误阶段写入或发生两次；Root 必须保留唯一 queue owner，timed callback 仅能调用当前 effect 一次，错误、重复和过期 effect 必须拒绝，旧消费者保持原行为。
- 临时 ControlProjection 缓存旧状态或改变 State／intents；现场值应来自当前 typed event。未知控制、非法 family／字段组合、预算／overflow 不得记零。
- 只验证私有 handler 就宣称 parser、track-end、envelope、world IRQ 或整拍已计费。prefix 的 continuation 必须明确，未完成目标仍保留 unpriced 错误。

先取得正式控制形状和现有两份 core 的完整 handler／声明 continuation 对照，冻结 Go E2E 后再实现。queue／noise 要验证实际 owner 写入，原 ROM、core、旧 fixture 和 golden 不修改。
