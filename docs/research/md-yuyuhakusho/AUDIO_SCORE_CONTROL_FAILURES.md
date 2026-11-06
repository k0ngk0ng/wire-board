# 比赛 score 控制动作与阶段事务失败场景

本清单、新动作／阶段测试与缺 API red 先于实现。已冻结的 Observed、原 42 fixture、旧测试和 parser sidecar 不修改为新期望。Root 自己持 fade、priority、selected、queue、pause、PCM 与 chip；score 不增加这些字段副本。

- `01/05` 只清 DAC Active，遗漏 NoteOff／Claimed／Tie；反而重置该行参数、owner 或停止 PCM。
- 淡出重跑完整 voice tick，推进 NoteWait、gate、envelope、modulation、pan、tempo 或 Ticks。
- FM BaseLevel 加一未按 byte wrap 和 signed threshold；负值被 clamp、错误 keyoff，或因 claimed 直接跳过参数更新。
- PSG 以 envelope level 写入或调用 envelope.Next；达到 16 仍写音量／错误清其他 flag；FF→0 环绕被遗漏。
- PSG 的原 note-off、claim、tie gate 抑制条件被绕过；FM level carry／loaded algorithm 被重新猜测。
- ResetScore04 清了 Ticks、Root字段或 PCM；擅自再发 04 的 YM27／六 keyoff／24 TL／四 PSG，造成 Root orchestrator 双写。
- 控制动作失败提交了部分 score 或输出；错误后继续调用 hook。
- 分阶段 Tx 在 Root 已推进 tempo 后又调用完整 Advance；一个实际 tick 计数两次或不计。
- 阶段乱序、重复 Tempo／Dispatch／Voices、过早 Finish／暂停结束，仍提交状态。
- Tx 期间对同一 driver 重入另一个 Advance／Apply／Begin，覆盖 working state。
- Abort／错误不释放临时事务，或 Finish 后继续修改已提交 score。
- Root 把控制动作前后整帧状态混为 action 期望，包含正常 voice tick／异步状态变化；用 Go 规则生成原版 golden。
- 捕获的 before／after 和 IO 不绑定原 core 对照、来源与 digest；初态不合法却隐藏 Restore 错误。
- 新接口破坏旧 Advance／Observed／State、原 42 流与 parser 原读取计数。

仅新增临时 working transaction 和枚举 score 动作，不持久保存 Op 历史／PC／原字节／CPU 栈或第二组 voices。外部效果仍必须在 detached 全 owner transaction 内执行，score 不能回滚 authority bus／chip／PCM。
