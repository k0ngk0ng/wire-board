# 完整稳定运行拍验收

本项将实际 Root、header 与 18 个声部连成一次完整稳定运行拍，从原版 gate 入口执行至实际 return。当前边界为初始 queue 为空、selected 为 FF、fade／pause／control 为 idle 且无额外 IRQ；待消费新请求、dispatch／header、非 idle fade／pause、world 调度和平台播放仍待接通。

307 个 profile 包含实际 115 个 owner 的首轮路径、primary 正常发声、全部有限终止／恢复代表、多声部初始化后的运行状态、稀疏／完整 tempo reload 和 DAC held-note 的 2／零值回绕／255。两份固定 core 分别以 observer 关闭／开启执行，共 1,228 次；完整原生状态、RAM、PSG context 与跨 core 结果一致。保留 210,664 个源步骤、53,802 次 FM／bus 事件和 229 次 PSG 写入。

整拍使用已知 auxiliary empty factory，在 master 0 持有总线，保留正式 pending PCM descriptor、RateRequest 17 和 flags 5。每个 owned active voice 从对应逻辑位置获取原版研究 context；有限 inactive row 可以保留越过末尾的旧 cursor，该 parser 不会被调用，仅保留它的 owner bank／参数。首轮采样发现此边界，在测试冻结和生产改动前修正，未修改运行时 score 规则。

失败计划、Go E2E、原版 oracle 与采样脚本先于实现冻结；真实缺少 API red 保存在本目录。首轮有一条通过、308 条失败记录。由 empty 整拍原版步骤定位到实现少算两处：queue long test 实际为 16 CPU，外层立即数 ADDA stride 实际为 12 CPU。只修正生产常量，测试和 oracle 未修改；第二轮新增 309 条记录全部通过。

Go E2E 通过实际 Root 与 mixed owner 执行完整调用，逐项比较所有 Main IO 的值、顺序、时刻和整拍结束 clock。Tempo、Root priority 和 queue 与原版相同；完整 Go State／Output 与原接口使用相同原版 feedback 序列时相同。实际 primary 的 voice-local bank 分支由整拍覆盖，不以旧隔离 route 标签替代。预算 300 在外部访问后耗尽，证明 header 与所有声部未重新预算；外部错误不提交 score。未计时的入口在外部 IO 前拒绝。

最终原生 race 与实际 Node/Wasm 各通过 115 个顶层测试、7,794 条记录，无跳过；其中 scoreclock 为 86 个顶层／7,692 条，rootcontrol 为 29 个顶层／102 条，用时分别为 409.217 秒和 252.744 秒，三个相关包 vet 通过。未重复其它游戏或完整长音轨矩阵。

完整回归、当前摘要和无损压缩日志见 `validation.json`。旧文件与摘要保留为历史证据，当前源码以本清单及实际回归为准。

复跑使用仓库内新的输出目录：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_running_tick_clock.py --mode native --out .local/md-yuyuhakusho/running-tick-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_running_tick_clock.py --mode wasm --out .local/md-yuyuhakusho/running-tick-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_running_tick_clock.py --mode vet --out .local/md-yuyuhakusho/running-tick-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_running_tick_clock.py --audit
```

API 与联合提交边界见 [RUNNING_TICK.md](../../audio/scoreclock/RUNNING_TICK.md)。[待办清单](../../../../docs/research/md-yuyuhakusho/TODO.md)中的完整 Root／world 项目仍未完成；试玩构建尚未开启完整音频。
