# VDP 中断信号层先行失败范围

本层只研究固定参考 NTSC、224 行、H40 profile 的视频计数、VDP IRQ 使能／pending／ack 和 M68K 状态读取相关位。不执行 CPU、固件、任意 PC、RAM 或像素 VDP，也不决定主 CPU 在哪条语义操作后接受中断。

- 把 CPU 进入 handler 的周期当作硬件 signal 周期；把旧观察器的嵌套计数当硬件 SR 层数。
- 从可见行 0 开始 frame，漏掉开头 VBlank、788 周期 VINT 分段和最后 overscan 行重装。
- 在空白行递减 H counter，或 register 10 写入后立即重装当前 counter。
- 按 raw CPU cycle 排序寄存器写；忽略同一 scanline batch 的 CPU 指令过冲以及原调用顺序。
- 混淆 M68K 状态读与 Z80 状态读。M68K 状态读不清 H／V pending；Z80 读的其他副效不属于本 profile。
- ack 总清 HINT、丢掉 enabled VINT 优先级，或错误清掉另一种 pending。
- 把状态寄存器所有像素／FIFO／DMA 位都宣称已恢复。本层只验证所声明的 IRQ／VBlank 位。
- CPU 没执行指令的 scanline 没有直接观察却被填成原生记录；缺行必须明确标记，并按声明外部操作推进模型。
- observer 改变 RAM、寄存器、画面或 PCM，丢失／覆盖原控制组；使用错误 ROM、core、输入或 C ABI。
- 状态存全套 VDP 寄存器、原地址、全局时钟副本、PC 或 source schedule；非法 profile／阶段／越界输入部分提交。
- snapshot 混用阶段、alias 或恢复后重复 IRQ；只在一个原生片段通过就扩大为全部模式或全局 world→IRQ 排程完成。

先保存真实缺实现／缺证据 red，再编写只读原生 observer 和独立 Go API。每次原采从冷启动逐步核对已冻结的 2,400 视频步骤；只保留所选比赛窗口的细节。静态关系来自 direct Ghidra 的 68000 清单，不称 REA 原生反编译。
