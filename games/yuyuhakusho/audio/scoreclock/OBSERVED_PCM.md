# 乐谱事件与实际 PCM 阶段接线

`BeginObservedPCM` 在实际 `ObservedHooks.ReadDAC` 回调处启动音乐仲裁或 channel-six 声像阶段。反馈由同一真实 PCM owner 现场生成；之后仅在对应的 `OpChipWrite` 到达时调用一次 `Consume`。调用方在真实 return 阶段调用 `Finish`，并将全部 detached owners 共同提交。返回的诊断 intents 不能再次播放。

`FeedbackRead.TrackPan` 是 working voice 的持久声像值，`WritePan` 是本次 YM 调用值。自动声像可以只改变后者的左右声道；DAC 的恢复声像继续使用 TrackPan。两者包含 AMS／FMS，均为现场只读值投影，未增加 `State` 字段或持续 owner。旧 Stereo 字段与诊断 intents 保持兼容。

普通 FM1～FM5 声像没有 DAC feedback 回调，使用 `BeginObservedPan`。调用方从当前 `TickTransaction.Snapshot` 读取 working voice 的保存值，再将实际 `OpChipWrite` 及该值传入；本次 YM 值直接来自该操作。不能读取仍未提交的原 driver 快照来猜测 working 状态，也不能把普通 FM 写入伪装成 DAC 查询。

`PanRequest.WritePan` 是可选的普通 voice 调用值，默认仍使用原 Pan 字段。Begin 将其复制为派生值，不保留调用方指针。重定向始终使用保存字段；instrument tail 本身从字段加载，拒绝不同的外部覆盖值。补充原版对照覆盖六种合法 FM voice row、四种 marker 和两组不同字段／调用值，保留旧 108 个 pan profile 的行为。

阶段核对 voice、逻辑 event position、intent offset、order、kind 和完整内容；预期 intent 返回独立值，不能通过修改其 Pan 指针改变内部预期。提前结束、错误／重复事件、失败后继续均被拒绝。music、pan 分支只要求各自实际使用的 bus 能力，沿用原有 timed leaf，不新增执行程序、资源副本或时间线。

此接口只消费声明的反馈／write／return 区间。clock 必须由外层计时器推进到该阶段入口，普通控制参数、包络、parser caller、Root、world IRQ 等其它工作仍须各自计费。阶段时刻不代表整个 tick 已计费，当前不能据此启用平台整场声音。

实际 E2E 使用正式四份资源、真实 score driver／transaction、mixed Runtime 和已冻结的原版阶段边界，覆盖六个 descriptor 的 24 种音乐接受／拒绝组合、四种实际 FM1 自动声像、48 个补充原版 field／argument profile，以及错误／重复 intent。来源、测试初始化修正及复跑入口见 [score-observed-pcm](../../verification/score-observed-pcm/)。
