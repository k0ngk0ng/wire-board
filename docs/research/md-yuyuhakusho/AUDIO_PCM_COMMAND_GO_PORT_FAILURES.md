# YM 地址 port 输出的先行失败场景

本清单与新 23 组 original-only port／frame-latch 向量先于地址输出实现。旧 v1／v2／分段 Bus 的向量与断言保持不动。

- 把地址统一补在 data cycle，而丢失真实 single 17T／dual 11T 地址到 data 间隔。
- 在 data 尚未发生时硬复位、跨 frame held，实际地址 latch 仍应保留先前已写值；data-only 视图无法重建此状态。
- 固定启动、自然结束和 boot 的 B6／28／2B 地址写点错误；bank1 port2 与 data port3 混用。
- 为输出真实地址而在设备增加 register-latch shadow 或第二芯片 owner；旧 data-domain proof 被混称 full port。
- 新 output view 未绑定 immutable Config／checkpoint identity；包含地址的 pending 输出被另一视图静默解释。
- 添加地址输出改变原阶段、原子过冲、grant／resume、mailbox 或 data／bank 事件时间。
- pending 地址事件丢失、错误端口、低位地址截断、资源／bus metadata 搭错或快照共享输出指针。

先用旧 data-time 补地址的真实失败证明中断场景差异，再冻结新接口 missing-API red。生产仅在现有固定语义的真实写点发地址事件；YM latch 继续由芯片 owner 独立持有。
