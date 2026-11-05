# 参考总线访问效果

`ResolveBankAccess` 计算给定一次 bank 访问后，固定参考内核施加的 Z80 等待、68000 延长和等待标记。调用者提供当前参考帧内的周期与 DMA 区间；本包不持有时钟副本、不决定下一次访问、不执行 CPU 或固件。

```go
effect, err := phaseclock.ResolveBankAccess(frameCycle, phaseclock.DMA{
    End: dmaEnd,
    Mode: dmaMode,
    Remaining: dmaRemaining,
})
// Apply only after checking err. SetWaitFlag is an OR effect on the owner flag.
```

`frameCycle` 是当前参考视频帧内的 master cycle，已包含原 memory operation 的基础成本，尚未包含本次 bus effect。不能传入全局绝对时间：每帧 896,040 周期不能被 7 整除。mode 0／1 的 DMA 可以将访问等到 `End`，mode 2／3 不造成这种 ROM-bus 等待。`Effect.At` 是等待结束的访问相位；`Z80After` 加入 45 周期，`M68KDelay` 是同一相位的 70／77 周期。`SetWaitFlag` 只请求置位，不清除调用者已有标记。

未知 mode 或超过 uint32 可表示的恢复周期返回错误和零 effect。该合同是参考 profile 的安全输入边界，不宣称原内核或真实硬件会在同一点拒绝。

两个完整自然原版流的 851,037 次 bank 读取逐项匹配，含 357 次 DMA 等待。原生 race 与 Wasm 各三项检查通过；mode 2／3、非零 DMA Remaining、已置位 wait flag 和 uint32 末端另经两个原生内核的 11 个声明私有状态核对。自然流本身只观察到 mode 0／1、原 zstate 1；不能把私有范围写成自然覆盖。

完整证据与限制见[总线时基研究](../../../../docs/research/md-yuyuhakusho/AUDIO_PHASE_CLOCK.md)及 [validation.json](../../verification/phase-clock/validation.json)。运行时不读取本目录测试夹具、原 PC 时间表或 ROM；夹具仅由 `_test.go` 嵌入。
