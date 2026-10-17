# 特殊发射的斜线上下文

实现前排除：把原 `$0069` 当常量 0 或立即数 105；读取投射物自身小数代替人物 1；读取回合开始快照而非发射时状态；遗漏 `$3E` 人物及球 12／15 的准备 API；在不走该斜线分支时强制索要无关上下文；缺少上下文仍静默发射。

复用已经恢复的 `PassLaunchContext`。本次原指令矩阵分别在 `$9E99`、`$B690`、`$BB2C` 停止，只比较原发射准备，不把后续原位置回填成 Go 状态。

## 已完成

两项准备 API 使用可选的 `PassLaunchContext` 参数，沿既有 `LaunchPass` 传递，不设置默认零值。World 分别在 `world_launched.go` 和 `world_team_special.go` 的实际准备调用处读取 `Actors[1].Player.Body.Position.X`，不会使用回合开始时的缓存。

20 次原准备调用覆盖人物席位 0／1、球 12／15，以及人物 1 的 X 小数 0／1／64／105／255。直接 API 与 World 的物理加载前状态均逐项对照；缺少上下文的等斜线调用继续明确失败。World 测试先复现原缺上下文错误，再接两行实际状态读取。

定向 race／Wasm 通过，包含原准备矩阵、World 接线、已有抛起／特殊球和 1,998／57 次连续轨迹；race 包级耗时 7.173 秒。未重复完整比赛。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/extract_special_launch_context.py
source scripts/env.sh
go test -race ./games/basketball/engine -run 'Test(OriginalSpecialLaunchContext|WorldSpecialLaunchReadsCurrentSecondPlayerX|OriginalLaunchedPlayer|WorldLaunched|OriginalTeamSpecialBall|WorldTeamSpecial|WorldLoop)'
```

证据位于 `.local/basketball/rea/special-launch-context/`，包括 `world-before.log`、`world-after.log`、`race.json`、`wasm.log`；夹具和原指令来源摘要位于 `testdata/special-launch-context*`。

## 设备改写特殊球的限制

`$BDD2` 在动作 14 被设备改成 4 后会使用 `$00FF` 指针。进一步确认 `$00FF` 是原音频驱动的共享工作字节，`$0100..$0102` 是图形上传缓冲前缀；这些字节均不能默认当零。完整受控原指令审计和原 music61／shot30 的工作字节样本证明，在合法 Court2 设备坐标上它们确实会改变下一动作，因此保留现有未知上下文错误。

最小尚缺语义源是比较发生时的音频工作字节、最近图形上传命令前缀及其完成状态；未把整个 APU 或任意 RAM 数组加入 World。此边界不影响已交付的发射上下文接线。

复跑源审计：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/audit_special_turn_alias.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_special_turn_alias_examples.py
```
