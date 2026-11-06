# 音符收尾与颤音验收

本项补齐正常音符的 FM／PSG cleanup、held-note modulation，以及 DAC commit 到采样发布前的前缀。完整参数处理、PCM 发布、世界 IRQ 与平台整场音频仍待完成。

- [先行失败场景](../../../../docs/research/md-yuyuhakusho/SCORE_CLOCK_COMMIT_MOD_FAILURES.md)、原版 oracle 和 `test-first.json` 均先于生产实现保存。`missing-api-red.jsonl` 保留实际缺少新 API 的失败；冻结测试和 oracle 未因实现修改。
- `native-commit-mod-full.json.gz` 覆盖 FM／PSG tie、modulation enable、DAC 休止／有效采样、delay 0／1／255、rate 0／1／2／255、steps 0／1／255 及四种 refresh profile，共 188 个场景、两份 pinned core、752 次执行。身份、命令及摘要见 `native-capture.json`。
- 被动 trace 前后的完整 native 状态、RAM 和寄存器一致，两份 core 的 1,404 个指令步骤、寄存器、RAM 和结束时刻一致。Go E2E 按原操作边界计算并比较每个 profile 的最终 cycle 与 refresh，校验预算耗尽、矛盾字段、未知 family 和 overflow 拒绝。
- `final/` 保存整个 scoreclock 包的原生 race、实际 Node/Wasm 与 vet 结果。原生和 Wasm 各 47 个顶层测试、2,618 条通过记录，无跳过。现有生产文件、测试、fixture 和 golden 未修改。

复跑使用仓库内新目录，避免覆盖既有结果：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_commit_modulation_clock.py --mode native --out .local/md-yuyuhakusho/commit-mod-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_commit_modulation_clock.py --mode wasm --out .local/md-yuyuhakusho/commit-mod-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_commit_modulation_clock.py --mode vet --out .local/md-yuyuhakusho/commit-mod-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_commit_modulation_clock.py --audit
```

脚本将 Go telemetry、缓存及临时文件限定在本 worktree，绑定检查时的来源、命令、环境和日志摘要。无需重新运行模拟器即可复跑 Go 验收。

该原版矩阵是无额外 IRQ 的固定声明入口，不证明私有入口的自然可达性，不验证整场声音输出。成本边界见 [COMMIT_MODULATION.md](../../audio/scoreclock/COMMIT_MODULATION.md)。
