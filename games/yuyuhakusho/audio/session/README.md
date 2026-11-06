# 联合音频状态与外层声音调用

`session.Runtime` 是 Scheduler、Root、score、PCM 和 FM／PSG 的联合 owner。`State` 绑定不可变资源身份并各保存一次 canonical state；World 的 queue、audio-pause 和主时钟只在每次调用中借用，不进入音频存档。JSON 恢复验证各 owner 及资源／profile，pending pause payload 与 FIR／PCM 分支均独立恢复。

`Interrupt` 从调用方提供的实际 `219ac` 边界执行，`Catchup` 从实际 `219ea` 边界执行。二者使用 canonical scheduler 与真实 driver wrapper，共用一个临时 clock、refresh 和整体预算。delay 为 1 的调用只递减，不当次提交；busy 重入只增加 backlog；catchup 在 driver 后读取并递减当时的 backlog，不使用入口旧值。

每次调用先对全部音频 owner 和借用的 World 字段建立可丢弃分支，只有完整成功后才一起提交。`mixed.Fork` 借用已经认证的不可变采样库，不在每次声音调用重新哈希载荷。失败不会留下半次声音调用或消耗外部 World 队列；返回的 intents／字段仍只作诊断，不重放。

`AdvancePCM` 显式返回真实主机争用成本，外部 main owner 必须保留该结果。`FrameBoundary` 只完成已经到达的 reference frame，不暗中 RunUntil 或推测世界边界。这些接口不决定 CPU 中断接受时刻，不把视频帧当成固定声音消费点，也不安装原版 PC／时间表。

完整原版中断／补跑、live 字段与联合 JSON 状态对照见[验收目录](../../verification/sound-irq-runtime/)。world IRQ／请求生产的独立时序、平台连续播放与浏览器整场音频仍待接通。

## Actual world producer transaction

`WorldFrame` stages the actual `StepChuSessionFrameObserved` producer path and this runtime together. Synchronous `WorldHook` calls receive the detached audio branch and World queue/pause owners; late object/palette errors discard both. Existing pure APIs remain available. This interface supplies no CPU/VDP pricing or acceptance policy, and exposes enqueue operations only. See [world-sound-live verification](../../verification/world-sound-live/README.md).

`WorldFrameWithHooks` additionally exposes actual default-arena work boundaries and stages the resulting audio effects in the same transaction. The main clock is still borrowed and must be staged by the caller; no whole-world IRQ policy is inferred. See [arena work verification](../../verification/world-arena-work/README.md).

`BeginBattleSampleIRQ` couples actual RAM sampling-prefix timing, physical controller sampling, real PCM line-batch progress and the complete sound call on one transaction. Its result still requires the outer IRQ return stage before resuming World. See [sampling IRQ verification](../../verification/battle-sample-irq/README.md).
