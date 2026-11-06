# 控制、渐弱与暂停整拍

`AdvanceDriverTick` 将 01／05／04、活动 fade 和 pause start／hold／resume 接入既有 queue／dispatch／voice 执行器。固定 level scan、PSG mute、instrument level 与即时 PSG 音量复用同一服务；整次调用只使用一个 clock、refresh 和预算。`AdvanceRequestTick` 的普通 PCM／FF 闭包保持不变。

Root 的 `ReadPause` 在实际计时点读取唯一 owner，返回值必须等于一次同步读取的结果。`PauseWrite`、`FadeWrite`、`BulkClear` 与 selected effect 使用同步一次性 mutation lease，接收与观察相同的版本、序号和阶段；遗漏、重复、过期、伪造或失败即终止。连续 bulk clear 仍是 no-extra-IRQ profile 的明确规则效果，不声称逐 long 中断可见性。

暂停 start 先取得实际总线，再通过 `PauseLevelSource.ReadPauseLevels` 捕获实际 YM 的 24 个 total level，随后按原顺序静音 FM／PSG 并提交唯一 pending payload。hold 不推进音乐参数、fade、请求或端口。resume 恢复保存的 FM level，保留 PSG 静音，成功后释放 payload。`score.State.Ticks` 统计实际 driver 调用，包括暂停调用；它不表示音乐参数在暂停期间推进。

01／05 只设置原版 fade 计数并清音乐 DAC flags，不停止独立 PCM。fade 使用真实 byte 增量与 FM 符号／PSG 16 级边界，保留 claimed、note-off、tie／gate 抑制。04 完成 header／score clear、selected reset、六次 key-off、24 次 FM mute 和四次 PSG mute；后续声部扫描保留实际 return 边界。

所有 owner 必须来自同一可丢弃分支，成功后才联合提交。失败不提交 score，但可能已修改 detached Root、queue、PCM 或 chip，调用方必须整体丢弃。返回的字段和 intents 仅用于诊断，不能再次播放。world IRQ、请求生产时刻、联合 checkpoint 和平台连续播放仍待接通。

完整原版调用、计数预期订正、混音域修复和复跑说明见[控制验收目录](../../verification/score-clock-driver-control/)及[独立芯片时间域](../chips/INDEPENDENT_LANES.md)。
