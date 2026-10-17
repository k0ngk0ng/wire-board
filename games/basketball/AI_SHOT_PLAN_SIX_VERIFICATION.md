# 原版投篮方案 6

实现前列出的失败场景：

- 对已经进入或尚未进入的动作必须同样重新请求原动作 `0x17`，不能保留动作初始化标记。
- 模式必须写为尚未进入的模式 1；不重置延续方案、计数器、目标、方向、跳跃阶段或投篮周期。
- 四个角色和方案参数的高位不能改变分派结果；原函数不消耗随机数。
- 真实球队 2 失败前态必须在原种子、原时钟下继续，并在逐帧 JSON 恢复后得到相同结果。

静态证据复用 REA `ev_8bac8418e07eaa8a98c2ac6472d25618bfc7cb159ee632963fe08b31581cafda` 的 bank 5 `$B4E3`，并核对 ROM 指令。原函数只有两次写入：`$0512+actor = 0x17`，`$0672+actor = 1`。独立原函数执行与自然游戏采样分别注明，不把合成矩阵称为自然比赛。

## 验证结果

`scripts/basketball/verify_shot_plan6_function.py` 在原始 bank 5 上执行 18,432 次 `$B4E3`，覆盖四名角色、全部 256 种动作字节、九个方向字节和方案参数 6／134。每次比较整个 2 KB RAM，确认除角色的动作和 AI 模式外没有修改。Go 测试比较完整决策，保留计划、方向、计数、目标、随机数及命令。

`TestShotPlanSixRealFailureContinuation` 从球队 2 的真实首失败状态（video 2624，原种子 `0x12345678`）继续 120 个视频帧，每帧对照 JSON 恢复后的结果。实现前两个测试均明确失败。第一次实现误把动作 `0x17` 的名称写成跑跳，原函数矩阵立即检出；保留该失败日志，最终准确使用 `ActionPass`（站立传球）。

定向 Native、race、Wasm 均通过；race／Wasm 同时覆盖现有方案 3／7 及自然样本。日志保存在 `.local/basketball/shot-plan6-20261018/`：`before.log`、`after.log`（保留失败）、`after-corrected.log`、`race.log`、`wasm.log`。

重新执行相同场地 1、球队 `[0,2]`、球员 `[0,1,0,1]`、全原版 AI 配置，两次完整比赛均通过：22,373 视频帧、7,429 逻辑更新、每遍 84 次恢复，轨迹 SHA-256 均为 `7ca7e57f8c7fa79fb52a7167a8e944ad7b1a9c1cfea9a521a4a8f1dbdcc02b48`。源代码前后哈希一致。完整诊断位于 `.local/basketball/shot-plan6-team2-20261018/native/`，未改种子或时钟，未放开服务器球队选项。

复现：

```sh
source scripts/env.sh
python3 scripts/basketball/verify_shot_plan6_function.py
go test ./games/basketball/engine -run '^TestShotPlanSix' -count=1 -v
go test -race ./games/basketball/engine -run 'Test(ShotPlanSix|ShotPlanThree|ShotPlanSeven|OriginalShotPlan|AIShot)' -count=1 -v
GOOS=js GOARCH=wasm go test ./games/basketball/engine -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" -run 'Test(ShotPlanSix|ShotPlanThree|ShotPlanSeven|OriginalShotPlan|AIShot)' -count=1 -v
python3 scripts/basketball/diagnose_team_rosters.py --teams 0 2 --characters 0 1 0 1 --court 1 --output .local/basketball/shot-plan6-team2-reproduction
```

以上证明该投篮分支及这一配置的完整比赛，不代表全部球队、球员或整款复刻已完成。
