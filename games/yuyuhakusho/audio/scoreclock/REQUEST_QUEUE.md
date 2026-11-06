# 请求消费与普通 PCM 分派计时

`AdvanceRequestTick` 在稳定运行整拍的同一个执行器中串联实际队列检查、消费、dispatcher、普通 PCM 仲裁与发布，以及后续 18 个声部。所有阶段共用一个临时 clock、refresh 和操作预算，默认整拍上限为 65,536；显式较小预算不在子阶段重置。输入是给定的主时钟起点，本接口不产生 world IRQ 时刻。

`ConsumeQueue` 是独立消费入口，从原版 consumer 的入口开始，已排除调用者成本。`QueueResult.Fields` 与 `RequestResult.QueueFields` 只保存本次调用中实际 owner 字段访问的诊断记录，不持久保存第二份队列或执行游标。

## 唯一 owner 与读取顺序

`engine.ConsumeSoundQueueObserved` 是队列的唯一选择规则；原有纯值接口也调用它。读取与写入直接使用 Root 的 priority／selected 和 World 的 queue 指针。它从 D 到 A 扫描，保留特殊 FF priority 的尾部请求、屏蔽高位后的优先级比较，以及末尾有条件的 priority 写入。

`RuleHooks.QueueTest` 在计时器指定的时刻读取实际整队列，非空时才调用 consumer；`QueueOperation` 为每次实际访问计时。`SelectedWrite` 为 dispatcher 的 selected reset 计时。各 effect lease 必须在回调内同步调用一次；遗漏、重复、过期或回调失败均终止调用。不能预先拍下 queue，再在计时点重放结果。

普通 PCM 分派沿用实际资源目录、仲裁与发布实现，覆盖 `F0` 的独立 descriptor 重映射。仲裁反馈在各自的读取阶段获取；发布保留已采样的 StartDual。Root 是 choice 字段的唯一 owner；如果 bus 同时实现 `PCMChoiceField`，该端口必须读写传入的同一个 Root，以便记录真实访问。FF selected 保留 no-dispatch 路径。

## 当前边界与失败处理

入口仍要求 fade／pause／control 为 idle，没有 pause restore payload，audio-pause 为零。当前接受正式 ordinary 目录闭包与 FF；music、primary、01／05／04 和 selected 0 的重初始化继续明确拒绝。活动 score voice 要求 chip 实现 `KeyGateChip`，空声部的普通 PCM 调用不需要构造虚假的 PSG owner。

Root、score、queue、bus、chip 和 PCM 必须来自可丢弃的同一分支，只有成功后才能联合提交。错误可能已产生部分外部访问或字段修改；本接口不代为回滚这些 owner，不自动 flush，也不重放返回的诊断事件。完整 dispatcher、非 idle fade／pause、world IRQ、请求生产时刻、联合 checkpoint 与平台播放器仍待完成。

原版字段时刻、完整普通请求与复跑记录见[验收目录](../../verification/score-clock-request-queue/)。
