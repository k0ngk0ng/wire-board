# 原版临时策略结束后的恢复

## 实现前失败场景

- cold5081人物2进入mode22，原21/22共用A8C1；不能直接改为默认追球。
- A8C1用actor/2选HumanTactics，但9C6D仍用actor*4+holder读9C8B；actor2/3会索引到相邻原字节。必须还原实际只读选择，不能擅自改成team*4。
- 原返回模式字节可能带Entered位；写入后保留全部其他AI字段、动作和随机流，不提前执行返回模式。
- 原清零0657+actor：odd席位对应Human.Status，even席位是此前未建模的相邻状态。新增EvenStatus仅保留原字节，不杜撰额外规则；不把even写忽略，也不清错队伍的Status。
- HumanTactics从现有输入配置读取；缺失或越界明确报错。原holder0..6可对应静态选择域，未知值不越界猜测。
- 原函数隔离矩阵必须覆盖4席位、6tactic、7holder、2mode、Entered与非零状态字节，并比较清理后的完整输入状态；自然样本另保留。

## 证据和实现

复用REA `ev_eef21fb30302a3ec59b8b3dc097be9a7fa2a559339d598963a8489a9d646759d` 的A8C1；保留原9B85/9C6D实际读取关系。`export_ai_resume_plans.py` 导出6个tactic×4个actor×7个holder的静态返回值表，保留actor2/3越过意图角色表时读到的相邻原字节。原函数确实会返回24或带Entered位的其他值；不替换为按team重新索引的“合理”结果。运行时仅查有界Go表，不解释ROM或指令。

HumanCommandState.EvenStatus保存0657/659，原Status继续保存0658/65A；零值omitempty维持旧存档和快照形式。AI发ClearCommandStatus命令，统一世界输入只清当前actor对应字节；兼容单人物入口同样返回该命令，未静默丢弃。HumanTactics沿用现有配置输入。

## 验收

测试先于实现。原18,000帧参考中全部43次自然A8C1调用完整WorldInputState一致，所有RAM/PNG与参考一致。另有2016组原A8C1隔离执行，覆盖非零状态、模式进入位、角色索引和全部既有tactic/holder选择；逐组比较整个世界输入状态，明确区别于自然对局。

该步骤越过cold5081，真实返回mode24，下一帧按原进入A49E。原1998输入与1998/57连续轨迹保持通过；最终Wasm与相关测试见 `.local/basketball/rea/ai-forced-shot/wasm-tests.log`。本子阶段早期仅因另一代理result测试尚未实现而临时按文件列表排除result_test.go；最终包级编译与验证未排除文件。

```sh
PYTHONDONTWRITEBYTECODE=1 .local/monopoly/venv/bin/python scripts/basketball/capture_ai_command_resume.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_ai_command_resume.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/export_ai_resume_plans.py
source scripts/env.sh
go test ./games/basketball/engine -run TestAICommandResume -count=1
```
