# 正常音高与 lookahead 的时基

`PricePitchWork` 从实际音高转换入口计算到原 pitch／marker 写回边界。输入仅为临时的 family、form 和已经转调后的七位音高索引，不包含源 token、PC、ROM、CPU 寄存器或原栈。函数不计算或保存第二份音序状态。

FM 的十二平均律 octave 解析包含随索引变化的除法成本。当前七位域下，内部成本可化为 `136 - 2*popcount(index/24)` CPU cycles，另加固定四周期；其 refresh skip 发生在内部工作后、固定四周期前。固定十一位 octave 对齐也有内部 22 周期加固定六周期的同类边界，不能把两者分别合并成单次 `spend(total)`。

FM／PSG 的 chromatic、rest 与 DAC marker 各按实际固定路径计费。写回后停止，callee return、caller、cleanup 和最终 note commit 均由后续明确阶段负责。PSG 私有高索引仅证明该读取路径的成本，不为正式音高目录添加未恢复的数据。

`PriceDurationProbe` 区分 take／reuse：take 停在 duration caller 之前，reuse 停在 final commit 之前。PSG 额外符号测试、原 lookahead 回退和 DAC 旧时长复制分别计入；duration caller、callee 和现场 commit body 不在这里重复计算。

所有操作使用现有临时 clock／refresh／预算机制，未知 family／form／outcome、非法索引、预算或 uint64 溢出均返回零值输出与错误。未恢复的 modulation、envelope、参数资源、乐器查找和采样发布时序没有因此变成零。

先行失败、原两份 core 的 1,064 个私有有界场景、4,256 次执行及 Go/Wasm 回归见 [score-clock-note](../../verification/score-clock-note/)。初版 Go 的 octave 位移漏掉内部 refresh skip，真实 red、初版代码和修复依据均保留；原 oracle 和冻结测试未修改。复跑使用 `scripts/yuyuhakusho/verify_note_clock.py --mode native|wasm|vet --out <新的仓库内目录>`，离线绑定审计使用 `--audit`。

该组件不等于完整正常 driver、独立世界／IRQ 或平台整场音频已经验收。
