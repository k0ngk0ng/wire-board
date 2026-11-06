# Score control oracle 先行失败场景

只读现有 Root action 与 canonical-v2；不新 CPU 捕获，不改 score Go、Observed、旧 goldens 或 catalog。

- 将 Root 的 fade／pause／queue／priority／selected 字段写入 score State 或把其变化当作 score action 变化。
- 将动作窗口的 after 偷换为整帧结束状态，混入 Tempo、voice clock、envelope 或新音符推进。
- StopDACFlags 清 owner／channel／参数、重置 DAC sample，或影响其他 voice；没有完整 after 采样却声称完全直接观察。
- FadeStep 调用普通 volume envelope 推进，而原操作直接使用 base level；改变 note wait、gate、modulation／pan／envelope cursor 或事件位置。
- 将 FullClear 原窗口的 31 YM＋4 PSG 写入归给 state-only ResetScore，或把 Root 所有字段一并归零。
- 用 Go 或本地规则实现制造 original expected values；原 snapshot、原 byte 数据和源指令证明必须另列 authority。
- 从原序列 PC 推出 GoState 并直接注入；初始化应通过合法 whole-tick setup 重建，PC 只作 private provenance。
- 无视窗口位于原 Tempo 阶段之后，把 Tempo0 的合法 setup 证明外推到非零 Tempo 的任意 orchestrator 时序。
- 将 FM 原生 level14～84 的样本表述为覆盖127→128、255→0、TL carry 或 claimed 抑制边界。
- 将 pause case 的 idle fade 当作 active-fade／pause 组合证明。
