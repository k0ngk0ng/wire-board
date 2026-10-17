# 无持球人发射与挂框球恢复

球场 3 的两次完整比赛失败来自不同路径：门装置重新发射动作 4，以及篮筐接触产生挂框动作 9。已恢复独立球处理及共享目标选择；世界层已接入这两条路径。

## 原版行为

| 路径 | 原入口 | 恢复行为 |
| --- | --- | --- |
| 无持球人发射 | bank 6 `$B65F`、固定 `$DFF8/$E078` | owner 6 使用球自身高度选择普通目标，不读取不存在的玩家；仍按原朝向、篮筐状态选择 X、弧高及接球点 |
| owner 6 特殊标记 | `$E078` 读取 `$055A` | 非零时清除该标记，并将目标高度改为 272；不清玩家特殊标记 |
| 挂框初入 | bank 6 `$B7EE` | 绑定 owner 6；停止原运动；X 速度设为 1；计数设为 96；纵深整数设为 42；高度为当前选中篮筐整数高度加 14；保留位置小数 |
| 挂框持续 | `$B846` 与 `$B7EE` 尾部 | 按球场左右半区选择横向抖动边界，每次加减 1/8 像素速度；计数减至零先切动作 10，再执行球的场地运动 |

门装置的原动作表对篮球写 **4**，对人物写十进制 **62**（`$3E`），不是篮球动作 9 或人物 `$62`。挂框 9 来自 `ResolveRimContact` 的原随机挂框分支，不能用门装置逻辑替代。

REA 证据：`ev_e334d6e67ec5f29ef0edf180d96d811d9eb87ed5f6ae515c56546d71b603e97b`，完整记录在 `.local/basketball/rea/ball-device/analysis/`。

## 接口

`OriginalShotBallContext.UnownedSpecialShot` 对应 owner 6 的 `$055A`。`OriginalShotBallStep.ClearUnownedSpecialShot` 表示消费该槽；调用方不能据此索引 `Actors[6]`。原 `ChangeSpecialShot` 仍只用于玩家。`originalShotLaunchPlan` 返回的消费标志按 `Holder` 选择这两类清除命令，Ball 12 可复用同一目标函数。

`StepStuckBall(ball, selectedHoopHeight, court)` 返回 `StuckBallStep{Ball, ReleaseHolder}`。高度必须来自 `SelectedHoop` 对应的当前篮筐对象；`ReleaseHolder` 表示写 owner 6。动作完成时返回的球已执行当次场地运动，不能再重复积分。

## 验证

`ball-device.json.gz` 保留 **1,054 次**原完整函数指令执行：88 次 owner 6 发射、6 次挂框初入、960 次挂框更新。覆盖目标高度边界、左右朝向、篮筐标志、独立特殊标记、左右抖动边界、正负速度、动画游标及倒计时 0／1／2／96。使用已记录的原场地 RAM 初始化隔离执行，执行原 MMC3 换页与场地运动；不是自然比赛录像或完整对局证明。

实现前失败场景见 `VERIFICATION.md`。先失败及通过日志为 `.local/basketball/engine/ball-device-{before,after,race,wasm}.log`。回归另保留已有普通／特殊球真实调用与普通投篮飞行对照。

尚未扩大到 owner 4／5、任意负高度或未恢复特殊球动作。挂框接触的产生及场地运动副作用仍归既有对应模块；本次不修改 RNG、AI、比赛时钟或设备触发条件。

复跑：

```bash
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_ball_device.py
go test -race ./games/basketball/engine -run '^TestOriginalDeviceShotAndStuckBallInstructions$'
basketball_goroot=$(go env GOROOT)
GOOS=js GOARCH=wasm go test -exec "$basketball_goroot/lib/wasm/go_js_wasm_exec" ./games/basketball/engine -run '^TestOriginalDeviceShotAndStuckBallInstructions$'
```

## 世界层与实际失败回归

`WorldActionState.UnownedSpecialShot` 独立保存 `$055A`，零值省略以保留旧快照形状。`extract_world_actions.py` 从原 RAM 读取该槽，不从测试期望补值；已有 1,998 次世界动作的 3,996 个前后记录均为 0。原冷初始化 `$C9CB` 设置 A=0、X=2、Y=7 后调用 `$F8BD` 清除 `$0200..$07FF`。探测以 165 填充待清内存后实际执行该例程，确认 `$055A` 清零；证据保存在 `ball-device-source.json` 的 `coldClear`。

`ball-device-runtime.json.gz` 原样保存两份 Go 完整比赛失败前态（视频帧 2,081／逻辑步 691，视频帧 2,339／逻辑步 777）。回归从各前态执行正常 `StepVideo`，分别验证无持球人投篮与挂框初入成功；这是 Go 集成回归，不计为原 ROM 自然执行。固定规则测试另验证非零 `$055A` 只清自己的槽，以及挂框读取当前选中篮筐高度并保留篮球小数。原 1,998 动作阶段和 1,998 连续世界更新保持通过。

世界层针对日志为 `.local/basketball/engine/ball-device-world-{before,after,race,wasm}.log`。球场 3 四配置完整比赛由主任务统一继续验收，本组件日志不宣称已通过完整比赛。
