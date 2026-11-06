# 乐谱现场 PCM 接线验收

本项将实际 score feedback、canonical intent 与 timed PCM／pan leaf 连接，保留唯一 score／PCM／chip owner。它验证阶段连接，不提供整拍成本，不表示 Root、world IRQ、联合 checkpoint 或平台整场声音已完成。

- [先行失败场景](../../../../docs/research/md-yuyuhakusho/SCORE_OBSERVED_PCM_FAILURES.md)、补充原版 oracle 和 `test-first.json` 均先于实现保存。`missing-api-red.jsonl` 保留真实缺少完整 pan 投影与阶段 API 的失败。
- 48 个补充原版 profile 覆盖六种合法 FM voice row、marker 0／1／128／255、不同保存字段／YM 调用值；两份既有 core 共 192 次执行。observer 开关前后完整状态与 RAM 一致，两 core 的 1,442 步及 574 条原始 Main 访问一致。身份、命令和摘要见 `native-capture.json`。
- E2E 使用正式四份资源与实际 score driver／transaction，覆盖六个 descriptor 的 24 种音乐接受／拒绝组合、四种正式 FM1 自动声像、48 个原版字段／调用值路由和九种错误／重复事件。音乐与声像相位的结束时刻来自既有原版边界，未将其它未计费阶段当作零成本。
- 原生 race 与实际 Node/Wasm 均通过：scoreclock 各 55 个顶层测试／3,005 条通过记录；既有 observed／transaction 检查各 11 个顶层测试／25 条记录；四条未改动的原版轨迹共 546 帧／5 条通过记录。无跳过，四个相关包的 vet 通过。没有重复执行其它游戏或完整 42 轨迹矩阵。

本轮保留两类失败与修正：初次编译发现接口错误地要求 pan 分支同时实现 MusicBus，生产接口已按实际分支收窄，测试保持原样。随后自动声像 E2E 使用了不存在的已拥有 FM6 声部，恢复阶段即被正确拒绝。正式音乐 header 均为 DAC、FM1～FM5 与三个 PSG；测试改为实际 FM1 自动声像，并在新增 `BeginObservedPan` 实现前保存新的 missing-API red。旧冻结测试全文、原失败、准确 diff、正式 header 清单及前后摘要保存在 `observed-pcm-test.before-setup-correction.go.txt`、`second-native.jsonl` 和 `test-setup-correction.*`；音乐、原版 48 profile 及错误事件断言未修改，任何旧 fixture／golden 未修改。

复跑使用仓库内新目录：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_observed_pcm.py --mode native --out .local/md-yuyuhakusho/observed-pcm-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_observed_pcm.py --mode wasm --out .local/md-yuyuhakusho/observed-pcm-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_observed_pcm.py --mode vet --out .local/md-yuyuhakusho/observed-pcm-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_observed_pcm.py --audit
```

脚本将 Go telemetry、缓存和临时文件限定在 worktree，并核验旧冻结测试存档、合法初始化修正、正式资源、源码和日志摘要。`*.before-observed-pcm.txt` 保存三个原生产文件，旧验收 manifest 继续保留历史摘要；本轮当前来源以新 `validation.json` 为准。

补充原版 profile 声明初始 held bus、mailbox 0、无额外 IRQ，不证明私有 FM6 入口的自然可达性。调用阶段与限制见 [OBSERVED_PCM.md](../../audio/scoreclock/OBSERVED_PCM.md)。
