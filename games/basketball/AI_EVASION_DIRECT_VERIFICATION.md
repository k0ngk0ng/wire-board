# 运球避让的其他原直接分支

实现前失败场景：将只在直接分派中存在的计划塞入随机表；计划 6 的计数与随机消费次序错误；将负计数或负竖速当成有符号比较；计划 7～9 混淆起跳前后方向或提前改物理；计划 10 重复重置既有跳跃；计划 12 计数溢出、方向翻转和非法原方向字节被归一化；计划 13 把动作等待改成固定倒计时；原共享 PlanParameter 的其他写入或相邻索引别名被误认作自然入口；失败后部分写回。原指令夹具和测试先于实现。

## 已恢复的直接分支

| 计划 | 原入口 | 行为 |
| --- | --- | --- |
| 6 | `$B8D8` | 首次计数为 0 时请求闪避并按原深度阈值消费一次随机数；等待闪避动作完成后恢复跑动，保留原计数和计划写入次序。 |
| 7、8、9 | `$B92C` | 按原跳跃阶段及竖速高字节请求跑跳／跳跃；分别使用斜向、水平和向下方向，不直接改位置或命中。 |
| 10 | `$B95D` | 尚未处于持球跳跃时才请求跳跃；已在该动作时保持状态。 |
| 12 | `$B9A7` | 请求持球跑动，计数每 8 次翻转方向；按原无符号计数进入模式 17。 |
| 13 | `$B9CB` | 先切换持球待机并清零计数，达到原等待阈值后切回跑动及暂停计划。 |

未修改 `$BA5C` 对应的八队随机表，表中仍只有计划 0～5、11。恢复直接分支不等于把这些计划加入自然随机选择。

计划 12 的原方向字节 `FF` 在翻转时变为 `7F`。现有 `Heading` 类型没有这一原始值的精确表示，因此保留明确、原子的未恢复错误；没有把它归一化为普通斜向。4 个席位的原调用证明该字节结果，当前未证明它在正常进入路径中出现。

## 原调用与回归结果

10,752 次原 `$A8DE` 调用覆盖已进入模式 26 的全部上述计划、四席位、动作首入位、计数溢出、区域、随机阈值、跳跃阶段、竖速和方向翻转。另有 28 条固定外部上下文的持续调用链，共 352 次调用，保留同一原 CPU／RAM；计划 6 在未执行闪避动画时继续等待，测试没有替它推进动作。链内不注入下一动作、随机数、跳跃阶段或计数。

Native、race、Wasm 定向检查通过，且包含既有运球避让、计划 2／3／4 和计划 11 的原证据回归。旧测试使用计划 13 作为未恢复哨兵；本轮恢复后将哨兵改为 14，旧失败保存在 `regression-sentinel.log`。这些调用链是 AI 组件验证，不是完整物理、完整比赛或原版菜单路径。

REA / Ghidra 证据保存在 `.local/basketball/rea/evasion-direct/analysis/`。分派 `$B7CD` 的证据为 `ev_4cc09a551f3605a39a7e3d8c15e8ecfa642769b2bc61ef01aed1657e6e9e9cf2`，其已解析调用者为 `$A8DE`；各直接处理函数保留独立函数证据。

## 实际调用者和入口限制

`audit_evasion_callers.py` 保留全部 8 个 PRG bank 中的字面地址候选，包括 22 个直接参数写入候选，以及可能通过索引碰到参数字段的 214 个相邻基址写入候选。候选字节序列不自动等同于已确认代码或可达路径；索引范围也不能仅由基址推断。

在这次有界扫描中，字面 `$B7CD` 调用来自 bank 5 `$A940`；直接立即数写入模式 26 的候选是 `$A774` 的 `LDA #$1A`，随后进入 `$A8DE` 时仍会执行原随机选择。没有据此将直接分支标记为自然不可达，因为间接写入、计算跳转和其他配置未全部穷尽。

同时对既有 18,000 视频帧原版参考做只读观察：保留了 486 次参数字段写入和 16 次 `$B7CD` 分派，观察版全部 RAM 及 1 张截图与参考一致。此次实际避让计划仅有 0、1、2、4、5；未观察到本轮七个直接分支。参数值 6、7、10、12 在其他模式（2、6、9、19、38）出现，不能把共享字段中的这些值当成避让模式 26 的入口证据。观察到的参数写入 X 仅为 1～3，不等于所有配置的索引证明。

相邻索引别名候选和未知自然可达性继续保留；未增加入口、改变随机权重或设计新电脑策略。结果分别位于 `caller-audit.json`、`caller-capture.json` 和 `result.json`。

```sh
source scripts/env.sh
python3 scripts/basketball/verify_evasion_direct.py
.local/monopoly/venv/bin/python scripts/basketball/capture_evasion_setters.py
python3 scripts/basketball/audit_evasion_callers.py
go test ./games/basketball/engine -run 'TestOriginalEvasionDirect|TestOriginalAIDribbleEvasion|TestEvasionChoices|TestOriginalAIPlan11' -count=1
go test -race ./games/basketball/engine -run 'TestOriginalEvasionDirect|TestOriginalAIDribbleEvasion|TestEvasionChoices|TestOriginalAIPlan11' -count=1
GOOS=js GOARCH=wasm go test ./games/basketball/engine -run 'TestOriginalEvasionDirect|TestOriginalAIDribbleEvasion|TestEvasionChoices|TestOriginalAIPlan11' -count=1 -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec"
```
