# 音高写入与自动声像验收

本项覆盖已经算出的 FM／PSG 音高写入，以及 note／held 两种入口的自动声像调用。完整 voice caller、Root／world 和平台整场音频仍待完成；试玩服务没有启用完整音频。

测试、[失败场景](../../../../docs/research/md-yuyuhakusho/SCORE_CLOCK_FREQUENCY_PAN_FAILURES.md) 和原版 oracle 均先于实现冻结。`test-first.json` 记录各文件摘要，`missing-api-red.jsonl` 保存真实缺少 API 的失败。第一轮实现因普通 FM bus 被过度约束为 `PanBus` 而编译失败，记录在 `first-implementation-native.jsonl`；只修正生产 API，使普通 FM 使用 `Bus`，channel 6 单独要求真实 `PanBus`。第二轮 131 条记录通过，测试、oracle 和旧 golden 未修改。

原版证据使用现有两份固定 core，分别在 observer 关闭／开启时执行 128 个 profile，共 512 次。完整原生状态、RAM、PSG context 和两份 core 的结果一致。40 个音高 profile 覆盖十种 channel、四种 word；88 个声像 profile 覆盖五种 mode、note／held、note-off／claimed、reset、period／elapsed、step limit 和 once stop。保留 2,996 个原版步骤、720 次 FM／bus 访问和 32 次 PSG 写入。初始 held bus、mailbox 0、无额外 IRQ 是这些固定 profile 的边界条件。

Go E2E 对全部 profile 比较真实 mixed chip／bus 的访问值、顺序、时刻及结束 clock。原版音高高字节保留原始值；只有比较 FM 高频率寄存器时，屏蔽硬件忽略的两位。实际 score transaction 另验证自动声像的一次真实写入、持久 Pan 不变，以及输出和最终 State 与原 driver 一致。预算失败、错误后继续、提前结束和重复结束均有验证。

最终完整 scoreclock 回归在原生 race 与实际 Node/Wasm 中各通过 75 个顶层测试、5,439 条测试记录，无跳过。原生用时 169.340 秒，Wasm 用时 89.625 秒；score 和 scoreclock 的 vet 通过。没有重复其它游戏或完整长音轨矩阵。`pan.before-shared-service.txt` 保留重构前实现；现有路由体仅提取为私有入口，父子调用共享同一个 service，旧测试继续覆盖该服务。

`validation.json` 绑定源码、输入资源、冻结证据及压缩后的完整测试日志。`native.jsonl.gz`／`wasm.jsonl.gz` 无损还原后的 SHA-256 与对应 command 记录一致；`*-command.json` 保存精确命令和工作区内的缓存路径。旧验收清单中绑定旧版 `pan.go` 的摘要保留为历史证据，当前改动以本清单与本次完整回归为准。

复跑时，使用仓库内尚未存在的输出目录：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_frequency_pan_clock.py --mode native --out .local/md-yuyuhakusho/frequency-pan-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_frequency_pan_clock.py --mode wasm --out .local/md-yuyuhakusho/frequency-pan-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_frequency_pan_clock.py --mode vet --out .local/md-yuyuhakusho/frequency-pan-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_frequency_pan_clock.py --audit
```

原版重新采样需要现有的本地 ROM／core 和研究环境，入口为 `scripts/yuyuhakusho/score-clock-frequency-pan-capture.py --out .local/md-yuyuhakusho/frequency-pan-native-repeat`。日常 Go 回归直接读取冻结 oracle，无须运行模拟器。本次只清理已经结束的私有采样目录内的一份重复 oracle、两份 observer dylib 和空目录，共 52,427 字节；正式 oracle、固定 ROM／core 和试玩服务均保留。

API 边界见 [FREQUENCY_PAN.md](../../audio/scoreclock/FREQUENCY_PAN.md)。完整联合音频提交、浏览器播放器和发布验收仍需后续实现。
