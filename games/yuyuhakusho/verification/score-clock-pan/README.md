# 声像路由验收

本项覆盖普通 voice pan、乐器 load 的 pan 尾部和第六声道的 DAC 恢复声像重定向。它补充已有 timed instrument transfer；完整 driver、世界 IRQ 和平台整场音频仍未完成。

- [先行失败场景](../../../../docs/research/md-yuyuhakusho/SCORE_CLOCK_PAN_FAILURES.md)与 `test-first.json` 在生产实现之前保存；`frozen-missing-api-red.jsonl` 保留真实缺少 API 的编译失败。
- `native-pan-full.json.gz` 是两份既有 pinned core 的 108 个私有场景对照。每个场景分别关闭／打开 observer，共 432 次执行；完整状态、RAM 和步骤一致。身份、命令和来源摘要见 `native-capture.json`。
- Go E2E 使用实际 mixed Runtime 独立产生反馈，比较 Main 总线、request marker、恢复声像、YM status／写入的每次时刻与值，以及最终 clock。覆盖 marker 低位非零、AMS／FMS、claimed、分阶段 intent、实际 bus ownership、预算和失败分支隔离。
- `final/` 保存整个 scoreclock 包的原生 race、实际 Node/Wasm 与 vet 结果。原生和 Wasm 各 45 个顶层测试、2,428 条通过记录，无跳过。现有原版 fixture、golden 和旧测试未修改。

复跑使用仓库内新目录，避免覆盖既有结果：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_pan_clock.py --mode native --out .local/md-yuyuhakusho/pan-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_pan_clock.py --mode wasm --out .local/md-yuyuhakusho/pan-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_pan_clock.py --mode vet --out .local/md-yuyuhakusho/pan-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_pan_clock.py --audit
```

脚本将 Go telemetry、缓存和临时文件限定在本 worktree 内，并绑定每次检查的来源摘要、命令、环境和日志摘要。无须重新运行模拟器即可复跑 Go 验收；原版 capture 脚本只用于必要的来源复核，不是生产运行时。

此原版矩阵仅声明合法 FM voice row、初始 held-bus／mailbox0、无额外 IRQ 的 profile；不以私有入口证明自然可达性，不覆盖特殊非 voice header-pointer 路径。调用阶段和提交边界见 [PAN.md](../../audio/scoreclock/PAN.md)。
