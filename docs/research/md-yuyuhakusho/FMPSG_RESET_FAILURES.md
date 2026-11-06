# Ordered YM reset 的先行失败边界

PCM 命令层的真实私有输入先完成 DAC 写 `7183080`，随后调用原 `fm_reset(7183006)`，芯片采样点 `7183008` 位于两者之间。按 raw 时间排序会在第 8 帧 byte56 首差；保留原调用顺序则 12 帧、8,832 个立体声采样及 DSP 状态一致。此前 20 组排序没有可听差异的记录已由 owner 订正，不能借它们声称 red。

原 `fm_reset` 只执行 `fm_update(rawCycle)`、`YM2612ResetChip`、busy清零。它不清 PSG、FIR／integrator、已生成 FM buffer 或采样进度。warm chip reset 保留共享 address latch 和延后生效的 keyscale，不能直接替换成冷 `ResetYMState`。

## 两种明确输入域

- `ResetYM(cycle)` 遵循普通非降 raw cursor。
- `ResetYMAfterDAC(requestCycle, completedDACCycle)` 只用于 Device 实际完成 DAC 原子段后才观察到 reset 请求的因果。完成 cycle 必须等于当前 frontier；请求不得早于已 flush 帧，回退最多 194 master cycles。该边界来自本固件 single DAC 的 13T×15，回退严格小于195；不称为所有 YM I/O 或硬件的极限。调用者不能把任意 FM／PSG 写伪装成该域。

特殊 reset 保持完成 frontier，不修改传给 FM 更新器的原 request cycle；不 sort、不 clamp，不让普通端口写任意回时。第二次 release reset 已晚于 frontier，走普通方法。两种方法共用一个原子事务，不保存输入历史、reset 时间副本或第二份芯片状态；旧 Queue 不增加 reset 或回时能力。

## 生产前必须覆盖

| 场景 | 可能失败 |
| --- | --- |
| 真实 `7183080→7183006`、跨 `7183008` | 排序后输出首差被隐藏；已生成采样被撤回或重算 |
| 明确私有回退194 | 边界未验证，195被误接收，错误宣称通用硬件上限 |
| reset前完整FM／LFO／SSG／CSM／timers | 冷初态替代 warm reset，address／latchedksr 被抹去，busy 未清 |
| PSG和FIR均有待播放数据 | 误当全音频reset，丢失PSG、integrator、tail或本帧输出 |
| 帧尾／lookahead、下一帧carry | 原输出与副效混淆，FMGenerated 被清零或raw cycle改写 |
| 同cycle／重复reset、随后data-only访问 | 丢共享latch，重复生成或重置错误分频相位 |
| 普通写或普通reset回时 | reset例外泄漏到旧单调合同或旧Queue |
| 假completed witness、超过194、跨已flush帧 | 接受不属于本域的顺序或部分提交 |
| snapshot／restore与容量／overflow | 半帧不可恢复、别名、失败后FIR／owner／计数改变 |
| Native／race／Wasm／既有PCM | 新域破坏17+62、旧严格PCM与原生状态oracle |

原生检查先于生产实现。完整 callback trace 与私有 chip 输入分别报告：前者核对已有 CPU 原控制的 PCM／已捕获 DSP 状态；完整 YM／PSG／FIR 状态在原生 chip 重放中额外采集，不能把此前没有记录的 CPU 字段补称为直接观察。所有旧 fixture 和误标订正保留。
