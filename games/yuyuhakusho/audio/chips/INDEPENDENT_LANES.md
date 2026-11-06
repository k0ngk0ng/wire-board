# FM 与 PSG 的独立输入时间域

原版按生产者顺序执行音频回调，FM 与 PSG 分别保留自己的时间。主机写入 PSG 后，下一次总线同步可能才提交时间更早的 PCM／YM 事件。不能将这些事件排序、夹紧时间、丢弃，或提前推进 PCM 以改变主机争用成本。

`WritePortOnLane`、`ReadStatusOnLane`、`WritePSGOnLane` 和对应 reset 入口保留实际调用顺序，各自验证芯片域内单调性。原有 `WritePort`、`ReadStatus`、`WritePSG` 和 reset 入口继续要求 aggregate cursor 单调，拒绝契约不变。不同域的 FIR 增量按原顺序加入同一缓冲，不重算已经生成的样本。

`InteractiveState.Cursor` 保留兼容的 aggregate frontier。仅当 PSG 超前时，`YMLag` 保存被遮住的 YM frontier 与 aggregate 的距离；`YMInputCursor()` 由二者相减得到。这个距离不能从 PSG 状态或 FM 已生成样本数推导，尤其是尚未生成样本的 address 写入。它不是输入历史或寄存器副本，零值不进入 JSON。frame boundary 推进各域最低合法时间，并在 YM 追上后清零距离。

旧的 `CommittedState` 不能表达仍存在的独立域差距，因此此时明确拒绝导出；完整 `InteractiveState` 的 JSON／restore 保留信息。快照校验还拒绝负 frontier、aggregate 与 PSG 不一致，以及将 frontier 放到已生成 FM 样本之前的状态。

mixed 用独立域入口接收 PCM 的真实 YM 事件和主机 PSG 写入。`LastPCMWasDAC` 指向最后 YM 输入的因果，PSG 写入不会清除此因果。reset 使用 YM frontier 判断顺序；after-DAC 仍只接受已证明的最多 194 master cycles 回退，不扩展为普通写入可任意回时。

8 组原版完整状态／PCM、同域和旧入口拒绝、JSON 恢复及跨帧结果见[验收目录](../../verification/fmpsg-independent-lanes/)。
