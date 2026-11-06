# Fade 局部赋时的先行失败场景

本清单先于新 fade-clock 实现与测试。范围仅为 01／05 的四指令 start leaf，以及从真实 caller 开始、到完成返回或两个未完成阶段入口为止的 header prefix；冻结 leaf、纯 fade 规则和其他 owner 均不修改。

- 将 start 当成包含 dispatcher table 或 caller 的服务，重复支付外层调用；或遗漏 leaf 自己的 RTS。
- 将 `CLR.B absolute-word` 误按 12 CPU cycles 计算；固定 core 的 opcode `4238` 为 16 CPU cycles，start 四条指令因此共 64 CPU cycles／448 master cycles。
- start 的 Remaining／Steps／DAC flag clear effect 顺序错误，或把全部 effect 移到函数末尾。相同值重复 store 仍须报告真实 effect。
- header 忽略 `375c2` 的真实 PC 相对 JSR（18 CPU cycles），或错误重复补入 caller。
- Steps=0 时仍读取／修改 Remaining，未保持携带的非零 byte；Remaining1→0 时提前进入 level scan／full clear。
- Remaining 原为零时遗漏 Steps 的真实写点；末步又错误发出 Remaining 重装。
- 将 taken／not-taken 的短分支、长分支成本混用；将 refresh 统一加到结束，或对 main RAM 写入额外加 YM／ZRAM 的 7 master cycles。
- 到 `38962` 后把未恢复的 6FM＋3PSG level body按零成本完成；到 `38a2c` 后提前计为完成04。输出必须明确 LevelScan／FullClear，且只停在各自入口。
- 通过任意 instruction／CPU cycles／原 PC program 提供成本，或保存另一份 clock／Root counter／score state。
- 输入 snapshot 在 IRQ／外部 stall 下已经失效，却继续冒充 live owner 读取。当前合同明确 SR2700、无额外 IRQ／辅助 stall 和稳定临时 work；更宽协同须另行验证。
- 未知 start command、无效上限、clock／refresh 溢出或预算耗尽时返回部分有效结果；本服务没有实际写入，失败应返回零值 result。

原生 oracle 先执行原函数，固定 source 架构成本只用于研究 provenance。两份固定 core 的 recorder 开关对照检查完整 serialized state、RAM、refresh 与每步边界；观察 core 的真实 RAM-write hook 还须与原步骤导出的 effect 点逐项相等。测试通过 overlay 留真实缺 API red，冻结后再实现。
