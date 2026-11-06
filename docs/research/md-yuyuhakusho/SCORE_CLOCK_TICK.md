# Driver 固定入口、返回与 priority byte 清除

新增 `EnterTick`、`FinishTick` 与 `ClearPrimaryPriority` 已在给定起点和 refresh deadline 下，与两份原生 core 的 290 个声明私有边界匹配。全部 pause byte、五种起点／refresh 相位、825 次即时 pause 读取、275 次 pause／priority 写点及最终游标一致；另与原自然暂停／恢复的两个实际字段写点相同。

这些固定段不构成完整 driver tick 成本。normal gate 在 duck／tempo／fade housekeeping 之前停止；pause-start 与 resume 在总线、静音／level 恢复前缀之前停止。世界 producer 到 IRQ、IRQ 插入及完整音序解析成本仍由后续阶段恢复。

## 接口和边界

```go
gate, err := scoreclock.EnterTick(clock, rootFields, limits)
// rootFields implements ReadPause(at) and WritePause(at, value).
// Continue according to gate.Route, adopting gate.Clock only on success.
```

| Route | 已计入部分 | 下一步 |
| --- | --- | --- |
| `TickNormal` | 清除当前临时 domain、读取 pause 和 normal 分支。 | 从 housekeeping 开始继续，不能直接跳到声部。 |
| `TickPauseHold` | hold 的检查和本次早退。 | 本次 driver 已结束，不再加 `FinishTick`。 |
| `TickPauseStart` | 正值请求确认写 `FF`，到下一阶段之前。 | 继续原总线／静音流程。 |
| `TickResume` | 负值恢复请求实际清为 0，到下一阶段之前。 | 继续原释放总线／level 恢复流程。 |

pause 在原读取点即时查询唯一 Root owner：normal 一次，`FF` hold 两次，其他非零路径三次。不能把第一次返回值当成后续读取的快照。`GateResult.DomainClearedAt` 只报告临时 domain 清除的位置，不持久保存原 F80E 或另一套 score 状态。

`FinishTick` 只计算一次固定最终返回。YM／PSG leaf 和 hold gate 已有自己的返回，不能重复追加。`ClearPrimaryPriority` 只计算一次 `CLR.B abs.w` 语义操作，并在实际 store 点调用唯一 priority owner；原型来源为 primary track-end 的 byte 清除。

bulk driver reset 的 `CLR.L (A0)+` 和 priority 恢复的 `MOVE.B` 是不同操作。不能用本 byte-clear helper给它们统一收费，也不能先计 bulk 工作再额外加一次 byte-clear。音乐 `PrioritySave`／`PriorityRestore` 的顺序和临时 latch 仍由其 Root transaction 处理。

## 测试与原证据

[失败清单](SCORE_CLOCK_TICK_FAILURES.md)、完整 native fixture、缺 API red 和测试源码先于 `tick.go`。缺 API 检查通过 Go overlay 将冻结测试文本作为虚拟文件编译，保留了旧 package 在并行验收时的可编译状态；取得真实 red 后，才写入相同字节的正式测试与实现。

原工具直接启动固定 CPU 执行到声明的下一个语义边界，PC 与步骤只进入研究 provenance。256 个 pause byte 在同一相位完整覆盖，关键六种 byte 另覆盖四种相位，再加五种返回与五种 priority 清除配置，共 290 例。每例两份 core、recorder 开／关都比较执行边界、完整 serialized state、refresh 和 RAM，总计 1,160 次原生调用。原生重采的完整压缩记录逐字节相同。

Go native race／Wasm 各四项顶层检查、290 个子场景与 vet 通过。fixture不含原 PC、opcode、跳转目标或可执行程序。固定语义成本来自[架构来源绑定](../../../games/yuyuhakusho/verification/score-clock-tick/cost-source.json)，没有根据原总耗时拟合常数。

原自然第 3,001／3,041 步分别在 `201741`／`201684` master cycle 写 pause 确认／清除。以各自原 driver 入口和 refresh deadline 调用新 gate，得到相同 store 时刻；这只验证给定起点后的局部赋时，不预测这些原入口为何在该时刻发生。

## 状态与错误

本包不保存 pause、priority、domain、busy、RAM、寄存器或第二份全局时钟。`Clock` 只在单次调用中作为暂时游标，成功后由 caller 采用结果。输入校验在外部访问前完成，预算不足和字段错误显式失败。

callback 属于 caller 的 detached transaction。错误可能发生在更早的语义 effect 之后，caller 必须整体丢弃未完成 transaction，不能将该失败结果提交给 authority。原私有执行使用 SR2700，不推进辅助 CPU／VDP，没有额外 IRQ、scanline 或 bank steal 插入；这里计的是固定架构工作与 refresh，不能当作任意世界环境的真实 elapsed。外部总线占用与全局 IRQ 仍须由后续统一调度组合。

测试命令与全部 SHA 见[验收索引](../../../games/yuyuhakusho/verification/score-clock-tick/validation.json)。本分工没有修改已冻结的 `ServeYM` 证明、其他音频 owner、engine、平台、总索引或进度百分比；没有上传、提交或发布。
