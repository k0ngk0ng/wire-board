# 音高写入与自动声像

`BeginFrequencyTransfer` 接收已经计算的音高字，准备 FM 的高／低寄存器或 PSG 的 latch／data 两次写入。`ExpectedEffects` 返回独立值；调用者在实际 score 意图到达时按顺序执行 `WriteNext`，全部写入后执行 `Finish`。Begin 不写硬件，提前结束、错序、重复操作或底层失败会使临时事务失效。

FM 路径保留正常 FM3 mode 为零时的检查、两次调用准备、voice bank 路由，以及末尾 return。两次写入复用现有 YM 服务，并共享 clock、refresh 和操作预算。FM 高频率寄存器使用硬件有效的六位数据；原版 oracle 继续保留被硬件忽略的高位，测试仅在明确的硬件比较投影中屏蔽。非零 FM3 mode 尚未计时，接口直接拒绝。

PSG 路径将 noise channel 的音高写入重定向到第三个 tone channel，保留 nibble／六位 continuation 的准备成本。两个实际 store 使用现有 `psgAbsoluteStore`；PSG 不增加 YM／ZRAM 的七个 master cycle 访问延迟。

`Catalog.PanAutomationStep` 读取现有 `param.AdvancePan`，不推进任何声部。`BeginPanAutomation` 按 note／held 入口计时：mode 4 在 note 入口再次经过 held dispatch，mode 2 的 once stop、reset、period 比较和 signed-byte step limit 均保留。声像输出只替换当次写入的左右声道，持久 `Pan` 和两个 counter 仍由原 score driver 唯一维护。

自动声像嵌套已有 `PanRouting`，共用同一个临时 service；未重写 YM、bus 或 PCM 路由。普通 FM 只需 `Bus`；channel 6 还必须提供真实 `PanBus`，以便必要时访问 PCM 声像请求。claimed 可以抑制硬件写入，但仍须完成子调用 return、elapsed 增量和外层 return。

这些接口只覆盖声明的准备音高写入和自动声像调用，不包括上游音高计算、完整 voice caller、Root／world IRQ 或整场播放。错误可能发生在已经写入 detached chip／bus 之后；调用者必须丢弃整个联合事务，不能只回滚局部 clock。接口不自动 flush 音频帧。

冻结原版、实际 score 回调和复跑方法见 [频率与自动声像验收](../../verification/score-clock-frequency-pan/)。
