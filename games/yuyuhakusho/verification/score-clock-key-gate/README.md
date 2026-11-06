# 音符开关与门限验收

[失败场景](../../../../docs/research/md-yuyuhakusho/SCORE_CLOCK_KEY_GATE_FAILURES.md)、原版 oracle 与 Go E2E 在实现前冻结，`missing-api-red.jsonl` 保留真实失败。首轮新增 290 条记录全部通过，测试和 oracle 未修改。

`native-key-gate-full.json.gz` 在两份既有 pinned core 上覆盖十种合法 channel、FM key-on／off 和 PSG key-off 的 note／tie／claimed 组合，以及 gate remaining 0／1／2／255，共 288 个 profile、1,152 次执行。observer 开关前后完整 native 状态、RAM、PSG context 相同，两 core 的 2,378 个步骤、360 条 Main 访问、24 个 PSG 写入及结束时刻一致；身份与命令见 `native-capture.json`。

E2E 使用真实 mixed Runtime 独立生成 bus／mailbox／busy，核对全部 Main 和 PSG 访问与结束 clock。FM tail 的 canonical YM return 不再重复，PSG／suppressed path 保留唯一 return；expiry 的原 SP 只用于证明父调用跳过，在 Go 中为语义结果和固定成本。shared budget、失序、overflow 与失败后拒绝继续均通过。

本项目与实际包络回调共同验收，原生／NodeWasm 各 5,308 条记录无跳过。共同 manifest、复跑命令见 [score-observed-envelope](../score-observed-envelope/)。profile 声明初始 held bus／mailbox0／无额外 IRQ，不以这些固定 leaf 代替完整 voice 或 world 时间线。接口边界见 [KEY_GATE.md](../../audio/scoreclock/KEY_GATE.md)。
