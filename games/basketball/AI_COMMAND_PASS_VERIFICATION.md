# 个人模式 10／11／30

实现前失败场景：

- 模式 10 未进入时未直接发出传球动作 17，或已进入时再次发出。
- 模式 11 在动作 10～13 以外传球，或忘记设置恢复计划 22；动作 13 在公共分派中可能被阻止，不能绕开原可行动判断。
- 模式 30 每次都把目的地重设为 10，覆盖球响应已经选择的目标。
- 球响应 carry 未就绪时清命令，或用响应前持球者判断而遗漏当次捡球后的命令清除。
- 清错人物的命令状态，尤其把人物编号改为队伍编号。
- 重复消耗随机数、覆盖计数／计划、改变未涉及的动作／朝向／位置。
- 被动作可用性阻止时仍进入个人模式；有支撑的动作 3E 未沿用公共分派规则。
- 未恢复球响应上下文时吞掉错误；新代码覆盖独立恢复的共享球响应。

源入口为 bank 5 `$A461`、`$A476`、`$A208`；模式 30 复用 `$BACC` 的原球响应。

## 源行为

REA / Ghidra 批量证据为 `ev_5681069475a63dd3365263e723c01ee88206c93e498114d7e1d0a94c27e37140`，保存于 `.local/basketball/rea/ai-command-pass/analysis/`。`$A208` 的反编译结果无法可靠表达 `$BACC` 返回的 carry，原指令验证在 `$A21A` 直接记录真实处理器 carry，不以反编译的 `in_C` 变量代替执行结果。

模式 10 只在首次进入时直接写传球动作 `$17` 和个人模式 1，不修改恢复计划。模式 11 标记为已进入；动作整数值位于 `$10..$13` 时写传球动作、个人模式 1 和恢复计划 22。直接写动作会清除 Entered 位，即使原动作值相同也不能使用「相同动作不重启」逻辑。

模式 30 只在首次进入设置目的地 `$10`，然后执行已有的原共享球响应。响应已就绪且响应后的持球者为人物时，清除当前人物的命令状态 `$0657 + actor`。当次拾取无主球也满足条件；不能拿响应前持球者 6 来判断。

新增组件只输出既有 `AIControlDecision`。球权和球动作由 `BindBall`／`Ball` 命令传出，命令状态由 `ClearCommandStatus` 传出；不直接修改世界或控制器状态。

## 原指令语料

- 6,304 次独立原指令调用。模式 10／11 同时比较直接函数和公共 `$A05D` 分派，覆盖动作可用性、Entered 位和两侧边界。模式 30 覆盖 11 种球动作、4 个高度、3 个距离、5 个计数、4 名人物和初入／已进入。
- 4 条原指令连续链，每名人物各一次，从已经建立支撑关系的模式 28 进入模式 10，再请求传球进入模式 1。链中仅更新原执行结果，没有重置每步状态；这是受控 AI 函数链，不代表完整真人 A／B 输入链或自然比赛。
- 原执行逐条断言全部人物和球的位置未变，并核对其他人物的命令状态未变。夹具保存完整决策期望、各人物命令状态和 ROM／夹具摘要。

测试先于实现编写，记录在 `before.log`。其他并行任务的编译窗口保留在 `functions-after.log`；实际验证结果完成后另记，不能把这些日志视为通过。

## 已执行验证

- 6,304 次原调用全部通过。对比完整 `AIControlDecision`、随机数、动作 Entered 位、球命令和各人物命令状态。
- 4 条模式 28 → 10 → 1 连续源链通过，每次决策经过 JSON 保存恢复后继续。
- 模式 30 实际记录 680 次命令清除，其中 40 次是本次捡球后清除；另有 320 次响应就绪但持球者仍非人物，保留命令状态。
- Native 定向检查 0.399 秒通过；连同已有 1,039 条共享球响应和拾取边界回归，race 4.197 秒、Wasm 1.040 秒通过。日志分别为 `after2.log`、`race.log`、`wasm.log`。

可重复执行：

```sh
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_ai_command_pass.py
go test ./games/basketball/engine -run '^TestOriginalCommandPass' -count=1
go test -race ./games/basketball/engine -run '^(TestOriginalCommandPass|TestOriginalSharedBallReactions|TestGroundBallReactionPickupWindow)' -count=1
GOOS=js GOARCH=wasm go test ./games/basketball/engine -run '^(TestOriginalCommandPass|TestOriginalSharedBallReactions|TestGroundBallReactionPickupWindow)' -count=1 -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec"
```

完整真人输入、团队模式、共享状态写回及整局链由集成验收单独证明；本报告没有将个人模式的受控调用当作整条 A／B 功能通过。
