# 正常音高与 lookahead 时基的先行失败场景

本项恢复音高转换和 duration lookahead 的有界阶段，继续使用已有原程序与两份固定 core。Go 测试与原 oracle 在新增实现之前冻结。

- FM 的 DIVU 成本随有效音高索引变化，却使用统一常数；忽略除法内部跳过一次 bus refresh 的特殊边界，或把检查放到整条指令结束之后。
- 把 128 个有效索引当成源 token／CPU 寄存器状态；生产不得执行原代码或保存原 opcode、PC、stack／ROM。
- FM、PSG、DAC 及 rest 分支混用；音高字段写回后又重复计费 return／cleanup，或错误包含未恢复的现场 modulation／gate body。
- duration probe 的 take／reuse 路径合并；漏掉 PSG 独有的额外测试、DAC 的旧时长复制或原读指针回退。
- 转换与 probe 的起止边界不明确，重复或遗漏 caller、duration callee、最终 commit；未支持的音色资源／参数 body 被默认为零。
- 未知 family／form／outcome、无效索引、额外字段、操作预算越界及 uint64 溢出返回部分价格，或修改调用方状态。
- 原生私有观察前后完整状态／RAM／寄存器不一致，两固定 core 结果不同或执行超出声明阶段。

私有 PSG 高索引可以验证固定读取时序，不据此把未恢复的相邻 ROM 内容引入正式音高表。此层只价格实际边界，不替代完整 driver、独立世界／IRQ、资源参数服务、采样发布或最终混音验收。
