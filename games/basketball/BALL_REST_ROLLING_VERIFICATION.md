# 基础球静止与滚动动作

## 实现前失败场景

- 冷配置第 322 次更新从滚动请求进入球动作 0，真实动作分派尚未恢复；不能用忽略错误或替换种子绕过。
- 动作 0 的入口必须由 bank6 `$B4FC` 分派表确定，不能把 `$B536` 的指令中间当入口；真正入口为 `$B52E`。
- 静止动作是否设置 Entered、动画是否推进、速度是否每次清零应遵循原分支，不套用其他球动作的初始化模板。
- 滚动先推进动画和物理，再根据 X 速度判停或执行原摩擦；有符号速度、跨零、X 为零而 Depth 非零均需独立验证。
- 原摩擦是共享参数表算法，不自定义阻尼。滚动首次初始化的排除接球者写命令由世界层提交。
- 完整自然原动作输入／输出必须包含所有权、排除者、请求与动画数据，捕获只读并核对完整 RAM 和截图。
- 新动作沿用真实物理 SurfaceWrites，出错时世界原子回滚；不改变特殊球、人物出手和 owned-ball 输入分派。

- 静止球在抬高平台触地时，bank4 `$8249` 会改为反弹且深度速度为 -0x80；地形当前保留该原分支，不把静止球悬留平台。
- 画面震动 `$06D5` 非零时，动作 0 保留旧 pose；这是必需上下文，不能无条件清零。

## 平台落地分支的后续核对（修正前）

原 bank4 `$8249` 只在积分后整数高度恰等于地形高度的分支检查静止球释放。若本次从上方向下穿过平台，由另一落地分支修正高度，应保留动作 0，到下一次等高物理更新才释放。先补此区分测试，再收紧释放条件。

## REA 证据与自然运行

有效证据位于 `.local/basketball/rea/ball-rest-rolling/bank6-corrected/`：

- `$B52E` 静止：`ev_14b338a12db4481cf74b9ad8f54274d88e8ce9ebaaae989b20e48b204f4bf7fa`。
- `$B6A1` 滚动：`ev_11dd131099c315015d1f4a75ce34c410db7b8f377dea88dc3ce74619b868c307`。
- `$EA5C` 原摩擦：`ev_a2b51236a13e0ea0241e7b92014a120f86e62dd2c255b3564a344e6f4c1621f7`。

初次探查使用了错误的 bank5 和 `$B536` 入口；`analysis/`、`bank6/` 的这两份探索输出不作为静止动作证据。正确 bank6 分派表 `$B4FC` 的第 0 项指向 `$B52E`，已据此重新运行 REA 并保留完整函数结果。

捕获复用自然 `special-ball-cpu-command` 的 18,000 帧控制时间线，不改 RAM。全部 RAM 和最终截图与既有参考一致，保留 `$E149..$E14C` 之间每个以动作 0／5 进入的完整调度，共 221 条（192 静止、29 滚动）；未剔除请求改变动作的记录。前后完整 WorldActionState 与字段、固定配置、录像摘要保存在 `testdata/ball-rest-rolling.json.gz` 及来源 JSON。

## 实现与验收结果

静止不改 Entered 和动画，画面震动时保留旧 pose；只清水平速度，仍运行原物理。滚动首次进入清排除接球者，物理后先判 X 是否为零，再执行参数 7 的摩擦：X 跨零截到零，Depth 跨零不截断。`CameraShake` 是动作上下文的动态值，由循环当前 Camera.Shake 传入，没有放入永久初始配置。平台静止释放仅在原等高分支触发，落地修正分支到下一次更新才释放。

221 条原调用逐字段相同。新增帧序、shake、过零和平台边界检查通过；原普通球、球请求、完整动作和连续世界轨迹以及 MotionSurface 事件回归通过。原生竞态、Wasm 日志分别为 `native-tests.json`、`wasm-tests.log`，平台分支补充修正前后为 `landing-before.log`、`landing-after.log`、`landing-race.json`、`landing-wasm.log`。

固定冷配置推进到第 383 次更新，下一边界是 `special/equipped shot launch not reconstructed`，已交特殊出手恢复工作。完整前态为 `cold-next.json`；仍保留 6,000 目标，不把 383 步称为完整游戏或完整比赛时间。

```sh
PYTHONDONTWRITEBYTECODE=1 .local/monopoly/venv/bin/python scripts/basketball/capture_rest_rolling.py
source scripts/env.sh
go test -race ./games/basketball/engine -run 'TestRest|TestRolling|TestBall|TestShot|TestWorldActions|TestWorldLoop|TestMotionSurface' -count=1
GOOS=js GOARCH=wasm go test -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/basketball/engine -run 'TestRest|TestRollingBall' -count=1
```
