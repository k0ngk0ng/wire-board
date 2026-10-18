# 辅助物合作指令模式 13、3、14

实现前隔离失败场景：误选被其他玩家持有或未落地的辅助物；两个候选未按对象 4→5 顺序选择；辅助物为空时未清除本人命令状态或写错误消息席位；靠近时左右／深度边界、动作首入位错误；成功后提前改变物理或附着关系；等待拾取阶段丢失 plan 或辅助物持有人；搭档附着掩码错误；对手选择、目标位置与原 B05C 距离阈值误译；恢复存档丢失共享命令写入。先保留原指令 oracle 与失败测试，再实现。

## 恢复内容

模式 13（`$A557`）按原 `$AD78` 的顺序查找辅助物 4、5，要求所有者字节不属于玩家 0～3、动作 0、整数高度 0。没有可用对象时，只清除本人命令状态、设置本人消息 2、请求待机。靠近成功后只绑定辅助物所有者并请求动作 `0x4E`，进入模式 3，后续计划为模式 14；不提前改变辅助物动作、附着或位置。

模式 3（`$A187`）等待原拾取动作完成。丢失两个辅助物的所有权时转恢复模式、计划 8；本人动作变为 `0x20` 后，才进入保存的计划。

模式 14（`$A599`）恢复搬运／合作决策。搭档实际附着到对应辅助物时请求原 `0x2B` 动作；否则按原持球者和对手动作选择目标。原代码在 `$B05C` 后检查水平距离的高字节和深度，而非采用通用「到达目标」判据。进入攻击分支后请求 `0x27`，保留原朝向与后续计划 22。所有移动、物理和附着都由既有动作阶段执行。

`AIAuxiliaryControlCommand` 仅表达原所有者和本人消息写入；公共世界输入层统一提交到 `Auxiliary[].Owner` 与 `Human[actor/2].Messages[actor%2]`。失败命令使用已有的 `ClearCommandStatus`，偶数席位和奇数席位的状态字节分别保存，不能互相覆盖。

## 原代码证据

REA / Ghidra 证据保存在 `.local/basketball/rea/ai-auxiliary-cooperation/analysis/`。

| 原函数 | 证据 ID |
| --- | --- |
| `$A187..$A1B6` | `ev_f4625036fa3008ca5625b4d4316d51c999abc1bc75bf489f27b0b97eca153c60` |
| `$A557..$A598` | `ev_2e6ecc7904dfa6371680ff768b402c77fdddc3badbfe3ab1b006e815114616a5` |
| `$A599..$A638` | `ev_09e4e6ec659e0b6d31a9075e1605caba0f65e48b8d9448ecee2a1b0664c0c237` |

受控源指令矩阵包含 8,248 次调用，覆盖四席位、首入、两个辅助物的可用性、坐标边界、搭档附着、持球者及对手状态。额外核对持球者字节 `FF` 时，先保留失败，再修正为原无符号字节比较，未把该值误认作玩家。

另有 16 次完整世界输入状态对照，调用原 AI 分派 `$A05D`；Go 通过 `advanceWorldComputerInput` 提交副作用并比较整个状态。接线前失败记录 `canonical-before.log` 明确显示本人消息未更新。

## 耦合阶段验证

8 条持续链覆盖四席位与两个辅助物，共 100 次更新。每条链只初始化一次，依次执行原接触状态重置、完整输入阶段、完整动作阶段。实际拾取动画从动作 `0x4E` 推进至 `0x20`，AI 依次进入模式 13、3、14、0；每步比较源状态并执行 JSON 保存恢复，没有在中途注入动作 `0x20`。

这是上述三个阶段的耦合验证，不包含完整的接触搜索、队伍 AI、比赛时钟或原版菜单路径，不能替代平台 A／B 指令的完整对局验收。早期探索脚本未对齐人类控制器槽，产生无关输入差异；最终夹具明确配置原控制器 0～3 且输入为空，旧失败日志保留。

```sh
source scripts/env.sh
python3 scripts/basketball/verify_ai_auxiliary_cooperation.py
python3 scripts/basketball/verify_auxiliary_cooperation_chain.py
go test ./games/basketball/engine -run '^TestOriginalAuxiliary' -count=1
go test -race ./games/basketball/engine -run '^TestOriginalAuxiliary' -count=1
GOOS=js GOARCH=wasm go test ./games/basketball/engine -run '^TestOriginalAuxiliary' -count=1 -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec"
```

结果与源码摘要见 `.local/basketball/rea/ai-auxiliary-cooperation/result.json`，原 ROM、夹具 SHA-256 见对应 `testdata/*-source.json`。
