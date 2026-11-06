# 乐器查找与分阶段传送

`BeginInstrumentTransfer` 从实际 claimed guard 开始，使用唯一所选乐器的 `InstrumentWork`、只读 `FMPatch` 和当前 domain／channel／base／loaded algorithm，准备本次临时传送。它不复制 score 或 chip 状态，也不保存原 bank、index、PC、RAM、原 stack 或事件历史。

```go
transfer, err := scoreclock.BeginInstrumentTransfer(clock, request,
    detachedMixed, detachedMixed, limits)
for _, expected := range transfer.ExpectedWrites() {
    err = transfer.WriteNext(expected)
}
nextClock, err := transfer.Finish()
```

实际 score consumer 应在每个对应的 chip intent 到达时调用一次 `WriteNext`，不能在 resource callback 中提前把全部寄存器发完。`ExpectedWrites` 返回独立值切片；`WriteNext` 在任何该阶段的外部效果前验证预期意图。`Finish` 拒绝提前结束，消费最后循环／非写入尾部，完成后或失败后不能重入。

load 包含完整的 bank 选择、逐记录查找、算法写回、25 个参数／TL 写入的 caller、六种 FM channel 的路由及共享 canonical YM 服务。完成点在 pan-routing 尾部之前，不重复 pan、DAC 重定向或 return。level 更新则使用实际 loaded algorithm 的 carrier mask，分别处理负 base、原 TL 高位 carry、未写入 operator 和最终 return。

三个 domain 的固定选择路径和 `records_skipped` 均计入原操作成本，所有阶段与嵌套服务共享同一 clock、refresh deadline 和预算。循环／请求列表只存在于这次可丢弃操作中。公开 `ServeInstrument` 是同一分阶段实现的便捷完整调用，不是另一套规则。

错误可能在此前已访问真实 bus／chip 后发生。调用方必须丢弃全部 detached owners，并只在整体成功后提交 canonical 状态。接口不自动 flush 参考帧、处理额外 IRQ 或替代世界／声音 producer 的线性化。

原程序在既有两份 core 上的 220 个声明 held-bus／mailbox0 场景，包含八种算法、六种 channel、音量／carry、claimed、同音色别名和较长查找。实际 mixed Runtime 独立产生 bus／mailbox／YM busy 反馈，按全部已观察的 Main 访问、有效端口值与最终时刻比较。硬件忽略位保留在 raw oracle，生产仅使用 typed chip-effective 值。

先行失败、冻结测试、原生证据和复跑方式见 [score-clock-instrument](../../verification/score-clock-instrument/)。验证入口为 `scripts/yuyuhakusho/verify_instrument_clock.py --mode native|wasm|vet --out <新的仓库内目录>`，离线审计用 `--audit`。这不表示 pan、其它参数、完整 driver／世界 IRQ 或平台整场音频已经通过。
