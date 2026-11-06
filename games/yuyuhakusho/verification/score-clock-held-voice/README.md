# 连续 held-note 声部验收

本项将既有 gate、自动声像／PSG volume、modulation、pitch calculator、key 和音高写入接成同一个实际 score 声部调用。它覆盖保持音符时的整个 FM／PSG body，尚未包括新音符、parser／control、track-end、外层扫描及完整 Root／world 和平台播放。

测试、[先行失败场景](../../../../docs/research/md-yuyuhakusho/SCORE_CLOCK_HELD_VOICE_FAILURES.md) 和原版 oracle 在生产改动前冻结。`test-first.json` 绑定这些文件；`missing-api-red.jsonl` 保留真实缺少 API 的失败。首轮实现 724 条记录全部通过，随后收紧操作元数据校验；冻结测试、oracle、旧 golden 和 score driver 均未修改。首轮完整日志无损保存在 `first-implementation-native.jsonl.gz`。

原版固定入口连续执行 721 个 profile：全部 580 个逻辑包络步骤，以及 141 个 caller／guard 分支。两份现有固定 core 分别在 observer 关闭／开启时执行，共 2,884 次；完整原生状态、RAM、PSG context 和两份 core 的结果均相同。保留 52,531 个源步骤、5,268 次 FM／bus 访问和 767 次 PSG 写入。初始 held bus、mailbox 0、正常 FM3 mode 为零且无额外 IRQ，是这些 profile 的边界条件。

每个 Go E2E 通过实际 score transaction 推进全体声部扫描，由 `HeldVoiceTransfer` 消费目标声部内每个真实操作，并驱动真实 mixed bus／chip owner。整段访问值、顺序、时刻和结束 clock 均与原版一致；NoteWait、gate、note-off、scale、调制与 pan counter 的最终投影也与原版一致。完整 Go Output／State 与同输入的原 driver 相同。FM 高频率寄存器的硬件忽略位只在比较投影中屏蔽，oracle 保留原始值。

预算 80 的真实 FM 路径在已经开始外部访问后耗尽，证明跨 gate／pan／modulation／calculator／两次频率写入的预算没有被重置。外部 chip 失败同样使 score transaction 回滚并关闭，失败或提前结束后 transfer 不可继续。外部 detached owner 已发生的访问不会被 score 回滚，联合提交方必须丢弃它们。

四个旧生产文件仅增加共享 service 的私有入口，固定逻辑体和公共校验保留；重构前文件及摘要见 `*.before-shared-service.txt`／`compatibility.json`。最终完整 scoreclock 回归在原生 race 与实际 Node/Wasm 中各通过 78 个顶层测试、6,163 条记录，无跳过；耗时分别为 190.206 秒和 99.196 秒。score／scoreclock 的 vet 通过。未重复其它游戏或整段长音轨矩阵。

`validation.json` 绑定当前源文件、输入资源、冻结证据、命令和完整测试日志；压缩日志还原后的 SHA-256 与 command 记录一致。旧验收清单中绑定这四个旧文件的摘要保留为历史证据，本次以新清单和完整回归为准。

复跑使用仓库内新的输出目录：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_held_voice_clock.py --mode native --out .local/md-yuyuhakusho/held-voice-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_held_voice_clock.py --mode wasm --out .local/md-yuyuhakusho/held-voice-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_held_voice_clock.py --mode vet --out .local/md-yuyuhakusho/held-voice-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_held_voice_clock.py --audit
```

原版重采需要已有的本地 ROM／core 和资源 provenance，入口为 `scripts/yuyuhakusho/score-clock-held-voice-capture.py --out .local/md-yuyuhakusho/held-voice-native-repeat`。Go 回归读取冻结 oracle，无须模拟器。本次已经结束的私有采样副本、两份 observer dylib 和空目录已清理，共 136,613 字节；正式 oracle、原版工具和正在运行的试玩服务保留。

API 及联合事务限制见 [HELD_VOICE.md](../../audio/scoreclock/HELD_VOICE.md)，后续工作见[项目待办](../../../../docs/research/md-yuyuhakusho/TODO.md)。试玩构建尚未开启完整音频。
