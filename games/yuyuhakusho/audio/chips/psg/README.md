# 集成式 MD PSG 边沿生成

本包支持固定参考 profile `IntegratedHQ150Stereo`：MD 的集成式 PSG、每步 240 个 master cycles、双声道 150% 振幅。它独立维护锁存、三个音调分频、噪声反馈和延迟音量提交；输出有序振幅边沿，不执行 CPU、ROM、原生内核或音序命令。

```go
chip, err := psg.New(psg.IntegratedHQ150Stereo, psg.ResetState())
edges, err := chip.Write(psg.Event{Cycle: 0, Data: 0x90})
more, err := chip.Advance(896040)
saved := chip.Snapshot()
restored, err := psg.New(psg.IntegratedHQ150Stereo, saved)
```

`Event.Cycle` 和 `Edge.Cycle` 使用同一个绝对时间线，原点由调用方选择；初态的 `NextTick` 和四个 `NextEdge` 明确保存内部相位。`Advance` 只推进严格早于目标时间的边沿。写入先同步再修改寄存器，尚未提交的振幅变化由 `PendingDelta` 保存；`Level` 是唯一的未放大振幅 owner，输出振幅和分频增量均为临时派生值。

`FrameBoundary(end)` 表达原参考帧的坐标重基，只推进尚未运行的内部时间，不拥有第二个帧计数或重采样器。如果原输入已超前帧尾不超过 1,008 cycles，此方法不重复运行。PSG 已产生的超前边沿必须保留给重采样器的尾缓冲，不能套用 FM 的临时输出丢弃规则。

相同时间的边沿保持原通道遍历顺序，不能合并：HQ FIR 的分数插值逐笔取整。每次最多推进两个视频帧；倒序、时钟溢出或超过上限均原子拒绝。`Snapshot` 与 `New` 全部使用值类型，不保存指针、原 context 或历史输入。

验证包含原生 API 批量推进与逐内部时钟观察的完整缓冲、PCM 和 typed 状态对照，以及独立边沿重新注入原生重采样器。Go native／race／Wasm 逐项比较 5 个场景、1,616 个操作检查点和 51,139 笔振幅边沿；白噪声覆盖实测的 57,337 次移位周期。详见 [研究记录](../../../../../docs/research/md-yuyuhakusho/FMPSG_PSG.md)。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/fmpsg_psg_verify.py --mode all
```

原参考程序及其 `psg.c` 仅作为本地状态和输出 oracle；本实现没有复制、移植或包含该 C 实现。振幅表按 2 dB 衰减公式量化。该 profile 的原生行为不能直接表述为所有实体硬件的精确行为。
