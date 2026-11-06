# 实际包络回调接线验收

本项把现有 envelope work／calculator／volume 接到实际 score driver 和当前 transaction；同时验收固定 key／gate 计时。完整 voice caller、其它正常路径、Root／world／平台整场音频仍待完成。

- [先行失败场景](../../../../docs/research/md-yuyuhakusho/SCORE_OBSERVED_ENVELOPE_FAILURES.md)、冻结测试和真实缺少 API red 均在实现前保存。首轮新增 583 条记录通过，测试／原版 oracle 未因实现修改。
- 全部 580 个逻辑步骤的只读查询返回独立 tick pointer；对 offset／level／fixed word 的改动不能修改 catalog。
- 对实际 score working state 的 374 个 volume、206 个 pitch 逻辑步骤逐项驱动完整事务，使用真实 PSG owner，在实际 OpChipWrite 才写端口。局部原版 clock 边界一致，输出与最终 State 与 legacy 同输入相同，没有重复推进 cursor／scale。getter 在已关闭事务后不可用。
- 共同回归入口还覆盖 [key／gate](../score-clock-key-gate/) 的 288 个原版 profile。原生 race 与实际 Node/Wasm 各 72 个顶层测试、5,308 条通过记录，无跳过；两个相关包 vet 通过。旧 fixture／golden 未修改，没有重复其它游戏或完整长轨迹矩阵。

复跑使用仓库内新目录：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_observed_envelope_key.py --mode native --out .local/md-yuyuhakusho/envelope-key-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_observed_envelope_key.py --mode wasm --out .local/md-yuyuhakusho/envelope-key-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_observed_envelope_key.py --mode vet --out .local/md-yuyuhakusho/envelope-key-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_observed_envelope_key.py --audit
```

脚本将 Go telemetry、缓存和临时文件限制在本 worktree，绑定全部 current source、资源、原版和日志摘要。`validation.json` 同时绑定两个验收目录。阶段结束 clock 只声明对应 calculator／volume，不表示整个 voice／tick 已计费；详见 [OBSERVED_ENVELOPE.md](../../audio/scoreclock/OBSERVED_ENVELOPE.md)。
