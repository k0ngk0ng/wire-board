# 持球动作 2 的自身索引恢复

实现前失败场景：

- 把原 Holder 6 当作无效玩家，或猜测一个人物充当持球人。
- 套用运球动作 1 的自身坐标复制，遗漏动作 2 的初始化、动画、场地运动和偏移。
- 用运动前的自身快照计算偏移，抵消该次位移或场地修正。
- 首次进入的速度、重力、排除接球者清理直到运动后才提交。
- 已进入时重复清速度，或动画循环／左右镜像／整数坐标偏移破坏小数位。
- 把 Holder 4／5／负值等未恢复索引一并放开。
- 单球调用通过，但完整 13 对象动作阶段和真实 Mode128 失败前态不能继续。
- 用 Mode0 整局通过代替当前 Mode128 配置，或忽略 JSON／Wasm 续跑差异。

原失败为场地 1、默认球员、全电脑、比赛 Mode128，video21719／logic7224。原指令完整动作阶段也产生 Holder6／BallHeld2，不能改变持球者绕过它。

## 原阶段确认与实现

复用 REA / Ghidra 证据 `ev_83eb8e40e7f1fc57f720bf11e1cf022f07b914951cdf9616b092fa8c2fdb6a89` 中 `$B585` 和 `$EC7B`，另存于 `.local/basketball/rea/held-ball-self/reused-rea.json`。

从精确失败前态编码完整语义状态后，原 `$D314` 准备与 `$D345..$D348` 的完整 13 对象动作阶段确实发生：人物 0 在 `$9270` 把球动作 10 改为 2，Holder 保持 6；球随后进入 `$B585`。因此这是原代码允许的自身索引，不是重新选择人物的理由。

`$B585` 首入先清速度和重力、清排除接球者；随后执行动画和 `$EEFF`，再于 `$B5C0` **重读** `$053F`，写 `PoseSource`。`$EC7B` 从该来源的当前动作、姿态和坐标计算偏移。真实前态里来源就是运动后的球自身，action2 的偏移为 X−4、深度−1、高度+7 像素，保留小数。

独立组件 `self_held_ball.go` 保持准备／运动／偏移顺序。偏移复用原动作选择表和偏移表，使用运动后的动作与姿态，不硬编码 action2 或假造人物。运动后如果 Holder 已不为 6，则明确返回缺少该实时来源的错误；当前证据不能把其他场地的所有权改写当作自引用处理。普通玩家持球路径和其他非法所有者保持原边界。

## 已执行验证

- 432 次原 `$B585` 调用，覆盖首入／已入、左右方向、不同位置／高度／速度及动画循环边界。
- 48 次原 `$B5C0..$B5F3` 尾部调用，验证运动后不同动作、普通／举球／隐藏姿态和镜像时，读取实际来源表项。
- 完整 13 对象动作阶段的全部状态、调度顺序和音效一致；运动加载点能看到已提交的清理，改变所有者时保留明确错误。
- 精确 video21719、Mode128 失败前态继续 300 视频帧，每帧 JSON 恢复一致；旧自身运球回归通过。
- Native 1.263 秒、race 11.354 秒、Wasm 3.480 秒通过。普通持球、坐标小数、举球和完整偏移表的 6 项针对性回归 0.422 秒通过。

## 首次整局未通过，后续定位到绑定漏写

只复跑 `court1-roster0-all-computer`，保留 Mode128、固定种子和正常时钟。Native 和 Wasm 都越过原 video21719 错误，但在 48,000 视频帧上限未结束；两次运行源码均稳定，最终前态一致。不能把动作恢复通过当作整局通过。

最终前态为时钟 0、比分 51:54，Holder6、BallDribble1，球仍腾空，速度和重力均为 0，保留待处理偏转请求 8。原 `$D314` 加完整 `$D345` 单阶段对照也保持同一球动作、位置和请求，只推进动画 elapsed85→86。当时的单阶段证据不足以证明原机器完整比赛永久卡住；后续完整原机链反证了这一归因，见下节。没有修改比赛结束、AI、时钟或原悬空球政策。

失败与当前结果保存在 `.local/basketball/rea/held-ball-self/`：`native-config-summary.json`、`wasm-config-summary.json`、两份 `*-limit-before.json`、`limit-original-stage.json`。初始矩阵的失败记录继续保留。

