# 箱子捕获与脱离人物动作

实现前列出的失败场景：将捕获直接当成受伤或解除；把 `$056C=4` 当对象号而非逃脱计数；每轮待机重置计数；电脑和真人使用相同自动跳脱；持球时直接改球动作 11 而非排队请求；球的推出方向误用新朝向而非旧姿态；在物理之后才释放球权或绑定特殊持有人；跳脱覆盖旧竖直冲量；跑跳重新计算横速；落地计数下溢被夹成 0；提前清除特殊持有人；未经过道具动作 7 就直接去掉箱子。

装置接线另列失败场景：人物完成动作时复制物理绑定，导致装置清除所选人物 `JumpPhase` 的写入无法传回调度器；把平台下方的受控前态误当正常起跳落地；在下一次动作才观察到重置，从而漏掉首步差异。

## 恢复范围与来源

人物 `$50..$56` 覆盖捕获待机、行走、跑动、刹车、跳脱、跑跳及落地恢复。电脑由 `$50` 的下一动画动作进入 `$54`；真人待机的下一动作为 `$50`。每次 `$54/$55` 落地递减 `CaptureCounter`，从 4 经五次落地变为 255；负数时人物转普通落地 6，固定道具 4 转动作 7。道具动作 7 的入口负责清除 `SpecialHolder`。JSON 中的 `captureObject` 仅保留为既有回放字段名，不再建立另一份状态。

| 原程序位置 | 恢复行为 |
| --- | --- |
| bank6 `$94F8` | 动作 `$50` 绑定特殊持有人；按旧显示姿态设置球横速 ±2 像素，释放球权并排队请求 11 |
| bank6 `$953F` | 动作 `$51` 行走和捕获姿态序列 |
| bank6 `$8831` | 动作 `$52` 共享跑动、`ShotCycle`、持球跑动的相邻动画字节；腾空转 `$0F` |
| bank6 `$9EC1` | 动作 `$53` 刹车，横速归零转 `$50` |
| bank6 `$9556`、`$EA28` | 动作 `$54/$55` 清附着；在旧竖直速度、重力上分别加 2272、288；落地递减同一计数 |
| bank6 `$9EE2` | 动作 `$56` 停止水平和竖直运动，落地姿态结束后转 `$50` |
| bank6 `$B1BD` | 道具动作 7 清除固定对象 4 对应的特殊持有人 |
| bank6 `$9749`、`$978D` | 普通坠落检查 `$053D..$0546` 的所有权；`$056B` 不在该扫描范围内 |

REA 静态来源保留在 `.local/basketball/rea/captured-player/analysis/`，批量反编译 Evidence ID 为 `ev_aaa1d310233d2f1144674761bc118a019f79f8190ae14cd85d7a1530a20c1e0f`。共享跑动来源另见 `.local/basketball/rea/possessed-idle/shared-run/`。

原指令夹具由 `scripts/basketball/extract_captured_player.py` 生成，保存在 `engine/testdata/captured-player.json.gz`，摘要及 ROM、初始 RAM、夹具的 SHA-256 保存在相邻的 `captured-player-source.json`。原指令执行仅截获 `$FBCC` 作为音效请求出口，不运行目标音频设备。

## World 接线

`engine/world_actions.go` 将捕获动作交给 `engine/world_captured.go`。绑定特殊持有人、球权释放、球请求和球横速先提交，再进入原场地物理；完成物理后提交姿态、附着和逃脱计数。球请求 11 的原接受条件只允许球动作 1 或 2，其余动作保留请求。

捕获跑动进入 `$0F` 后，普通坠落沿用真实所有权扫描。`SpecialHolder` 不再被误判为持有其他对象；仍检查篮筐、两个道具以及携带人物的所有权。

`CompleteCapturedPlayer` 临时切换原物理绑定的附着输出地址，结束时恢复该地址。它保留同一个绑定实例，使装置写入的 `resetJumpPhase` 能在外层调度器提交，而不等下一次动作才生效。

