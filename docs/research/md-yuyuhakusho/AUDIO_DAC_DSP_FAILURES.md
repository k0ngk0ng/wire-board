# DAC-only DSP 与重采样的先行失败范围

本清单先于 `audio/dsp` 生产实现及其测试。现阶段先读取固定参考进程的真实配置与归一化初始状态，不凭默认注释选择滤波，不改已冻结 `audio/pcm`、仲裁、世界或流式接口。

- 离散 YM2612 的 enum 0 被误解为布尔关闭；选择错误芯片模型、HQ／线性 resampling、preamp、mono 或实际未启用的低通。
- 8-bit DAC 转 14-bit 的中心、符号、移位、原声道限幅、离散 ladder 正负偏移或静音 pan 的残余偏移错误。
- 不同 cycle 事件排序错误；相同 cycle 丢弃或重排；生成 chipTick 与寄存器写的先后、end 处事件及半块分割发生重播／漏播。
- 把 emulated CPU handler 时间误当物理 pin edge；忽略原 frame flush 与超出帧尾的 handler 调用可能存在的语义边界。
- 只比较听感、阈值、裁切或拟合增益；未从同一明确初始相位逐样本比较。
- FIR 相位、插值、左右通道、signed 算术、32-bit wrap、high-pass 量化、PCM16 clipping、低通是否启用或通道交织顺序错误。
- 将可由 absolute master time 和 immutable profile 推导的 offset／next chipTick 再存一份；未启用低通仍存冗余状态；同一 DAC 值保存多种可变位宽 owner。
- 区块边界改变输出、快照丢失 integrator／18-tap tail／上一芯片输出；保存／恢复借 native 指针、RAM 或 host context。
- `origin+time`、固定点乘法、区块样本数或资源分配溢出；非法 profile／事件／顺序部分提交状态或取消旧有效声音。
- 使用给定原事件时间通过后，宣称独立 cadence、prefetch、PCM 命令、FM／PSG 或世界声音全部完成。
- 把整个原 core、Cgo、CPU／RAM VM 或原 PC 时间表带入生产；复制完整模拟器而非独立 typed DSP 算术。

未知 profile 明确拒绝。本阶段仅拟支持实测离散 YM2612、HQ、44.1 kHz、preamp100、filter0、mono0 的 DAC-only 条件。若参考内核有分块相关现象，先保留反例与源控制流解释，不用调参制造通过。
