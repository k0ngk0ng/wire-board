# 包络工作量与原版计时来源

本目录记录 `d05f44fa` 时冻结的 Go 包络 binding／计时来源证据；该研究节点尚未包含 Go 实现。后续实现与验收见 [score-clock-envelope](../score-clock-envelope/)，整场音频仍有独立的完成门槛。

`audio-score-envelope-work.json.gz` 将原 580 个逻辑包络步骤绑定到当前 family／envelope ID／prefix 或 cycle／tick index：pitch 206 步、volume 374 步。正式闭包仅两步含额外 restart，资源使用 `restarts` 标量；没有原 byte、跳转目标、地址、原始游标或可执行导航列表。原声音 tick 与 cursor 未修改。文件 1,948 bytes，SHA-256 为 `b2d0d9363bef8e47832c67be65bbf380291a0e4a87c9e4b35c78bbf9445db028`。

`provenance.json.gz` 保留研究用源位置及每个逻辑 tick 的对应关系。`native-envelope-full.json.gz` 覆盖全部 tick、terminal 的 FM／PSG route、全部 256 个 signed-byte scale，以及 volume 的 base、note-off、claimed、tie／gate 与 noise channel；共 986 个 profile。`native-envelope-none.json.gz` 补充 44 个无包络 profile。两份既有 pinned core、observer 开关共 4,120 次执行，完整 native 状态、RAM、PSG context 和两 core 结果均一致。原 core／ROM 未修改。

`producer-before-none-supplement.txt` 保存第一组采集时的 producer；后续 producer 只增加 `--only-none`，不重写原组数据。两组均保留原始步骤、访问时刻、字段与身份，来源摘要及统计见 `source-validation.json`。

已核实的乘法边界：该 pinned core 的 `UseMuluCycles` 使用 `38 + 2 * popcount16(signExtend(scale))`，MULU 的固定表项为 0，没有 DIVU／可变移位的内部 refresh skip。后续 Go 实现仍须对冻结原版结果验证，不能用这个静态结论替代运行验收。

本次 profile 为声明的 calculator／volume leaf，无额外 IRQ。pitch terminal 停在 key-off leaf 入口之前，volume 包含实际 PSG 输出；不把这些结束时刻当作整个 driver／world tick。失败场景见 [SCORE_ENVELOPE_WORK_FAILURES.md](../../../../docs/research/md-yuyuhakusho/SCORE_ENVELOPE_WORK_FAILURES.md)。
