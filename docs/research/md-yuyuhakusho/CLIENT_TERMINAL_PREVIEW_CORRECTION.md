# 终局 Preview 推断订正

撤回此前「completed checkpoint 的 Preview 没有绘制事件，因此会丢失最后角色画面」的推断。该推断没有比较真实终止帧，忽略了原规则在终止前的淡出与清场；不是已观察到的像素缺陷。

对原 49 条冻结轨迹重新采集实际最后 tick：42 条是自然终局，7 条是固定 pan 输入的非终局端点。每条重放均再次匹配冻结的最终 world 和完整 pixel trace 摘要。采集未改动规则、共享呈现算法、存档格式或终局行为，未运行 ROM 或模拟器 CPU。

42 个自然终局均满足以下事实：

- manager `0x8040` 在前一帧为阶段 26，终止帧执行 `clear-world` 后为阶段 0。
- 最后一帧恰有 3 个事件：`0x8000` 回调、`0x8040` 阶段 26 回调、`clear-world`；draw 和可见精灵像素均为 0。
- `VideoMode=0`、`Palette.Status=0`，仅 `0x8000` 仍有效；实际终态已淡出为黑屏。
- 经原样 checkpoint 恢复后的 `Preview`，与实际终止帧 RGBA 逐字节一致，差异数 **0／42**；原权威 checkpoint 完全不变。
- 预览帧号为原帧 **F**，没有增加到 F+1。非终局的 7 个 pan 端点仍按下一帧 **F+1** 预测，它们的正常像素差异不计作终局恢复缺陷。

42 个终局共同的 RGBA SHA-256 为 `811a21a28045551ab6fdb3a94f27d5aee5043f0ba626f166295fc2e5813444b0`。无需为了该推断增加持久绘制意图、SAT、像素、历史帧或近似终态；平台结算浮层按平台结果显示，Canvas 可以恢复原有黑屏终态。

[机器结果](../../../games/yuyuhakusho/verification/client-terminal-preview/result.json) SHA-256 为 `aee3e1d4531064bf467a6f0e87b0a41379a6bd7343b91f89b3713abf8849bf0a`。同目录 `command.json` 保存执行前后源码身份，四个 `sample-*` 目录各保留前一帧 checkpoint、终局 checkpoint、最后事件、原始 RGBA 和 PNG；probe 源码保存在 `runtimebridge/testdata/terminal-probe.go.txt`。

复现命令：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/probe_client_terminal.py
```
