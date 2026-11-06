# 完整控制 ledger 的先行失败场景

本清单和新 original-only 20 组向量先于控制扩展。v1 production、原 12 组测试和历史源归档保持不动；missing API red 在独立 `.local` 测试包执行。

- 遗漏 single／dual 及 boot 的 YM28=6，或为加入新控制而改写旧 12 组向量。
- `SetRestorePan` 错误立即改动当前 DAC pan；只应更新以后自然结束恢复使用的 canonical byte。完整 B6 低位也须保留。
- 硬复位的 assert／release 两次真实 chip reset、release 15-cycle 对齐、boot 逐项初始化和后续 idle／重新起播错误。
- reset 请求落入 DAC 原子操作时，把完成的 DAC 与 raw 较早 reset 按 cycle 重排，或者 clamp 原始时刻。实际调用顺序是独立证据，必须保留。
- `7F` 的 1-byte single payload 仍预装 128 bytes；额外 127 次只影响总线，不允许从资源外取值解码。该私有合同不把 diagnostic 自动纳入公开 finite 目录。
- `F0` 原 descriptor index122 被误写成 `7A`，导致飞影黑龙使用错误资源。
- DMA effect 没有在每次 bank 访问发生处应用，或将 `Remaining!=0` 的 wait flag 当成 ZRAM `1FFF`／完整外部调度。当前设备先限定 `Remaining==0`，未证明的环境原子拒绝。
- 为让 reset PCM 通过而排序事件、修改旧 DSP 或宣称 quiet-FM 的 DAC reset bridge 等同完整 YM reset。新控制先比较完整 ordered ledger／状态；受旧单调 DAC Queue 限制的 raw 逆序场景不强行扩大波形结论。
