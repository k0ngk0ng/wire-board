# 普通 PCM 发布验收

本项覆盖已解析 ordinary descriptor 后的状态采样、释放总线、仲裁、再次申请总线、metadata 发布和最终 return，并接入真实 Root 外部命令调度。完整 Root／world 时序、联合 checkpoint 和平台整场音频仍未完成。

- [先行失败场景](../../../../docs/research/md-yuyuhakusho/SCORE_CLOCK_ORDINARY_FAILURES.md)、最终原版 oracle 和 Go E2E 均先于生产实现冻结。`missing-api-red.jsonl` 保留真实缺少 API 的编译失败；之后测试和 oracle 未因实现修改，首轮实现的 155 条新增记录全部通过。
- 原版语义依据保留已有 13,036 条仲裁记录。新计时矩阵使用 122 个正式资源及其八组 command metadata，按全部正常 canonical 输入枚举可达的 22 种分支形状，选择代表并加上每个资源的 accepted closure，共 140 个 profile、两份既有 pinned core、560 次执行。包含私有 callable 的 code，不以此证明正常世界 producer 的可达性。
- `native-ordinary-full.json.gz` 保存 9,597 步、4,158 条原始 Main 访问、690 次状态读取和 1,564 次 metadata 写入；publication 为 124 组 12-store、6 组 10-store、2 组 8-store，另 8 组拒绝。observer 开关前后完整状态、RAM、步骤一致，两 core 的全部结果一致。Root choice 的 134 个阶段记录使用实际 RAM 前后字节和 pinned step 的 refresh 后时刻，不伪称额外 RAM hook。
- 首次采集漏录 single 的四个 bank/header 目标字节，尚未编写 Go 测试或实现时即发现并修正 observer，随后增加 3／5 次读取及 0／8／10／12 次 metadata 覆盖断言。原不完整 oracle、C／Python 源版本及说明保存在 `incomplete-before-coverage-fix/`；原 ROM／core／生产代码未因此修改。
- Go E2E 使用正式四资源、实际 mixed Runtime、唯一 Root ChoiceBits 和 canonical resourcebridge／engine 决定，逐项比较状态读取、两次 hold、choice、metadata 首末时刻、所有结果字段与结束 clock。额外验证实际 Root 分派、后续 mode 改变仍保留 sampled StartDual、错误／重复 intent、共享预算、不可变 metadata、时间／hold／choice 约束和 detached 分支隔离。
- `final/` 的原生 race 与实际 Node/Wasm 各 59 个顶层测试、3,160 条通过记录，无跳过；五个相关包 vet 通过。原生产文件、已有测试／fixture／golden 未修改，没有重复运行其它游戏或长轨迹矩阵。

复跑使用仓库内新目录：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_ordinary_clock.py --mode native --out .local/md-yuyuhakusho/ordinary-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_ordinary_clock.py --mode wasm --out .local/md-yuyuhakusho/ordinary-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_ordinary_clock.py --mode vet --out .local/md-yuyuhakusho/ordinary-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_ordinary_clock.py --audit
```

脚本将 Go telemetry、缓存和临时文件限定在 worktree，并绑定来源、正式资源、oracle、环境与日志摘要。`native-capture.json` 保存身份和原版命令。阶段边界及限制见 [ORDINARY.md](../../audio/scoreclock/ORDINARY.md)；profile 声明初始 held idle player、无额外 IRQ／DMA／duck，不表示整场声音已经验收或启用。
