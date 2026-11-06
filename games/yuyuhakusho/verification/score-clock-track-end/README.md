# 音轨终止与声部恢复验收

本项在统一实际声部入口中复用现有 held-note／新音符服务，接通有限音轨终止、跨声部恢复和 Root 的唯一 priority／queue effect。完整 driver 外层扫描、Root housekeeping、world IRQ、联合 checkpoint 和平台播放仍待完成。

原版 119 个 profile 覆盖全部有限 track-end、claimed guard，以及恢复目标未占用、owned inactive、active 和 noise 情况。两份现有固定 core 分别以 observer 关闭／开启执行，共 476 次；完整原生状态、RAM、PSG context 和跨 core 结果一致。保留 31,148 个源步骤、9,696 次 FM／bus 事件、33 次 PSG 写入与 110 次 priority／queue store。初始 auxiliary 使用已有 empty factory，在 master 0 持有总线并保留正式 pending PCM 描述符与请求。

测试从实际 score transaction 的 voice-entry working State 获取恢复目标，Root 的 `WorkingScore` hook 在每个真实操作前提供同一事务快照。此前已扫描 sibling 的状态不会重新推进一次。计时执行器驱动真实 mixed bus／chip；priority 和 queue 在唯一 Root effect lease 中执行，观察值、顺序、store 时刻与原版一致。完整 Go Output／State、Root 和队列与原接口同输入结果相同。

FM active target 的恢复直接进入 instrument seek，复用相同 25 次参数写入和 pan tail，跳过普通 claimed／bank 前缀。FM inactive target 不清除 claimed 或重载。PSG inactive target 仍恢复 flags，noise target 在对应意图处重写 noise latch。原版 primary marker 为负值，使用 voice-local bank；统一入口按实际 slot 选择资源分支，既有独立样本的历史 route 条件保留。

DAC track-end 的 source stack 结束位置与普通声部不同，证明它跳过整个 driver 剩余扫描和最终 return。运行时只返回 `EndsDriverTick` 语义并计入一次固定跳过／return 成本，不模拟源 stack。Root effect 另外验证遗漏、重复及过期回调均被拒绝；失败不提交 score。

失败计划、原版和 E2E 先于生产代码冻结，真实缺少 API red 保存在 `missing-api-red.jsonl`。实现前修正过 lease witness：必须选择 AB 的 queue-bearing voice 2／slot 15，而不是同音序另一声部，原文件和记录保留。首轮 119 条通过、两条失败记录来自 DAC raw marker word 被误当成 tonal BasePitch；原版 marker `0x8000` 与 canonical DACDescriptor 的类型不同。测试仅排除 DAC 的 tonal BasePitch 比较，原始文件、失败 SHA 与依据在 `test-projection-correction.json` 中保留，oracle 和生产代码未因此修改。第二轮新增 121 条记录全部通过。

最终原生 race 和实际 Node/Wasm 各通过 113 个顶层测试、7,485 条记录，无跳过。其中 scoreclock 为 84 个顶层／7,383 条，rootcontrol 为 29 个顶层／102 条；用时分别为 360.142 秒和 221.421 秒，三个相关包 vet 通过。未重复其它游戏或完整长音轨矩阵。

完整回归、原生／Wasm 命令、无损压缩日志和当前摘要见 `validation.json`。旧文件及摘要继续保留为历史证据，当前改动以本清单和实际完整回归为准。没有把该阶段验证当成完整对局声音已通过。

复跑使用仓库内新的输出目录：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_track_end_clock.py --mode native --out .local/md-yuyuhakusho/track-end-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_track_end_clock.py --mode wasm --out .local/md-yuyuhakusho/track-end-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_track_end_clock.py --mode vet --out .local/md-yuyuhakusho/track-end-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_track_end_clock.py --audit
```

原版重采另需现有 ROM／core 与资源 provenance。API 范围见 [VOICE.md](../../audio/scoreclock/VOICE.md)，剩余工作见[待办清单](../../../../docs/research/md-yuyuhakusho/TODO.md)。试玩服务保持原构建，尚未开启完整音频。
