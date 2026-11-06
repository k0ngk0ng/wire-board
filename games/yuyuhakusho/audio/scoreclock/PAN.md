# 分阶段声像路由

`BeginPanRouting` 覆盖普通 voice 声像入口与乐器 load 的声像尾部。输入使用当前 FM channel、claimed 状态和唯一 score owner 的完整 `score.Pan`，保留左右声道、AMS 和 FMS。instrument 入口要求上游 claimed guard 已通过；普通 voice 入口包含该 guard。

第六声道在实际持有辅助总线后读取当前 PCM request marker。整个字节非零即更新 PCM 的自然结束恢复声像；为零则释放总线、恢复中断状态，再进入普通 YM B4 写入。此处仅等待 BUSACK，不读取或等待 busy mailbox。marker 与恢复声像仍分别由 `pcmcommands.State.Global.StartMarker` 和 `RestorePan` 唯一持有，mixed 接口不缓存副本。

```go
routing, err := scoreclock.BeginPanRouting(clock, request,
    detachedMixed, detachedMixed, limits)
effect, hasEffect := routing.ExpectedEffect()
// 对应 score intent 到达时验证并应用该 effect。
if hasEffect {
    err = routing.Apply(effect)
}
nextClock, err := routing.Finish()
```

Begin 停在对应 effect 之前，不能在反馈回调内提前发出 YM 写入或恢复声像。重定向分支的 Apply 写恢复声像，Finish 再释放同一总线；普通分支的 Apply 使用已有 canonical YM 服务，Finish 消费最终 return。全部阶段共享本次操作的 clock、refresh deadline 和预算。提前 Finish、错误或重复 Apply，以及外部失败后继续操作均被拒绝。`ServePan` 只是同一分阶段实现的完整调用。

每次 ZRAM 访问的额外 7 master cycles、channel 分支、caller、BUSACK、SR 恢复及 return 均按原版操作边界计费。helper 仅保存可丢弃的临时工作，不持有原 ROM、PC、RAM、原程序、持续事件历史或额外声像 owner。发生错误后调用方必须丢弃相关 detached owners；本接口不自动提交整场状态或 flush 参考帧。

原版对照使用既有两份 pinned core，覆盖两个入口、六种 FM channel、marker 0／1／128／255、两种完整声像字节及 voice claimed，共 108 个场景、432 次执行。observer 开关前后的完整 native 状态、RAM 和步骤一致，两份 core 的访问与结果一致。Go E2E 使用实际 mixed Runtime 独立生成总线、PCM marker 和 YM busy 反馈，逐项比较全部 Main 访问与最终 clock。

这些是声明的合法 FM voice row、初始 held-bus／mailbox0、无额外 IRQ 的私有 profile，不证明额外 header-pointer 路径的自然可达性，也不代表完整 driver、世界 IRQ 和平台整场音频已验收。冻结测试、原版证据和复跑入口见 [score-clock-pan](../../verification/score-clock-pan/)。
