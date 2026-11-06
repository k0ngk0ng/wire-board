# 完整新音符声部验收

本项把 parser、参数处理、乐器／level 写入、音符转换与清理、FM／PSG 后处理和 DAC 发布接入实际 score transaction，所有阶段共享同一 clock、refresh 和操作预算。它覆盖正常新音符 body；track-end／ownership、外层扫描、完整 Root／world 与平台播放仍待完成。

候选来自 23 份已接受音序、115 个实际声部，观察 120,742 次 driver call 和 85,396 次新音符调用，保留 1,081 种路径／owner／资源代表。仅按结构抽样会漏音乐 descriptor，采样规则在生产改动前已补齐全部六种 descriptor；另加入 claimed 和 DAC feedback 分支，共 1,095 个 profile。候选生成源码保存在 `case-export-source.go.txt`。

`test-first.json` 记录原先行测试、失败计划、采样脚本与原版 oracle 的摘要，真实缺少 API red 在实现前保存。第一轮 521 条通过、578 条失败，第二轮 821 条通过、278 条失败；完整日志保留。实际实现漏接 tie cleanup 的语义观察，已修正生产状态机。

测试初始化与投影另有明确订正，依据及原始文件保存在 `test-setup-correction.json`、`*.before-setup-correction.txt` 与采样脚本归档中。原始 `native-note-voice-full.json.gz` 保持原样，当前 oracle 为 `native-note-voice-canonical-aux.json.gz`。

- 原版安静存档位于 auxiliary idle block 的中间，原 pc 为 `0x60`、cycles 为 75；`mixed.New` 从既有 empty factory 的 `0x5d`、cycles 0 开始。当前采样复用已有 `command.initial`，并与实际 mixed test 一样在 master 0 持有总线，然后才设置给定 main origin。两个不同初态的结果不再直接比较。
- Pending RateRequest 17／flags 5 继续保留，当前描述符的四字节 metadata 明确绑定到正式音乐 descriptor；Go 初始状态补上对应 `SingleDescriptor`。没有把实际 pending PCM 改成固定空闲反馈。
- 原版 byte 25 对 FM 保存 algorithm／feedback 的打包值，对 PSG 保存 noise latch。比较 canonical 三位 `LoadedAlgorithm` 时只取 FM 的有效位；PSG 的 FM algorithm 字段应保持原值。原始字节未修改。
- disabled pan 的旧 counter 不会被源程序观察，重新启用时会覆盖。canonical control 规范化为零，因此测试中的期望也明确为常量零；active pan counter 继续直接比较，未从实际结果复制期望。

订正后的原版 profile 在两份现有固定 core 中分别以 observer 关闭／开启执行，共 4,380 次，完整原生状态、RAM、PSG context 及跨 core 结果相同。当前保留 607,194 个原版步骤、174,224 次 FM／bus 事件和 850 次 PSG 写入。唯一 queue-replace 控制紧接有限音轨的 track-end，因此没有作为正常新音符样本；该路径继续显式 unavailable，随终止处理完成。

Go E2E 消费目标声部内全部真实操作，驱动实际 mixed bus／chip／PCM，逐项比较访问值、顺序、时刻、结束 clock 和声明的参数／counter 投影。整体 Output／State 与原 driver 在相同实际 feedback 序列下相同。音乐发布还比较 metadata 组的首／末 store 时刻，预算耗尽与外部写入失败须回滚 score 并关闭 transfer。第三轮新增 1,099 条记录全部通过。

最终完整 scoreclock 回归在原生 race 与实际 Node/Wasm 中各通过 82 个顶层测试、7,262 条记录，无跳过；用时分别为 335.795 秒和 197.296 秒，score／scoreclock 的 vet 通过。范围包含现有 721 组 held-note 和全部旧 leaf 检查，未重复其它游戏或完整长音轨矩阵。

完整回归与摘要见 `validation.json`。日志无损压缩后保留原字节 SHA-256；旧验收清单中绑定旧文件的摘要继续作为历史记录，本项当前源码以新清单及完整回归为准。

复跑使用仓库内新的输出目录：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_note_voice_clock.py --mode native --out .local/md-yuyuhakusho/note-voice-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_note_voice_clock.py --mode wasm --out .local/md-yuyuhakusho/note-voice-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_note_voice_clock.py --mode vet --out .local/md-yuyuhakusho/note-voice-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_note_voice_clock.py --audit
```

原版重采另需现有 ROM／core、候选与资源 provenance，不能用 Go 回归日志替代。API 范围见 [NOTE_VOICE.md](../../audio/scoreclock/NOTE_VOICE.md)，当前剩余工作见[待办清单](../../../../docs/research/md-yuyuhakusho/TODO.md)。试玩构建尚未启用完整音频。
