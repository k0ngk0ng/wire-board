# 正常音序的语法工作成本

`PriceParserWork` 为已有 typed parser metadata 中明确支持的结构性 Work 逐操作更新临时 clock。它不修改 score、选择下一事件、复制原 stack 或执行原 PC／opcode。`FinishParserControl` 单独处理完成控制 handler 后返回 parser 的固定回边。

| Work | 起止与成本归属 |
| --- | --- |
| token-read | 从 token 读取准备到 control-call／cleanup 边界。DAC 与 FM／PSG 的这一小段成本相同。 |
| control-dispatch | 包含 PC 相对 caller、中央分派及进入已选 handler；停在 handler body 之前。 |
| jump／call／return／loop | 包含该有限 flow callee 及其 return；不含外层 caller／parser 回边。first／repeat、taken／exit 独立处理。 |
| duration-scale | 包含倍率 callee 及 return，0／1 次加法均有完整 setup、分支和写回成本；caller 成本单独归属。 |

每次调用在原固定操作边界检查 refresh 和操作预算。函数只返回本次 Clock，不保存另一份全局时钟或 parser 状态。语义字段超出该 Work 的明确形状、未支持的 action、未知 Kind、倍率超过当前 0／1 域或未定价的现场 body 均失败，不能把缺失成本当作零。

当前明确不提供 parameter-read 的 handler body、pitch-resolve、duration-probe、note-commit 或资源寻址成本。实际 metadata 的 39,045 个键、39,053 个变体中，63,728 个结构性 Work 已有价格；另 116,103 个 Work 在本接口继续返回 `ErrParserWorkUnavailable`。资源的 `unpriced` 标记没有因此改成 false。

原版对照使用同一份固定程序及既有两份 core，构造 148 个有限 RAM／clock profile，比较额外只读记录前后的完整 native 状态、RAM、寄存器及两 core 的每步时钟。新覆盖的 first-exit loop 是私有 callable profile，不声称在原自然音乐中出现。运行代码、原时刻与原 RAM 仅保留为研究／测试 oracle；生产只含固定语义规则和架构常量。

先行失败、冻结 Go 测试、原 oracle 和所有旧声音时序回归见 [score-clock-parser](../../verification/score-clock-parser/)。复跑入口为 `scripts/yuyuhakusho/verify_parser_clock.py --mode native|wasm|vet --out <新的仓库内目录>`，离线绑定审计为 `--audit`。原生 oracle 的有界复采使用 `score-clock-parser-capture.py --out <新的工作区 .local 目录>`，不修改 core、原 ROM 或原有 goldens。

该组件未完成整个正常 driver、独立世界／IRQ、音高／参数 body 或平台播放接线，不应据此声明整场音频通过。
