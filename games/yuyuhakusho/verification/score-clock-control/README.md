# 正常控制参数与 Root queue effect 验收

本项为正常控制 handler／continuation、PSG noise 与真正的 Root queue 写入补齐计时。parser caller／backedge、track-end／其它 live body、完整 Root／world 调度和平台整场声音仍待完成。

- [先行失败场景](../../../../docs/research/md-yuyuhakusho/SCORE_CLOCK_CONTROL_FAILURES.md)、最终原版 oracle 和 Go E2E 均先于实现冻结，`missing-api-red.jsonl` 保留真实缺少 API 的失败。旧 State、intents、fixture 与 golden 不变。
- `native-control-full.json.gz` 覆盖正式 25 种控制形状与一个声明的 PSG pan-ignore 兼容形状，分别验证 claimed／unclaimed 和四种 refresh profile，共 208 个场景、两份 core、832 次执行。两 core 的 820 步、PSG 与 queue effect 一致，observer 开关前后的完整状态、RAM 与 PSG context 一致。含 4 次实际 noise port 写入与 8 次实际 queue mutation；身份、命令及摘要见 `native-capture.json`。
- 初步采集在 Go 测试与实现前修正了输入投影：明确保留 noise 参数与 queue/noise continuation，并将 pan operand 的左右位组合写为明确括号。初步 oracle 和原 producer 保留在 `preliminary-before-input-projection/`，正式测试使用后续冻结版本；原 ROM／core 未修改。
- 首次实现编译发现新 `ControlWork` 名称与旧 Root idle-control 类型冲突。新 API 机械更名为 `ControlParameterWork`，旧类型保持不变；旧冻结测试全文和命名修正摘要保存在 `control-test.before-api-name-correction.go.txt`、`api-name-correction.json`。验证器要求当前测试精确等于原文的单一标识符替换，任何断言、条件、参数、原版数据和期望值均未因此改变。更名后新增 241 条记录全部通过。
- 实际 E2E 验证正式 25 种控制事件的 working projection、noise 真正 PSG owner、Root 唯一 queue owner 的准确写入时刻、同步且仅一次 effect、遗漏／重复／过期／外部失败和 shared budget。Root 的默认旧消费者顺序也由全部原有 Root 检查覆盖。
- 原生 race 与实际 Node/Wasm 均通过：scoreclock 各 63 个顶层测试／3,401 条记录；Root 各 29 个顶层测试／102 条记录；既有 score observed／transaction 与四条原版轨迹共 30 条记录、546 帧原版兼容回归。每个运行时合计 3,533 条通过记录，无跳过；三个改动包 vet 通过。没有重复执行其它游戏或完整长轨迹矩阵。

复跑使用仓库内新目录：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_control_clock.py --mode native --out .local/md-yuyuhakusho/control-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_control_clock.py --mode wasm --out .local/md-yuyuhakusho/control-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_control_clock.py --mode vet --out .local/md-yuyuhakusho/control-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_control_clock.py --audit
```

脚本限制 Go telemetry、缓存与临时文件在本 worktree，并绑定当前源码、正式资源、Root／原版夹具和日志摘要。`*.before-control.txt` 保存原生产文件；旧验收摘要作为历史证据保留，不改写旧 hash。原版 profile 为声明 handler／continuation、无额外 IRQ，不证明整个 tick 已计费。调用边界见 [CONTROL.md](../../audio/scoreclock/CONTROL.md)。
