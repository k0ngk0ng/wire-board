# Driver 固定入口／返回与 priority 写点的先行失败场景

本清单先于新 tick helper。已冻结的 `ServeYM`、原测试和期望保持原样。

- 把 `pause` 只在入口读一次；原 normal gate 有一次读取，非零路径还会按原位置重读。
- 将正值 pause request、`FF` hold 与负值 resume 混为同一个分支；提前返回或把确认／清除写点统一移到末尾。
- 忽略初始当前 domain 清除、word 条件分支未跳的额外成本、绝对 word RAM 的 CLR 成本或 refresh 位置。
- 将只验证到 normal housekeeping 之前的 gate 写成整个 tick prefix 已完成；跳过 duck／fade／tempo／queue／dispatch 未恢复工作。
- 将固定最终返回成本重复加到已包含 RTS 的 YM／PSG leaf，或在 gate 的 hold 返回后继续推进 score。
- priority 清除没有在其真实字段 store 点通知唯一 Root owner，或用一笔 tick 汇总 bool 替代多个写点。
- 使用任意 opcode／PC／寄存器／RAM VM、原逐 tick 耗时表或可由调用者传入的 instruction program。
- observer 改变原控制状态，私有状态被写成自然输入覆盖，或错误／预算耗尽后仍提交部分 Root 状态。
- 将 SR2700、无 IRQ 插入的固定边界证明推成全世界 IRQ 排程或实际游戏声音已经接通。

原生工具只启动原 CPU 执行，并保存研究 provenance；Go helper 的缺 API red、fixture 和测试先于实现。字段回调、游标和外部状态在 caller detached transaction 上使用，不建立第二个 pause／priority owner。
