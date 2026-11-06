# 连续 held-note 声部的先行失败场景

- NoteWait 的零值回绕、gate 为零／等待／到期，以及 gate 到期提前返回混用；重复计算 caller／return，或漏掉 FM／PSG 不同的 branch 长度。
- Pan／PSG volume、modulation、pitch calculator、key 和频率写入各自重新预算或重置 refresh，导致分段通过但整段时刻不一致。
- 在 score 实际写入意图之前输出硬件；FM 高／低或 PSG pair 错序，volume／pitch 终止时的 key-off 被执行两次。
- 从已经推进的 envelope cursor 反推本拍音高，或增加第二个持久 cursor／counter／prepared word；终止 bypass、调制回绕或 pan 当次 stereo 覆盖持久字段。
- 忽略实际 Operation 的 voice、owner、position、sequence、counter、intent offset 或值；重复／漏掉阶段后仍能结束，错误后还能继续执行。
- 联合事务失败后错误提交 score，或只回滚 clock 而继续使用已经写入的外部 chip／bus。
- 将完整 held-note body 的验收扩大为 note parser、track-end、完整 driver tick、world IRQ 或平台整场播放；未支持的入口须继续显式拒绝。

本计划、原版连续调用 oracle 与 Go E2E 测试先于生产改动。现有逻辑 score driver 仍是动态声部状态的唯一 owner，原版 ROM／core 仅用于开发对照。
