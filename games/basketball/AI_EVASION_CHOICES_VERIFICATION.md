# 原版运球避让计划 2／3／4

## 实现前失败场景

- cold1909 由原随机表选中 mode26 的计划2。不能替换计划、追加随机抽取或跳过分支。
- 计划2比较自己与队友 X 的16位整数差的绝对值，严格小于96才把parameter改为5；本次不立即执行5。不以敌人距离或Depth代替。
- 绝对距离>=96时请求原Pass并转mode1，保留nextPlan、方向和随机流。差值跨16位和-32768边界按原字节语义处理。
- 同分派表的计划3只把mode改6，延迟到下次AI执行投篮规划；不立即消费随机数。
- 计划4发CarrierRunningJump、JumpPhase=-1，方向按阵营为Right／Left，mode1／nextPlan17；与计划5的斜向逃离不同。
- 原完整函数与自然入口记录先于实现。隔离原函数矩阵明确不算自然对局；新命令经既有统一应用入口，不在AI层执行物理。

## 原版证据与范围

本轮 REA 原函数证据 `ev_df0e795cb32e40f07adf423936c903e7a326d63ac3c6a24a6f0286d657a5dbd5`，位于 `.local/basketball/rea/evasion-choices/analysis/`，分别覆盖 B840／B87F／B885。B7CD 的原分派表对应计划2／3／4；既有随机选择表保持不变。

`capture_ai_evasion_choices.py` 在原 18,000 帧参考中监听这三个真实入口，保留两个完整自然调用（计划2、4各一次），全部 RAM／PNG 与参考一致。测试比较完整 WorldInputState，包含动作命令、AI模式、参数、JumpPhase、随机流以及未改动的其他人物／球。

`verify_evasion_choice_functions.py` 另用工作区 py65 隔离执行原 B840／B87F／B885，共1056组，覆盖4席位、正常及16位回绕坐标、距离95／96／97附近和-32768边界。全部结果与Go匹配。该矩阵是隔离原函数证据，计划3不能据此宣称有自然对局覆盖。

## 验收结果与下一边界

先写测试再实现。新增自然／隔离调用、原5次mode26调用、1998输入阶段、1998单初态连续更新和57主动完整更新，均通过原生竞态与Wasm验证。日志为 `.local/basketball/rea/evasion-choices/{tests-before.log,native-tests.json,wasm-tests.log}`。

固定cold配置同一种子推进到2885／6000，下一错误是特殊球20的接触响应（`preparation: interactions: special-ball contact response not reconstructed`）。完整前态保存为同目录 `cold-next.json`，已交主线程分派物理／交互恢复。本轮未改变随机权重、测试目标、物理或碰撞结果。

```sh
PYTHONDONTWRITEBYTECODE=1 .local/monopoly/venv/bin/python scripts/basketball/capture_ai_evasion_choices.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_evasion_choice_functions.py
source scripts/env.sh
go test -race ./games/basketball/engine -run 'TestEvasionChoices|TestOriginalAIDribble|TestWorld(Inputs|Loop)' -count=1
GOOS=js GOARCH=wasm go test -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/basketball/engine -run 'TestEvasionChoices|TestOriginalAIDribble|TestWorld(Inputs|Loop)' -count=1
```
