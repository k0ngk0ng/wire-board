# 包络 Go 绑定与计时验收

本项将已冻结的包络来源接入 Go：580 个逻辑 work 绑定、五资源 identity、pitch calculator 计时与分阶段实际 PSG volume。它不表示 pitch caller／terminal key-off、Root／world／整场音频已经完成。

- [失败场景](../../../../docs/research/md-yuyuhakusho/SCORE_ENVELOPE_WORK_FAILURES.md)、Go E2E 与两份原版 oracle 先于实现冻结，摘要见 `test-first.json`，真实缺少 API 的编译失败保留在 `missing-api-red.jsonl`。首轮新增 1,034 条记录全部通过，冻结测试／oracle 未因实现修改。
- 580 个 work 全部与正式逻辑 cursor 绑定，包含 prefix-end／cycle 别名和缺项、重复、未知字段、非法位置、错误 catalog 的拒绝。绑定不改变声音 tick、State 或原 envelope cursor。新 raw 资源进入五资源共同 identity，四／五资源 PCM checkpoint 拒绝混用。
- 两份原版 fixture 共 1,030 个 profile、4,120 次 native 执行，来源见 [score-envelope-source](../score-envelope-source/)。Go 逐项比较全部 pitch calculator／volume 边界、256 个 signed scale、restart、none-envelope、volume clamp／hold／key-off／note／claim／tie-gate 和实际 PSG port 值／时刻。现有 `param.PitchWord` 的声音结果与原版 word／scale 一致。
- volume Begin 不提前输出；实际 Write 到达时才调用唯一 mixed PSG owner，Finish 消费唯一 return。提前结束、错误后继续、共享预算、overflow、未知 work／owner 等约束通过。terminal pitch 明确停在 key-off leaf 之前。
- 原生 race 与实际 Node/Wasm 各通过 4,436 条记录：scoreclock 67 个顶层测试／4,435 条记录，resourcebridge 既有兼容检查 1 条；无跳过，三个相关包 vet 通过。没有重复执行其它游戏或完整长轨迹矩阵。

正式文件 `data/audio-score-envelope-work.json.gz` 为 1,948 bytes，解压后 58,364 bytes。压缩 SHA-256 `b2d0d9363bef8e47832c67be65bbf380291a0e4a87c9e4b35c78bbf9445db028`，还原内容 SHA-256 `b4894e6670be37251ba21796fdf48a8ab321033c6d52c3731fa4cf3f8361003f`；再次从 pinned ROM 导出后 work／provenance 均字节一致，见 `reexport.json`。没有上传 OSS 或修改正式构建。

复跑使用仓库内新目录：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_envelope_clock.py --mode native --out .local/md-yuyuhakusho/envelope-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_envelope_clock.py --mode wasm --out .local/md-yuyuhakusho/envelope-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_envelope_clock.py --mode vet --out .local/md-yuyuhakusho/envelope-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_envelope_clock.py --audit
```

脚本限制 Go telemetry、缓存与临时文件在 worktree，并绑定当前代码、正式资源、原版 fixture 与日志摘要。调用与 owner 边界见 [ENVELOPE.md](../../audio/scoreclock/ENVELOPE.md)。