可重复执行：

```sh
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_self_held_ball.py
go test -race ./games/basketball/engine -run '^TestSelf(HeldBall|Dribble)' -count=1
GOOS=js GOARCH=wasm go test ./games/basketball/engine -run '^TestSelf(HeldBall|Dribble)' -count=1 -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec"
BASKETBALL_PREPARATION_MATRIX=1 go test ./games/basketball/engine -run '^TestRuntimePreparedConfigurationsCompleteMatchAndRestore$/^court1-roster0-all-computer$' -count=1 -v
```


## 原机长链反证与首差修复

从原 video21719 失败前态只做一次显式语义对齐，随后原机继续 10,000 视频帧。无观察器与有观察器两次运行的全部 RAM、5 张截图完全一致；初始世界全部语义字段往返一致，包括真人命令 Accepted 字段。这不是自然菜单选项或原版完整两节比赛证明。

原机并未卡住：其连续链经过自身 held2，再恢复人物持球，66 个世界更新后正常进入 phase5，比分51:54。修复前 Go 与原机的前 6 个世界一致，第 7 个世界首次出现差异：原 Holder0，Go Holder6；当时所有人物字段仍一致，球位置和 PoseSource 因持球者差异开始分叉。

原 `$907E` 的人物运球静止动作 `$10` 首次进入时，在 `$9089` 执行 `STX $053F`；已有 `StepCarrierAction` 漏了这次绑定。进入动作必须在场地运动前重新绑定该人物，已经进入的动作则不重复绑定。现通过 `BindBall` 命令恢复，并在世界适配器的对应入口先提交、随后消费该命令，不在运动后重放覆盖。

因此，这次超时来自复刻的绑定漏写，**不能归为原版悬空球缺陷**。自身索引 6 的原 held／dribble 行为依然是需要恢复的短暂合法路径；不能通过强制重力、改持球者或跳过它来消除现象。从 Go 后期状态对齐原机只能证明该状态之后的源行为，不能反推该前态在原机正常起点一定可达。

本次补充验证：

- 360 次原 `$907E` 调用，覆盖四名人物、初入／已入、原持球者0～3／6、不同位置和球动作；完整运动前绑定及13对象阶段一致。
- 独立 Go 从同一21719前态连续推进，66个世界全部与原机一致，直至phase5；最终世界通过转场保留的 Carry 验证，没有裁掉末帧。比较仅排除原零页候选 scratch 和已有独立图像验证的 Projection；其余全部持久玩法字段保留。`world-comparison-after-binding.json` 为66／66、无差异。
- 补充定向 Native、race、Wasm通过，覆盖新绑定、新旧自身球路径和完整66世界对照；保留 `binding-before.log` 等红灯证据。
- 从冷启动重新跑唯一失败配置：场地1、默认球员、全电脑、Mode128。Native、Wasm各完成两次正常两节比赛，均22373视频帧／7429逻辑步、84次恢复、比分31:48，轨迹摘要 `c2e5eda8b5b386a0adbfd5e60351354f9aef038580ec6d650efff15cdd67d376`。四次运行使用同一稳定源码摘要 `5ab82a7fc338e236ece15f476df5c6c268ec984c94bcb80809deeb72a8be90fa`，均 accepted。冷启动与旧21719前态是不同起点；更早的绑定修复会改变全程轨迹，不能混用比分作比较。

补充结果：`native-after-binding-config-summary.json`、`wasm-after-binding-config-summary.json`、`original-machine-result.json`、`world-comparison-after-binding.json`。原机采集与独立Go推进脚本保存在 `.local/basketball/rea/held-ball-self/`；原指令／66世界夹具通过 `scripts/basketball/verify_dribble_idle_binding.py` 重建。原 `$907E` REA 函数证据 `ev_a96fad29a2b9b83a9434e6600d9cfeb7538f31ceef2293f7fb50248c794b3c08` 继续复用。

```sh
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_dribble_idle_binding.py
go test -race ./games/basketball/engine -run '^(TestOriginal(DribbleIdle|SelfHeldContinuous)|TestSelf(HeldBall|Dribble))' -count=1
GOOS=js GOARCH=wasm go test ./games/basketball/engine -run '^(TestOriginal(DribbleIdle|SelfHeldContinuous)|TestSelf(HeldBall|Dribble))' -count=1 -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec"
```
