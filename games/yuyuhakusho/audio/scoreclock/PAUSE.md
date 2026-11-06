# 暂停与恢复的固定 caller

`StartPause` 和 `ResumePause` 从 `EnterTick` 已完成实际确认／清除写入后的时钟开始，执行固定暂停或恢复 body。它们复用 canonical YM／PSG leaf，并在同一临时 service 中共享时钟、refresh deadline 和总操作预算。

```go
gate, err := scoreclock.EnterTick(clock, detachedWorldPause, limits)
paused, err := scoreclock.StartPause(gate.Clock, detachedMixed, detachedMixed,
    func(at uint64) (rootcontrol.PauseLevels, error) {
        return rootcontrol.CapturePauseLevels(detachedMixed.Snapshot().Chips.YM)
    }, limits)
// Commit paused.Levels to the unique Root pending owner only after all work succeeds.
resumed, err := scoreclock.ResumePause(resumeGate.Clock, pendingLevels,
    detachedMixed, detachedMixed, limits)
```

调用方须根据实际 gate route 选择一次相应 body；normal、hold、pause-start、resume 不能混用。hold 的早退已经由 `EnterTick` 计入，不能再次追加 return。normal 继续交给完整 driver 的后续阶段，本接口不代替它们。

暂停先保存／屏蔽中断，再申请实际硬件 bus，按实际 mailbox 及 BUSACK 重试；恢复调用前的中断状态后，从当前芯片捕获 24 个有效 TL。固定保存组的成本、三 channel／四 operator／两 bank 的 mute caller、保留组恢复和四路 PSG mute 都在原操作边界计费。源码中这些 JSR 是 PC 相对调用，成本为 18 CPU cycles，不能从反汇编显示的目标地址推成 absolute 调用。

恢复先释放同一硬件 bus，再按原顺序还原 24 个有效 FM TL。PSG 不额外恢复，后续音序继续驱动它。helper 的 YM 访问会释放 bus；整帧中随后由世界／IRQ 产生的长期 hold 仍属于外部 owner，不能由这里补造。

结果只有本次 clock 和必要的 pending levels。没有 CPU、RAM／寄存器镜像、原栈、原 PC、持久事件表或第二套全局时间。固定 32-byte 原保存组只作为成本边界，生产 payload 仍使用已验证的 24 个有效 TL。原 TL bit7 的硬件忽略投影沿用 Root 规则。

嵌套 YM、PSG 和 caller 操作使用同一个有界预算。错误可能出现在此前已有真实外部访问之后；丢弃所有 detached owners，并且只在整次调用成功后提交 World pause、Root pending payload、score 和 mixed 状态。接口不自动 flush 参考帧，也不处理额外 IRQ；调用方保持原就绪／调度边界。

原版自然 IO 证明使用已观察的外部 bus／mailbox／status 反馈，覆盖暂停 255 次、恢复 244 次 Main 访问的时刻和有效端口值。独立 mixed Runtime 另验证真实芯片／PCM 的捕获、静音、JSON 恢复及继续。自然记录未直接覆盖 PSG 尾部和最终 RTS；这部分依赖既有 PSG 原生 leaf 证明和固定静态成本，未扩写成整场逐采样一致。

测试先于实现。派生夹具初版误选世界请求写入，已按原实际 driver ACK／clear 点修正；v1、真实 red、提取脚本旧版和修正依据均保留，原自然记录及冻结 Go 测试未修改。来源、全部旧 leaf 回归和复跑入口见 [score-clock-pause](../../verification/score-clock-pause/)。
