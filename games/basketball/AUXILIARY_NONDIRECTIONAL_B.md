# 持物无方向 B 的原动作

地面普通无方向 B 对应 `$4F` 放下道具，并非高抛。该路径已恢复并接入世界动作；输入仍请求原动作，没有改写成方向 B。

## 4F 放下

原 bank 6 `$9BA4` 使用人物姿态 15／3 ticks、5／4 ticks，结束后回 idle。判断在动画推进之后：进入第二帧且 clock=0 时，把所选道具设为动作 1 并清 attachment；clock=2 时才将其设为动作 0、owner=-1。其他帧不重复发命令。选择规则是优先 owner 等于当前人物的对象 4，否则选择对象 5，保留原规则而不重选最近道具。

接口采用两个阶段：`PrepareAuxiliarySetDown` 输出人物状态和 `AuxiliarySetDownCommand`；世界立即提交道具动作／归属，再调用 `CompleteAuxiliarySetDown` 执行原 EEFF。`world_auxiliary_setdown.go` 保持此顺序，避免门装置看到旧归属或让后提交的命令覆盖物理副作用。

REA 证据：`ev_2b5588b7f56a312eba6a999f127fbd52cb7aa05f20460fa4bfe45970af721552`。192 次原完整 E18B/9BA4 探测覆盖进入、每个动画时点、两个玩家索引、所选道具、朝向以及 Court3 门地形。新组件和真实世界适配都逐项比较整个 WorldActionState。

另有一条 17 周期原 805C 输入→人物／辅助动作链：A 拾起后只输入无方向 B，原请求为4F。Go 从同一初态持续运行真实输入及生产动作 callback，最终人物与道具回到 idle、道具 owner=-1。保留34个人物／道具动作边界及每次输入内存，未中途灌回坐标或动画。这是显式起点的原函数链，不是完整自然比赛。

原始夹具与 SHA 位于 `engine/testdata/auxiliary-setdown-*-source.json`；提取脚本为 `scripts/basketball/probe_auxiliary_setdown.py`。先失败及结果日志为 `.local/basketball/engine/auxiliary-setdown-{before,after,world-before,world-after,race,wasm}.log`。

## 68 与 65 的区别及后续边界

跑中无方向 B 请求 `$68`，仍走原角色攻击表 `$9FA7`：

- 4 个角色表项使用 handler 9（team/character 为 0/0、0/1、1/0、4/0）。原 `$A80E` 将持有道具设动作 10、清 attachment，人物使用 variant 12 并设置重复计数 3；每次调用发原声音。
- 7/0 使用 handler 8，读取当前人物 Pressed 的 A 位；已由动作上下文显式传递，缺失时仍报错，不能默认无按键。
- 其余27项的负 handler 有明确原行为：回到 `$23` 携物急停，不是通用高抛，也不是任选攻击。

空中 B 请求 `$65`，含义是角色空中 B 攻击。32个表项中，现有攻击组件覆盖 handler 2／19／15 的21项；其余11项分布于 handler 11／20／16／31／10。不能把已覆盖角色的行为外推到全部角色。角色映射记录在 `.local/basketball/rea/auxiliary-nondirectional/role-map.json`。

4F 与68现已接入真实世界动作。68恢复了32个角色表项，以及辅助动作8／9／10／11的联动，详见 [跑动无方向 B 恢复验收](AUXILIARY_RUNNING_B.md)。上述65缺失handler仍需独立恢复与门禁，未用修改输入规避。REA 辅助证据为 `ev_d34ee42f7f50a2d1a6e06a804e4df085557dcdaed7655cf9ff66330f8af30552`。

## 复跑

```bash
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/probe_auxiliary_setdown.py
go test -race ./games/basketball/engine -run 'TestOriginalAuxiliarySetDown|TestOriginalPickupAndNondirectional|TestWorldOriginalAuxiliarySetDown|TestWorldOriginalInputToNondirectional'
basketball_goroot=$(go env GOROOT)
GOOS=js GOARCH=wasm go test -exec "$basketball_goroot/lib/wasm/go_js_wasm_exec" ./games/basketball/engine -run 'TestOriginalAuxiliarySetDown|TestOriginalPickupAndNondirectional|TestWorldOriginalAuxiliarySetDown|TestWorldOriginalInputToNondirectional'
```
