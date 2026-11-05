# 方程、常量与研究来源

生产代码为独立 Go typed 算术与小型连接图；没有复制／链接 C 原核心，不执行 CPU、固件、ROM 或任意 RAM。这里的「独立」指生产运行时和状态结构独立，不是声称从未阅读参考实现的 clean-room 法律结论。

数值和行为的参考是已固定的 Genesis Plus GX commit `60bee4c724859710ce05672239e5b601ad01382e`，`core/sound/ym2612.c`、`psg.c`、`sound.c` 和 `core/system.c`，并用已固定原生 dylib 的真实输出逐项核对。YM 源文件注明原 MAME FM 作者 Tatsuyuki Satoh、Jarek Burczynski，以及 Eke-Eke 的修正；其中的芯片行为还引用 Nemesis 的硬件测试与 Sauraen 的 die 分析。上述来源只用于研究语义和 oracle，不作为随游戏分发的实现。

`ym_tables.go` 的正弦／指数表由 [fmpsg_generate_tables.py](../../../../scripts/yuyuhakusho/fmpsg_generate_tables.py) 从下列方程生成，再按芯片整数舍入方式量化：

- 对数正弦：`round(256*log2(1/sin((2*i+1)*pi/1024)))`，仅保留 256 个四分之一波数值，其余波形由对称性推导。
- 指数 mantissa：`floor(65536*2^(-(i+1)/256))`，先截到 12 bits，再按最近值收至 11 bits 并扩为 13-bit 幅度。
- Detune、LFO 时钟和 PM bit8 contribution 为该参考芯片的数值事实。PM 其他有效位按独立的二次幂权重、逐位截断及四分之一波对称性推导，没有复制完整 32,768-entry PM 表。

PSG 源码明确含 NonCommercial 条件，不能把它的源码复制或直译进产品。本项 PSG 使用单独编写的 latch、divider、反馈位和边沿方程，并以无修改 native API 调用对照；相关来源和验证边界见 [PSG 文档](psg/README.md)与 [FMPSG_PSG.md](../../../../docs/research/md-yuyuhakusho/FMPSG_PSG.md)。

统一重采样复用 `audio/dsp` 的唯一固定 step-kernel 表。`dsp.StepKernel` 仅返回值副本；没有第二份 264 个系数。该数学 profile 的 Shay Green／LGPL 2.1-or-later notice、来源 URL 和原许可文字保留在 [KERNEL_SOURCE.md](../dsp/KERNEL_SOURCE.md)。本包没有移除或改写该 notice。

原始 native context 中的指针、缓存、连接地址和 RAM 字节均不进入生产。研究脚本使用原 scalar inspector 和已声明私有端口输入，完整保存原生输出和 typed 状态；不把这些事件时刻用作生产时间表。
