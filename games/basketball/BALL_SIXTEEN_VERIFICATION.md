# 特殊球动作 16 恢复

## 隔离验证前列出的失败场景

- 初次进入必须执行原投篮目标、发射和音效；已进入的动作不能重复清零或重复释放持有人。
- 发射目标读取真实持球人和第二名球员位置；无主球读取其独立标志，不能虚构球员。
- 初始竖直速度为 4 像素／更新，重力为零；不能误保留普通投篮抛物线。
- 两侧转折线、精确边界、向内／向外运动、零速度和小于 1 像素的正速度分支不同。
- 只在竖直速度非负时判断转折；向下飞行只进行原下降速度限制。
- 动画 11／12 循环，JSON 恢复和连续推进不能回到第一帧。
- 运动阶段可能改变球和篮筐选择，尾部必须读取当前值；未知转折选择不得用动作 16 常量替代。
- 下降限制只改整数高字节，保留小数；落地转动作 6，不制造得分或碰撞结果。
- 连续原指令链覆盖发射、转折、下落、落地；隔离链不代表自然完整比赛验收。

原入口 bank 6 `$BB78`。证据与执行结果如下。

## 原指令与语义实现

REA / Ghidra 证据保存在 `.local/basketball/rea/ball16/analysis/`。批量分析证据 ID 为 `ev_c1c6d67c5d2a4c3058283078065a3c4ad59de065a6ef4be45664de326012cb4d`，包含 `$BB78`、`$B998`、`$BDD2`、`$BD14`。ROM、初始 RAM 及夹具 SHA-256 见 `engine/testdata/ball16-source.json`。静态分析用于定位分支；实际结果由未改写 ROM 的 py65 调用验证。

`PrepareOriginalBall16` 在 `$BB8E` 动画前结束。原发射保留水平和深度速度，将竖直速度设为 4 像素／更新并清空重力。先提交原音效 30、球权与目标信息，再执行场地物理。

`CompleteOriginalBall16` 复原动画帧 11／12，按原场地函数更新位置，再读取当前篮筐选择。竖直速度非负时，左右转折线分别为 80 和 447；原函数比较整数 X 和速度整数高字节，右移亚像素速度不会触发右侧转折。转折后走 `$BAAB`，归位至对应篮筐 X、深度 45，并进入原下落。下降速度限制保留小数部分；接地最后转入动作 6。组件不生成得分或碰撞。

场地物理若替换动作且竖直速度仍非负，原 `$BDD2` 会按新动作索引转折表。本次保留明确的未恢复错误，不以旧动作替代新动作。世界接线应沿用动作 18 的准备／完成边界，运动后的篮筐通过回调读取，不能再次覆盖已提交的准备信息。

## 已保留的验证

- 370 次原指令调用，比较完整准备边界、完整结果及音效；覆盖初入、已进入、无主持球状态、特殊标志、双向边界、零速、亚像素、负速度、下降限制与落地。
- 6 条连续轨迹，共 397 步，左右方向各覆盖初始高度 30、100、210。Go 从每条首状态独立推进，不用后续原状态回填；每步经过 JSON 保存恢复，均抵达动作 6。
- Native、race、Wasm 定向验证均通过，最终结果见 `native-final.log`、`race-final.log`、`wasm-final.log`。单元测试先于实现编写，缺少 API 的失败保存在 `before.log`；并行任务编译窗口日志保存在 `after.log`、`race.log`、`wasm.log`，不能视为本组件规则失败或通过。

边界夹具经过复核：目标 X 取运动后的 79／80／81、446／447／448，生成器逐条断言物理完成时的整数 X；初始夹具仍保存在 `.local/basketball/rea/ball16/initial-ball16.json.gz`，最终夹具重新执行 Native／race／Wasm。

可重复执行：

```sh
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_ball16.py
go test ./games/basketball/engine -run '^TestOriginalBall16' -count=1
go test -race ./games/basketball/engine -run '^TestOriginalBall16' -count=1
GOOS=js GOARCH=wasm go test ./games/basketball/engine -run '^TestOriginalBall16' -count=1 -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec"
```

此处仅验证动作组件和受控原指令连续链；世界接线、自然整局、浏览器视觉与其他特殊球不在本组件已通过的范围内。没有修改原版悬空球缺陷。
