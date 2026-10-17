# 原战术5与避让计划11验证

实现前失败场景：

- 原战术选择界面输入改变0666后，全队选择表可选个人mode5；不能只允许配置值而在个人分派报错，或替换为另一防守AI。
- mode5入场只清Counter并设Plan5；后续调用不能重复清零，也不能先修改原随机数。
- 原缓存zone阈值30、对手zone阈值50、松球的半场字节与A0/50边界必须按原顺序/无符号字节判定。
- 只有满足原威胁条件才进入BACC；其现有球反应包括拾球、目标、计数与随机更新，不应简化为追球。
- 回位目标固定22；到达时原B1E9面向当前Holder对象，Holder6是球而非人物，分数/位置小数不能影响整数比较。
- team2避让表真实含11；计划11是mode26的PlanParameter，不是个人mode11，不混淆或随机重选以规避。
- B97B首次选择走路11必须设Counter8和原纵向Heading；已是11时仅按8位DEC及符号判断退出到mode1/Plan17，保留原动作进入位与其他字段。
- 人类队友覆盖会改用team0避让表；持久计划与首次选择的随机数消费次数必须分别验证。
- 原选择界面/全队/个人及避让入口用原机器码串联，显式合成输入与自然游戏记录分开标注；原生、race、Wasm保留相同定向验收结果。

## 实现与原输入链

mode5恢复在`ai_home_guard.go`，由原个人分派入口选择；只调整原动作、朝向、记忆并复用现有BACC球反应，不写命中、得分或物理结果。plan11在`ai_dribble_evasion.go`按B97B恢复，不改变既有选择表或随机算法。

两条隔离原函数链都使用未修改ROM指令：

1. 原战术选择界面`B4B9`读取Right pressed，将cursor3增至4，`B4FE`写HumanTactic5；随后原`99F0`在真人队伍松球条件下选择搭档个人mode5，原`A05D→A229`执行该策略。界面采样调用在`B501`前停止，不包含渲染等待或完整菜单；不把当前运行中的A/B队友命令误称为Select修改战术。
2. 原`99F0`在team2全电脑持球配置选择mode17；近前方对手经`B15B`使mode17选择mode26。初始随机数固定`12345678`，mode17实际消费前两次随机，下一次`A8DE`使用第三次结果98选择team2表index6，得到plan11。没有在两次AI调用之间强写mode26或plan11，也没有重抽随机数。该链验证AI选择阶段，位置为显式配置，未推进整场物理。

## 验证结果

- REA `ev_ec24c8549046c6114830d3e8af0a1790fdbbd825ee9983299e860f53acd9fd7a`：A229、B97B、A8DE、B007、BACC均成功解析；完整产物位于 `.local/basketball/rea/ai-mode5-plan11/analysis/`。选择界面另保留带bank3 SHA的`selection-source.txt`。
- `ai-mode5-isolated.json.gz`的1584条原调用覆盖四席、入场/持久状态、zone22及边界、持球身份、松球两侧阈值和原BACC子路由，比较完整WorldInputState，包括未改字段。
- `ai-plan11-isolated.json.gz`的1200条原调用包括1008条动作/进入位/计数/zone矩阵，以及192条首次选择、人类队友覆盖、负计划参数与NY后场例外。覆盖DEC的127/128/255边界；无替代随机或新按键策略。
- 原选择链与team2对手链各保留独立夹具。所有原输入、来源RAM/ROM SHA、输出SHA、调用范围与音频返回桩限制见 `testdata/ai-mode5-plan11-source.json`。这些都是隔离原函数证据，不冒称自然原版完整对局。
- Native、race、Wasm相同30项定向测试全部通过，含原球队/个人、球反应、避让、1998次完整输入阶段及原子性回归。结果与日志SHA保存于 `.local/basketball/rea/ai-mode5-plan11/verification.json`。没有修改或重跑root负责的完整比赛门禁。

当前范围仅新增个人mode5与mode26的plan11；个人mode11、33/34/35和其他未恢复计划仍明确报错。mode5的原B1E9目标仅覆盖当前已恢复的持球身份0..3/6，未知来源不默认为人物或球。菜单呈现及自然操作进入完整比赛不由本项证明。

## 复跑

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_ai_mode5_plan11.py
source scripts/env.sh
go test ./games/basketball/engine -run 'TestOriginalAI(Mode5|Plan11|TacticSelection|ForwardOpponent|ControlAndHoopCollection|DribbleEvasion)|TestOriginalDribble(Attack|Obstacle)|TestEvasionChoices|TestOriginalSharedBallReactions|TestGroundBallReactionPickupWindow|TestOriginalCompleteTeamAIPhase|TestWorldInputs|TestUnrestoredAIStrategy' -count=1 -json
go test -race ./games/basketball/engine -run 'TestOriginalAI(Mode5|Plan11|TacticSelection|ForwardOpponent|ControlAndHoopCollection|DribbleEvasion)|TestOriginalDribble(Attack|Obstacle)|TestEvasionChoices|TestOriginalSharedBallReactions|TestGroundBallReactionPickupWindow|TestOriginalCompleteTeamAIPhase|TestWorldInputs|TestUnrestoredAIStrategy' -count=1 -json
task_go_root="$(go env GOROOT)"
GOOS=js GOARCH=wasm go test -exec "$task_go_root/lib/wasm/go_js_wasm_exec" ./games/basketball/engine -run 'TestOriginalAI(Mode5|Plan11|TacticSelection|ForwardOpponent|ControlAndHoopCollection|DribbleEvasion)|TestOriginalDribble(Attack|Obstacle)|TestEvasionChoices|TestOriginalSharedBallReactions|TestGroundBallReactionPickupWindow|TestOriginalCompleteTeamAIPhase|TestWorldInputs|TestUnrestoredAIStrategy' -count=1 -json
```
