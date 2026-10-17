# Court 7 独立运动与原开场

Court 7 的独立运动绑定、原开场和背景动画空入口已完成 Native、race、Wasm 验证。本阶段只允许独立运动／辅助对象调用使用 Court 7；没有修改运行时、开场、世界循环的整体门禁或平台选项。完整比赛与真实输入验收仍须在共享接线后完成。

## 原场地行为

Court 7 的地形表有 11 行，纵深排他上限为 81。原图格共有 640 个高度 0、32 个高度 80、32 个高度 24 的格子。最后一行两侧高度 80，中部高度 24；不能套用纽约平台或纯矩形地面。

原 `$89FB` 在纵深整数 79、横坐标小于 120 或大于等于 392 时处理后排边缘：

- 高度整数低于 80 时设置边缘位 `0x40`，保留其他空气状态位；若当前动作为 14，则把纵深速度的整数部分改为 1，保留原分数部分。
- 高度整数位于 `[80,128)` 时，原位置和待提交位置的纵深整数各加 1，按单字节规则回绕，不向分数或更高字节传播。
- 高度整数达到 128 时跳过该修正。

这些变化发生在原表采样之后；不能重新查表或用对称钳位代替。原地面设备分派 `$8458` 对 Court 7 指向 `$84D6` 空入口，没有专属地面机关、得分或随机数效果。

原开场 `$D257..$D303` 使用 Scene 1 和 Court 7，辅助对象种类为 13／14，与 `court & 3` 的原种类表一致。它复用已有开场、输入和道具逻辑，不需要新增场地机关状态。

## 测试先行与独立运动接线

先列出纵深边界、后排高度分界、动作 14 的分数字节、空气／边缘位、辅助对象种类、其他对象写回及纽约通道误用等失败场景，再增加 `court_seven_binding_test.go`。首失败日志为 `.local/basketball/rea/court-seven/binding-before.log`，失败原因是独立运动门禁尚未允许 Court 7。

随后只修改两处：

- `court_motion_binding.go` 的独立场地绑定上限改为 7，未知编号仍拒绝。
- `auxiliary_rest.go` 仅在具有完整原运动绑定时允许 Court 7。缺少绑定仍报错，不借用纽约地形。

`verify_court_seven_motion.py` 对 13 个对象各执行 384 次原 `$EEFF`，合计 4,992 次。配置覆盖横坐标 119／120、391／392 等边界，纵深 0／78／79／80，持有、下落、动作 14 的纵深移动及高度 79／80／127／128。完整状态、其他对象效果、地形缓存及绑定提交全部匹配；观测到 312 次最终边缘位、1,546 次纵深整数变化、156 次动作 14 的纵深速度变化。设备音效请求为 0，符合 `$84D6` 空入口。

这是明确配置输入的原函数执行，不是 4,992 场自然比赛。原 ROM 指令不变，仅截取音效请求出口；未知设备写入仍报错。夹具及 SHA-256 位于 `engine/testdata/court-seven-motion*`。

## 被动原开场证据

`capture_court_seven_opening.py` 复用既有 Court 7 图像参考：只在首次 `$D1B5` 选择 Court 7，此后不改状态，保留原控制器输入。被动记录 `$D257..$D303` 的完整前后 RAM。捕获运行与未加观察钩子的 1,750 帧参考，其完整 RAM 和 6 张 PNG 逐字节一致。

原输入中辅助对象种类为 `[9,0]`，真实开场将其写为 `[13,14]`；Go `InitializeOriginalPeriod` 的完整输出与原调用一致，包括原角色、位置、所有者、计时器及其他初始化字段。记录为 `court-seven-opening.ram.gz` 与 `court-seven-opening-source.json`。这证明实际原开场初始化，不代表从原菜单自然选择本场地或 Go 已完成整个比赛。

## 没有专属背景动画

原 `$9B95` 的 Court 7 分派目标为 bank 4 `$A32C`，直接返回。既有 114 次原调用及 256 次原指令探针共 370 次，均没有共享状态、随机数、比分、图块、调色板或音效变化。`TestCourtSevenOriginalAnimationIsNoOp` 逐项比较这些原输出。

因此运行时对 Court 7 保持 `observeRuntimeCourtAnimation` 的空返回即可，不增加转动计时器、随机数副本或伪造的背景特效。人物、球、道具等自身动作动画照常运行；「无背景动画」不等于整个画面静止。场地基础调色板为 8，没有 Court 4／6 那样的动态调色板分支。

## 最小共享接线方案

由根任务协调以下改动，独立绑定测试通过不直接开放平台：

1. 将 `runtime.go`、`opening.go` 两处阶段校验、`opening_world.go`、`world_preparation.go`、`world_actions.go` 的场地上限扩展为 7。
2. 在 `opening.go` 和 `world_actions.go` 的原运动选择条件加入 Court 7，确保人物、球、道具和篮筐实际使用其原地形。
3. 不增加新的持久化机关字段。原开场／跨节已有共享字节初始化继续按源规则处理；背景动画保持空返回，基础调色板沿用 Court+1 的原来源。
4. 现有 `opening_test.go`、`runtime_test.go`、`runtime_court_six_test.go` 用 Court 7 检验「未来场地未开放」；真正接线后须把未知编号改为 8，保留未知场地拒绝测试。
5. 保留原种子、控制器请求和完整时钟，运行四默认控制配置及仅新增 Court 7 的 16 个准备配置。随后验证实际平台准备、开赛与浏览器场地／后排边缘／道具显示，最后决定可选范围。

目前未发现需要新增独立规则的 Court 7 专属机关，但通用人物／道具／AI 的未恢复动作仍可能由完整比赛或真人输入触发。不能用此独立批次宣称全部 32 名球员、任意操作、战役或已知原版悬空问题完成；本次没有调整悬空、卡局、超时或胜负规则。

## 复跑与证据

```sh
source scripts/env.sh
python3 -B scripts/basketball/verify_court_seven_motion.py
.local/board-art-env/bin/python -B scripts/basketball/capture_court_seven_opening.py
go test -race ./games/basketball/engine -run '^TestCourtSeven.*$' -count=1 -v
GOOS=js GOARCH=wasm go test ./games/basketball/engine -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" -run '^TestCourtSeven.*$' -count=1 -v
```

`.local/basketball/rea/court-seven/independent-{native,race,wasm}.log` 均已终态 `PASS`，耗时分别为 0.691／3.119／0.675 秒。未重跑 Court 5／6 的完整比赛或准备矩阵，也未运行其他游戏。
