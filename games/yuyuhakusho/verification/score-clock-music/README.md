# 音乐 DAC 仲裁与发布验收

本项将有效 DAC note commit、现场 blocked／priority 仲裁、accepted metadata 发布和最终 return 连接到真实 mixed Runtime。完整控制参数、包络、普通 PCM 发布、世界 IRQ 和平台整场音频仍未完成。

- [先行失败场景](../../../../docs/research/md-yuyuhakusho/SCORE_CLOCK_MUSIC_PUBLICATION_FAILURES.md)、原版 oracle 和 `test-first.json` 均先于实现保存。`missing-api-red.jsonl` 保留最后冻结版本的真实缺少 API 失败；之后测试和 oracle 未修改。
- `native-music-full.json.gz` 使用正式六个音乐 descriptor 的原始绑定，覆盖 marker 0／1／128／255、ModeFlags 0／33／64／128／192／255、pan 192／181 和不同 refresh profile，共 288 个场景、两份既有 core、1,152 次执行。身份、命令、源摘要和原始 descriptor 证明见 `native-capture.json`。
- 两份 core 的 6,432 个步骤和 2,832 条原始 Main 访问一致；observer 开关前后完整 native 状态与 RAM 一致。包含 288 次 marker 读取、144 次 flags 读取、48 组 accepted publication／384 次原始 metadata 写入。
- Go E2E 使用生产 resourcebridge 的四份压缩资源，验证实际不可变 descriptor 身份、独立 PCM 反馈、全部 bus／反馈访问、首末 metadata 时刻、完整 pan、最终 canonical 字段及结束 clock。原始八次写入及其源数据顺序保留在 oracle 中，运行时只在持总线且屏蔽中断的区间发布整体 metadata。
- 同时验证 continuation 与完整 commit 的一致性、共享预算、open hold／当前帧要求、scheduled boundary 与 metadata overlap 拒绝、错误／重复 intent 和 detached 分支隔离。`final/` 的原生 race 与实际 Node/Wasm 各 51 个顶层测试、2,916 条通过记录，无跳过；scoreclock、PCM 与 mixed 的 vet 均通过。旧生产文件、测试、fixture 与 golden 未修改。

复跑使用仓库内新目录：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_music_clock.py --mode native --out .local/md-yuyuhakusho/music-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_music_clock.py --mode wasm --out .local/md-yuyuhakusho/music-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_music_clock.py --mode vet --out .local/md-yuyuhakusho/music-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_music_clock.py --audit
```

脚本将 Go telemetry、缓存和临时文件限定在本 worktree，并绑定源码、正式资源、原版夹具和输出日志摘要。两个 bus package 的额外 vet 命令及环境另存于 `final/bus-packages-vet-command.json`。

原版 profile 声明初始 held bus、busy 9、无额外 IRQ；不证明私有入口的自然可达性，不代表恢复总线后的整场 PCM、世界时间线或浏览器播放已完成。阶段和调用约束见 [MUSIC.md](../../audio/scoreclock/MUSIC.md)。
