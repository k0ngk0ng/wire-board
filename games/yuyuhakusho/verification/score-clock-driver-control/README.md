# 控制、渐弱与暂停整拍验收

本项把 01／05／04、活动 fade 和 pause start／hold／resume 接入同一个 driver 执行器，所有阶段与后续请求、音轨初始化和 18 个声部共用时钟与预算。普通 PCM 专用接口继续保持原来的闭包和拒绝契约。

## 原版与先行失败

103 组完整原版调用包含 42 组控制指令、41 组 fade 和 20 组 pause，覆盖空 score、活动音乐／primary、byte 与音量边界、不同抑制 flags、fade 后新请求、终止清音、pause 正／负值及 pending payload。两份固定 core 分别关闭／开启观察器，共 412 次执行；完整状态、RAM、PSG、字段访问和跨 core 结果一致。

保留 81,767 个源步骤、18,426 次 Main IO、188 次 PSG 写入和 224 次 Root 字段访问。原版数据与端口时刻在实现后未修改。

profile 使用与既有 running 相同的固定 auxiliary held0 边界。实际 FM total level 在 master 0 写入，retained TL cache 在原地址按物理／逻辑 operator 映射设置；hold／resume 使用真实独立 pending 值。整个 profile 不插入额外 IRQ，不声称证明 world 请求生产时刻或逐 long 中断可见性。

失败计划、103 组 oracle、producer、输入构造和 Go E2E 在生产前冻结，真实缺少 API 的 red 保留。第一轮 127 条记录通过、3 条失败：一组 fade 后 primary 的跨 FM／PSG 调度、父测试，以及暂停存档继续执行测试。

暂停存档 E2E 起初把 `score.State.Ticks` 错当成音乐推进次数。既有未修改的 `FinishSuspended` 明确按实际 driver 调用计数，旧 Root 原版回归也逐次断言此计数。只订正了新 E2E 的调用次数预期；完整音乐状态、IO、payload、队列及正常继续断言保留。原测试、精确 diff 与依据见 `test-call-count-correction.json`，未修改原版数据。

订正后，第二轮仍为 127 条通过、3 条失败，暂停 start／hold／resume 已通过，继续执行触发相同跨芯片问题。诊断显示 Main IO 在失败前与原版逐项一致：YM data 为 104,903，PSG 为 118,105，随后 Main 在 123,453 取得总线时，PCM 才提交 105,495 的 YM address。统一 cursor 错误拒绝合法顺序；提前推进 PCM 会丢失原版此处的 287 master cycles 争用成本。

该问题由[独立芯片时间域](../fmpsg-independent-lanes/)的原版完整 YM／PSG／FIR／PCM 对照先行验证后修复，保留原调用顺序与各芯片域的单调约束。第三轮控制、初始化与普通请求的 609 条针对性记录全部通过，包含 103 个控制 profile 和真实 mixed JSON 恢复后的暂停／继续；没有修改端口时刻或删除失败场景。

## 复跑与边界

最终原生 race 与实际 Node/Wasm 各通过 127 个顶层／8,403 条 clock／Root 记录，其中 scoreclock 为 98 个顶层／8,301 条，Root 为 29 个顶层／102 条，用时分别为 512.127 秒和 326.486 秒。全部芯片／mixed 消费者两端各通过 25 个顶层／184 条记录，用时分别为 159.605 秒和 57.953 秒，无跳过，五包 vet 通过。未运行其他游戏矩阵。

`validation.json` 绑定最终检查、当前代码与无损压缩日志。当前 API、guarded owner effects 和 bulk clear 边界见 [DRIVER_CONTROL.md](../../audio/scoreclock/DRIVER_CONTROL.md)。返回 intents／fields 不得再次播放，所有 detached owner 只在成功后联合提交。

使用仓库内新的输出目录，原生竞态和实际 Node/Wasm 分别执行完整 clock／Root、全部芯片与 mixed 消费者；不运行其他游戏矩阵。所有 Go 缓存、telemetry 与临时目录限制在 worktree 内。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_driver_control_clock.py --mode native-clock --out .local/md-yuyuhakusho/driver-control-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_driver_control_clock.py --mode wasm-clock --out .local/md-yuyuhakusho/driver-control-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_driver_control_clock.py --mode native-chips --out .local/md-yuyuhakusho/driver-control-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_driver_control_clock.py --mode wasm-chips --out .local/md-yuyuhakusho/driver-control-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_driver_control_clock.py --mode vet --out .local/md-yuyuhakusho/driver-control-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_driver_control_clock.py --audit
```

[完整复刻待办](../../../../docs/research/md-yuyuhakusho/TODO.md)中的 world IRQ、请求生产时刻、联合 checkpoint 和平台整场播放仍未完成；试玩构建继续保持完整音频关闭。
