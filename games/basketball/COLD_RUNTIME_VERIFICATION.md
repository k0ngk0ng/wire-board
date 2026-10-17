# 冷配置长运行验收

当前固定配置已在原生和Wasm上各完成两次6000逻辑更新，四次轨迹摘要相同，scores=[12,55]。这项验收不包含视频比赛时钟，不代表整个游戏复刻完成。

## 实现前失败场景与范围

- 使用默认纽约球场、球队 0/1、种子 0x12345678，从 NewOriginalMatch 和原开球流程进入世界，不加载参考快照。
- 真人席位 0 持续不按键，其余人物由原 AI 控制；目标固定为至少 6,000 次逻辑更新。不将逻辑更新声称为完整比赛时间。
- 遇到首个尚未恢复的动作、AI、碰撞数据或别名必须失败，保存更新编号、错误和前态。不得换种子、缩短目标、跳过错误、注入快照或补写自创策略。
- 冷初始化若遗漏字段，会使原分支读取错误数据；恢复必须绑定原代码证据，并保留已有原自然运行轨迹验收。
- 保留确定性结果摘要和复跑命令；同样配置与输入第二次运行须产生相同摘要。

## 首个边界（历史记录）

首次执行固定测试在第 230 次更新停止：`actions: object 1: special shot has not been reconstructed`。人物 1 前态为动作 `0x1B`、待执行动作 `0x1D`，ShotCycle=4、AimWindow=32，持球者为 1。完整前态和固定配置保存于 `.local/basketball/rea/cold-runtime/first-failure.json`，原测试输出为 `first-failure.log`。该人物／球动作交对应恢复工作处理；当时尚未通过 6,000 次更新；以下历史失败未被删除或替换。

复跑命令：

```sh
source scripts/env.sh
go test ./games/basketball/engine -run TestColdConfiguredWorldSixThousandUpdates -count=1 -v
```

测试固定使用相同初始化与输入运行两次，成功时输出每步完整世界状态累积的 SHA-256，并核对两次相同。

## 自动保留错误和长期结果

`PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_cold_runtime.py` 在工作区内配置 Go 缓存并执行上述固定测试。可加 `--race` 或 `--wasm`，不可同时选择。每次分别保存对应 flavor 的 `*-tests.jsonl` 与 `*-result.json`，包含退出码、通过状态、命令、日志 SHA-256、完整首差前态或两次成功轨迹摘要。编译失败也保留原始输出，不误报为游戏成功。`--output` 只能选择工作区内目录。

最初实跑报告工具时，完整首差前态与首次记录逐字段一致，完成更新数为230、目标6000，报告明确失败。

## 当前完成结果

原生和Wasm的可复跑报告均为passed=true、exitCode=0，各有两条updates=6000的completedRuns。四次完整状态累积SHA-256均为`4a81c896e779e4bdec6ca7d8bd461c0fb92cdc20bc18a8e3ec580b4794afcef8`，最终scores=[12,55]。报告为 `.local/basketball/rea/cold-runtime/native-result.json` 和 `wasm-result.json`，对应JSONL保留精确Go命令和测试输出；首次失败产物继续保留。

初始化仍为DefaultOriginalMatchConfiguration，纽约、teams0/1、seed0x12345678，真人0闲置，其余原AI；运行期间没有中间快照注入，没有缩短目标或改变seed。所有原规则缺口逐一恢复并附各自证据，未补自创AI。此结果只覆盖该固定冷配置的6000逻辑更新，视频时钟、比赛结束和更多主动输入由独立Runtime验收覆盖。
