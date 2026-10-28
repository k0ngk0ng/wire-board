# 暂停期间的实时呈现时钟

`Module.OnPausedTimer` 为显式启用的实时模块提供暂停期间的权威呈现推进，例如释放仍在播放的暂停提示音、推进不影响规则的图形状态。它不运行普通 `OnTimer`，也不恢复物理、电脑决策或玩家超时。

模块必须同时声明 `Realtime: true` 和原有 `OnTimer`，才能注册 `OnPausedTimer`。未提供该回调的游戏保持原行为：暂停时清空全部期限，不执行系统帧。普通非实时模块不能注册暂停回调。

暂停中的模块可以保留正数 `Match.SystemDeadline`，但必须通过所属模块的 `Module.ValidateMatch` 校验。未绑定模块能力的 `Match.Validate` 继续拒绝暂停系统期限；服务端安装和恢复快照均使用模块校验，不根据客户端字段开放能力。

| 暂停回调字段 | 允许行为 |
| --- | --- |
| `Data` | 更新呈现状态；游戏负责验证不改变物理、资源或规则结果 |
| `SystemDeadline` | 推进到 `Context.Now` 之后，或清零停止该时钟 |
| `CheckpointKey` | 在重要呈现边界变化，以便先持久化再发布 |
| 其他 `Match` 字段 | 保持原样，包括 `Paused`、`Actors`、`Deadline`、`TimeoutActors`、`Attention`、`Eliminated`、`Schema`、`Result`、计分资格及日志 |

回调收到 `Actor == -1` 和房间逻辑时间。初始加载屏障、关闭／结束状态、尚未到期或已清零的期限仍然阻止执行。宿主在调用前保留控制信封的不可变副本，因此原地修改 `Actors` 等切片也会被拒绝；返回错误、panic、非法信封和过期的新期限都不会提交半帧。

暂停状态下不调用 `BotAction`，普通 `OnTimer` 与超时托管仍停止。玩家可以通过已授权的普通 `Apply` 操作恢复游戏；只有恢复成功后，后续系统帧才回到普通回调。宿主不会根据暂停计时器自行解锁、移除席位或产生结算。

`Data` 对平台是不透明的。分离回调可防止平台错误调用整个规则步进，但不能识别任意游戏 JSON 中哪个数值属于生命或物理。游戏仍须用自己的验收证明暂停呈现不会改变这些状态，不能把存在这个回调作为游戏暂停逻辑已经完成的证据。

## 持久化和重放边界

暂停帧复用现有实时路径：普通呈现帧驻内存，周期检查点和正常关闭保存最新快照；`CheckpointKey` 改变的关键帧必须先成功提交 SQLite，再发布。失败时原有快照、检查点及日志保持不变。若音效释放等边界需要在进程崩溃后继续保持，应在该边界改变 `CheckpointKey`。

关键暂停帧的 `realtime_frame` 日志增加 `pausedTimer: true` 和 `timerAt`。后者是实际传给回调的房间逻辑时间，单位毫秒；离线重放应选择暂停回调，不能按普通 `OnTimer` 重放。现有服务启动恢复最近已保存快照，不会逐项重放所有系统日志，也不保证未保存普通帧零回退。

暂停呈现只更新版本和更新时间，不更新 `LastActive`。30 天无操作保留期限继续生效，包括服务启动时的过期清理；呈现时钟不能使无人操作的房间永久存活。

## 验证

失败场景先记录于 `paused_timer_test.go` 和 `module_paused_timer_e2e_test.go`。后者使用真实 HTTP／WebSocket、平台调度器及 SQLite，覆盖呈现与物理分离、控制信封篡改、panic、加载屏障、期限边界、重启、闲置过期、重复请求、关键事务回滚和日志离线重放。

```sh
source scripts/env.sh
mkdir -p .local/paused-timer/tmp
export TMPDIR="$PWD/.local/paused-timer/tmp"
export GOTMPDIR="$TMPDIR"
go test -race ./internal/gameapi ./internal/server \
  -run '^(TestPausedTimerCapabilityValidation|TestModulePausedTimer.*)$' \
  -count=1 -json > .local/paused-timer/recheck.jsonl
```

本轮 red／green、检查范围和 HTTP／WS 场景结果保留于 `.local/paused-timer/`。夹具只验证公共时钟契约，不代表魂斗罗音频、视频或暂停表现已经验收；没有为其他游戏启用回调。
