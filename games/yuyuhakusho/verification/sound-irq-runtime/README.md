# 外层声音中断与联合状态验收

本项从原版声音中断／补跑入口执行到实际 return，串联 canonical scheduler、delayed enqueue、真实 driver wrapper、Root 和全部声部，使用同一个 clock、refresh 和整体预算。联合 `session.State` 各保存一次 Scheduler／Root／score／PCM／FM／PSG，World queue、pause 和主时钟不进入音频存档。

122 组原版 profile 覆盖既有 103 组完整控制路径，以及 delay 1／2／65535、delayed 普通／特殊请求、满队列与重复请求、busy 重入、backlog byte 回绕、0／1／2／3／255 次补跑，以及带实际音乐、fade 和 pause 的连续补跑。两份固定 core 分别关闭／开启观察器，共 488 次执行；完整状态、RAM、PSG、字段和跨 core 结果一致。

保留 127,491 个源步骤、378 个实际 driver 入口、21,488 次 Main IO、219 次 PSG 写入、1,649 次 scheduler 字段访问和 33 次 delayed request 字段访问。

E2E 对照全部端口值／顺序／时刻、scheduler 与 request 的实际字段访问、每次 wrapper 入口和最终 clock。每个 profile 另从联合 JSON 状态恢复，通过真实 mixed owner 得到相同音频状态与 World 字段；资源／owner 不匹配和跨 child driver 的预算失败不提交任何 owner。engine observed 规则另复用全部原 scheduler 边界，验证负 slot 的 TST／CMP live read、effect lease 和 driver 后 backlog 递减。

失败计划、原版数据、producer、输入构造和测试在生产前冻结，真实缺少 package／API 的 red 保留。首轮 127 条记录通过、4 条失败：普通双路发布遇到单路 capture guard，另有两处新 E2E 准备错误及父测试失败。准备错误是 combined adapter 将音乐 publication 误按普通格式记录，以及失败样本查找遗漏 native producer 的数字前缀。只订正这两处新测试适配，原稿、diff 与依据见 `test-adapter-correction.json`；原版 input、oracle 和端口时刻保持不变。

真实生产修正是允许实际 held-bus 的普通 slot A／B 请求更新独立字段，其 single descriptor 来源不变。真正改写 single 来源和同一 dual slot 的未验证半捕获覆盖仍拒绝。旧 FramePlan 的保守拒绝契约保留：首轮完整协议回归揭示该兼容要求，原测试未修改。第二轮新增 131 条记录通过，兼容修正后的 126 条相关记录也通过。

当前完整验证、历史失败和原样来源由 `validation.json` 绑定；旧清单保留历史摘要。API 与提交边界见 [SOUND_IRQ.md](../../audio/scoreclock/SOUND_IRQ.md)和[联合状态](../../audio/session/README.md)。本项只证明给定实际入口 clock 的声音调用，不决定世界 CPU／VDP IRQ 接受或请求生产时刻，不是平台整场音频完成证明。

最终原生 race 与开启 GC 检查的实际 Node/Wasm，各通过 129 个顶层／8,527 条 clock／Root 记录，用时分别为 545.735 秒和 358.132 秒；PCM／芯片两端各通过 40 个顶层／289 条记录，用时分别为 236.969 秒和 86.229 秒。engine scheduler 两端各通过 7 条记录，相关 vet 通过，均无跳过。一次 Go/Wasm 运行时坏指针错误未在针对性、20 次压力复跑及完整 GC 检查中重现；原因未证实，原始失败与[复核记录](WASM_GC_INVESTIGATION.md)保留，不能据此替代最终浏览器稳定性验收。

复跑使用仓库内新的输出目录；脚本限制全部 Go 缓存／telemetry／临时目录，Wasm 使用实际 Node 执行器。engine 仅选择 scheduler，不运行其他玩法矩阵。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_sound_irq_runtime.py --mode native-clock --out .local/md-yuyuhakusho/sound-irq-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_sound_irq_runtime.py --mode wasm-clock --out .local/md-yuyuhakusho/sound-irq-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_sound_irq_runtime.py --mode native-audio --out .local/md-yuyuhakusho/sound-irq-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_sound_irq_runtime.py --mode wasm-audio --out .local/md-yuyuhakusho/sound-irq-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_sound_irq_runtime.py --mode native-engine --out .local/md-yuyuhakusho/sound-irq-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_sound_irq_runtime.py --mode wasm-engine --out .local/md-yuyuhakusho/sound-irq-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_sound_irq_runtime.py --mode vet --out .local/md-yuyuhakusho/sound-irq-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_sound_irq_runtime.py --audit
```

[完整待办](../../../../docs/research/md-yuyuhakusho/TODO.md)继续保留世界时序、World／audio 联合 checkpoint、平台连续播放、无损资源发布与浏览器完整对局验收；试玩完整音频仍关闭。
