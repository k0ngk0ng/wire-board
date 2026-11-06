# 外层声音中断与补跑计时

`AdvanceSoundInterrupt` 与 `AdvanceSoundCatchup` 在现有 driver 执行器外增加 delay／delayed request、busy、backlog 和实际 driver wrapper。调用方提供已经确定的入口 clock；本层不选择世界 IRQ 接受点。所有 scheduler、enqueue、wrapper、Root 和声部服务共用一份临时时钟、refresh 和预算，子 driver 不重置预算。

scheduler 和请求选择由 engine 的 observed 规则操作唯一 owner。原有纯值 scheduler 接口也调用同一实现。enqueue 在负 slot 的 TST 和 CMP 分别读取实际队列，保留中途变化；字段读写的 effect lease 必须同步执行一次。wrapper 的固定跳转、所有分支与 return 独立计价，诊断 `Entries` 记录实际 wrapper 入口，不接收原版 PC 程序。

外层原始接口仍要求 detached owners；错误可能发生在前面的 child driver 已完成之后。需要整体提交与存档时使用 [session.Runtime](../session/README.md)，不对部分状态做事后补偿。

原版完整调用与当前验证见[声音 IRQ 验收](../../verification/sound-irq-runtime/)。这些证据只证明给定入口 clock 的外层声音调用，不能替代世界 CPU／VDP 调度证明。
