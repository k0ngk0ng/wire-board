# 连续 held-note 声部计时

`BeginHeldVoice` 从实际 FM／PSG 声部入口开始，接收 score transaction 当前 working voice 和 `RoutineVoice` 的 enter 操作。它覆盖 NoteWait 未到新音符边界时的整段调用：gate、FM 自动声像或 PSG volume、modulation、pitch calculator、终止 key-off，以及最后的音高写入和 return。NoteWait 为零时按原版回绕；gate 到期时直接完成声部提前返回。

调用者将这段声部内的每个实际 `Operation` 和当时的 working voice 交给 `Consume`，包括声部 leave，最后执行 `Finish`。执行器验证连续 sequence、voice／owner／position、intent offset、counter 前后值和硬件意图。FM 高／低写入分别执行，PSG pitch pair 在对应的实际分组操作中依次执行。Begin 和纯计时阶段不提前写硬件。

所有子调用复用已有的 canonical 服务，并共享同一个临时 clock、refresh 和操作预算。PSG volume 或 pitch envelope 的终止路径不会再执行第二次 key 服务；原版的 tail return 和 gate 提前返回分别只计算一次。重构后的独立 leaf API 保留原有校验与行为。

音高只在实际资源回调、canonical cursor 推进之前，从只读 catalog tick 和当前 working voice 计算一次临时请求。终止时的固定 bypass word、scale、detune、调制增量和六位硬件有效值保持各自语义。执行器不推进或持久保存第二套 cursor／counter；自动声像也不改写持久 Pan。临时 transfer 不能作为联合 checkpoint 保存。

这个入口明确拒绝新音符边界、DAC 和尚未证明的 owned FM6。声部扫描 caller、parser／control、新音符、track-end／ownership、完整 Root／world IRQ 和平台整场播放仍由后续执行路径负责。全局未计时错误保持原样，不能根据本项验收把未知工作视为零。

底层错误、预算耗尽、提前结束、错序或重复操作都会使 transfer 失效。score transaction 的失败可以撤销 score 的分支，不能撤销已经发生在外部 detached chip／bus 上的写入；调用者必须丢弃整个失败的联合事务。接口不自动 flush 音频帧。

原版整段调用和实际 score／mixed owner 的验收见 [score-clock-held-voice](../../verification/score-clock-held-voice/)。
