# 完整新音符声部计时

`BeginNoteVoice` 从 NoteWait 为 1 的实际声部入口开始，`Consume` 接收目标声部内每个真实 `Operation` 及当前 transaction 的 working voice，包含最后的 voice leave；`Finish` 返回整段结束 clock。它串联固定 parser 结构、参数处理、乐器／level 传送、初始 key-off、音高转换、early／final cleanup、duration、FM／PSG 后处理和 DAC 发布。

所有阶段共用一个临时 service，复用已有固定 leaf 与实际 bus／chip／PCM owner。Parser 的 parameter-read 是后续控制前缀所拥有的读取说明，不再计费一次；flow handler 完成后才计入 parser backedge。乐器的 25 次写入与 pan 尾部、FM／PSG 不同的 early cleanup、rest key-off、tie 参数观察和最后 return 分别保留原边界。

FM／PSG 的新音符后处理复用 `HeldVoiceTransfer` 内部的同一套 pitch、volume、pan 与 key 服务，以显式 note 分支进入；held 公开入口仍保留原范围。`Catalog.InstrumentParameters` 只返回当前唯一所选资源的不可变工作与独立值 patch，不新增所选音色或源 index 状态。

DAC 和特殊声像的 feedback 在实际读取操作到达时准备。调用者的 `ReadDAC` hook 必须转交同一个 transfer 的 `ReadDAC`；反馈只可读取一次，后续 result 和实际写入必须匹配。音乐 metadata 在对应 chip intent 才通过唯一 PCM owner 发布，保留 held interval 的起止点；不能在资源回调中提前发送整组效果。

该接口不执行原 CPU、ROM 或原 flow program，不推进第二套事件／包络 cursor，不作为联合 checkpoint 保存。它尚不处理 track-end／ownership、外层声部扫描、Root／world IRQ 或完整平台播放。唯一 queue-replace 控制位于有限音轨的 track-end 前，随终止路径继续实现，当前显式返回 unavailable；未知工作保持错误。

错误后必须丢弃整个 score 与外部 owners 的联合分支。score 回滚无法撤销已发生的 detached chip／bus 访问。接口校验顺序、owner、事件位置、意图和阶段，提前／重复结束或错误后继续均被拒绝；不自动 flush 音频帧。

原版、测试先行、初始状态订正与可重复验证记录见 [score-clock-note-voice](../../verification/score-clock-note-voice/)。