## 验证范围

- 560 次受控人物原指令调用，比较所有可恢复的 World 状态及音效请求。
- 131 组人物与箱子原指令调用，从一个初态连续完成电脑五次跳脱。每组按人物、箱子顺序调用；这不是 131 次完整 World 更新或 131 局比赛。
- 50 条球请求 11 接受或保留用例，覆盖 25 个球动作和 entered 状态。
- 4 条特殊持有人存在时的普通坠落原指令用例。
- 16 次装置原指令调用：平台下方旧前态 1 次，以及从 32 像素表面起跳到落地的连续 15 次。比较每步完整 World 状态及音效请求。
- World 测试先由真实 Kind10 接触搜索与响应进入动作 `$50`，不以强制写入捕获动作作为测试入口。覆盖被捕获者持球、其他人持球、无人持球，以及电脑完整五次跳脱与箱子动作 7。

旧装置测试使用 `(X=280, Depth=76, Height=0)`。原程序在首步就触发装置转 `$3E`，因此等待「捕获跳跃落地」是错误判据。该前态仍作为 `below-surface-original-regression` 保留，断言首步的原动作和跳跃阶段重置；另有 `surface-landing` 从实际 32 像素表面验证跳脱落地。这两项均从已完成真实捕获的 World 状态建立受控装置边界，不声称 Court2 会自然生成 Kind10 箱子。

这些验证不代表完整比赛或浏览器验收。真人 A+B 输入、触屏和控制器映射由相应输入接线验收承担；这里验证动作链和 World 提交顺序。特殊持球跑动在原姿态 7 时的音效请求 1 保留在逻辑输出中，本次没有修改平台音效资源；普通捕获到逃脱链不请求该声音。

## 可重复执行

在仓库根目录执行：

```sh
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/extract_captured_player.py
go test -race ./games/basketball/engine -run '^(TestWorldCaptured|TestWorldKind10CaptureToComputerEscape|TestOriginalCaptured|TestCapturedDropRequest|TestKind10|TestBallDeflectionAndDispatchAgainstOriginal|TestBallRequestRetainsIllegalAndProcessedRequests|TestFalling|TestWorldLoop)' -count=1 -json > .local/basketball/rea/captured-player/final-race.json
GOOS=js GOARCH=wasm go test ./games/basketball/engine -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" -run '^(TestWorldCaptured|TestWorldKind10CaptureToComputerEscape|TestOriginalCaptured|TestCapturedDropRequest|TestKind10|TestBallDeflectionAndDispatchAgainstOriginal|TestBallRequestRetainsIllegalAndProcessedRequests|TestFalling|TestWorldLoop)' -count=1 -v > .local/basketball/rea/captured-player/final-wasm.log
```

重建原指令夹具需要工作区内已有的授权 ROM、初始 RAM 和 py65；运行 Go 及 Wasm 测试只读取已提交夹具，不需要 ROM。

失败证据保留在同一产物目录：`world-before.log` 为缺少捕获调度分支，`fall-before.log` 为误将特殊持有人纳入普通坠落所有权检查，`device-reset-before.log` 和 `device-diagnostic-state-before.log` 为旧装置判据及逐步状态，`device-contract-before-ready.log` 为修正判据后保留的真实失败。最后一份记录显示旧前态、表面落地和原指令对照均得到 `JumpPhase=37`，原程序要求为 0。`device-contract-after.log` 记录修复后全部通过。中间编译窗口的日志单独保留，不作为行为验收结果。

最终验证：`final-race.json` 中 22 个顶层测试、含子测试共 28 项通过，包耗时 11.564 秒；`final-wasm.log` 中相同范围通过，包耗时 3.514 秒。两次进程均以状态码 0 退出。范围含新增装置回归、原捕获链、Kind10 接触、球请求、坠落和已有 WorldLoop 轨迹；没有运行其他游戏或把受控验证记为完整比赛。
