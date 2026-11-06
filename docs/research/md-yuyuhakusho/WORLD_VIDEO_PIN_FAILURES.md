# CPU IRQ 请求操作的失败范围

本轮在首次实现后继续复核固定 core 源码，发现 HINT 的 `m68k_update_irq(4)` 是按位 OR，区别于 VINT、ack 和使能位变化使用的赋值。首次实现只有赋值请求，须先用现有 API 取得语义失败，再增加闭合的 OR 请求效果；首次实现及测试均另存，保留修正前证据。

- CPU pin 为 6 时，HINT 的 OR 请求不能把它降为 4。
- VDP 的 VINT pending 不能用来反推当前 CPU pin；pending 与 CPU pin 分属不同 owner。
- HINT disabled 时仍置 pending，但不产生 OR 请求。
- VINT 边沿、ack 和寄存器使能更新不能误用 OR，尤其清 0 必须是赋值。
- 效果不能同时声明赋值和 OR，也不能在没有请求时令调用方误用 Level。
- snapshot 不添加 CPU pin 副本；外部 CPU owner 按效果顺序更新自己的 pin。

这属于源码关系的验证，不宣称已经直接观察到原 CPU ack 内部的 pin 变化。
