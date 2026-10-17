# 真人持辅助道具输入验证

实现前失败场景：

- 持有辅助物时A/B走原83B9上下文，不能报错或套普通拳脚/持球策略。
- A使用当前Pose镜像写04F0/04F1并请求27；不能使用Facing或丢弃道具独立Heading。空中/附着条件由8353决定是否加1。
- B的水平判断读取本次控制器产生的Heading，不是上一帧Heading；原方向、3D附着、跑动、地面/空中分支顺序不能改。
- 同时存在LinkedActor时原83B9跳过道具所有权分派；两个Owner都相同时按4、5顺序取第一个，不误改另一件道具。
- 已持物必须在8374跳过近物搜索，即使腾空且CarriedPlayer标记成立，也不能重新拾物。
- 方向/双击跑/A+B跳跃沿原分组20；B保持、松开、短按和A/B两拍合并保留原计时，不发明取消规则。
- 输入阶段只能更新原请求、记忆、Heading/Facing及原命令；不能直接推进位置、抛出、释放Owner或替代后续动作接受。
- 测试通过实际控制器Held/Pressed调用原805C；独立矩阵不直接写最终Request作为用户操作证据，完整拾取链另按原E18B推进。
- 新增Heading按原04F0/04F1提取。旧主自然夹具的零值需有RAM审计，非零输入链需重提；持续动作测试只同步外部输入字段，不能重灌运动/动画状态。

## 原规则与实现

复用 REA `ev_2d5b2051cfeab8dbab30daf503e2548c1f4026363e3fed63bae88347c9d0c283` 的8374/83B9/8353原上下文分派，及 `ev_77f361411f66bf6eb0ac2be4ddae512d29e12f12ec526174d0f488f9c65e4301` 的805C/81E0原输入计时。

- `world_human_input.go`恢复持物A/B分派；当前控制器方向从本次InputResult读取。LinkedActor优先级、Owner4/5先后顺序、Pose与Facing的不同来源都保留。
- 已持物时不再因腾空/CarriedPlayer条件重新搜索附近物品。方向、双击跑、A+B跳跃、两拍按钮合并以及松B请求仍沿原分组和计时。
- `AuxiliaryState.Heading`代表原04F0/04F1，按与Player一致的Heading枚举保存，不使用omitempty隐去来源。A只写选中辅助物的Heading，B水平抛投只写其Facing；输入阶段不释放Owner、不执行运动。
- 原地持物A请求27，空中按8353条件可变28；水平方向B请求29，空中可变2A，松B后分别请求2B/2C。无水平方向的地面B请求4F，跑中B可请求68，空中中立B可回普通攻击65。没有把所有B操作简化为抛投，也没有虚构短按取消逻辑。

## 对照与连续链

`verify_held_auxiliary_input.py`执行未修改原805C机器码，生成1714个完整输入边界：1488个持物/附着/链接/按钮序列隔离调用，加226个连续动作链内输入调用。覆盖四席、两辅助物槽、Pose/Facing不一致、当前/旧Heading冲突、地空A/B、B保持/松开/短按、重复A、A+B合并、方向/双击跑跳和附着父对象分支。

五条连续链由第一次真实控制器A经原805C/E18B完成拾取，之后只改变控制器Held/Pressed：

| 链 | 输入调用数 | 原后续结果 |
| --- | ---: | --- |
| 地面A | 23 | aux14→3抛出并落地 |
| 水平方向B | 59 | 29瞄准，松B后2B→aux2高抛 |
| 跳起后A | 41 | 24跳起，28空中A投掷 |
| 跳起后水平B | 67 | 24跳起，2A→2C空中高抛 |
| 双击跑后A+B | 36 | 携物跑跳，Owner保持 |

Go测试从每条链的唯一初态持续保存输入/世界状态，先运行真人输入，再调用生产`advanceWorldPlayer`和`advanceWorldAuxiliary`，逐次匹配原动作后的完整状态。没有用期望值写Request、Owner、位置或动画；当前PickupFlags由Go输入效果累积。本项是两对象的原函数/生产动作适配链，不是其他人物同时参与的完整比赛或浏览器E2E。C000/FBCC只保留07FF音效锁存并返回，音频驱动不作为该对照对象。

## Heading来源迁移

`audit_auxiliary_heading.py`核对19174个原RAM快照（自然world actions/inputs/preparation和AI记录），04F0/04F1均为0，证明这些旧主夹具零值的来源。92个carry动作边界重新从原函数链提取Heading后，移除新增字段即可恢复旧夹具SHA `2541ae0b3cedd024400aab6dd7fc29dcffabb7fd308504125e7b2588db57ccd4`，所有旧字段完全不变。

原92边界动作测试只增加外部输入阶段的Aux.Heading同步，与原有Player.Request/Heading同属输入效果；位置、速度、动作与动画继续连续维护。迁移审计及原始来源SHA位于 `.local/basketball/rea/held-auxiliary-input/heading-migration.json`。

## 验收与范围

Native、race、Wasm同一13项测试全部通过，包含1714次新输入、五条连续链、原1117次主动输入、1998次完整输入/动作阶段及准备阶段回归。Native记录分为输入12项和补充完整动作1项；三种运行的测试集合一致，清单与SHA见 `.local/basketball/rea/held-auxiliary-input/verification.json`。

本次没有修改world_actions.go或world_loop.go；携物动作适配由其维护者提供。4F/68等原请求在输入矩阵中核对，但其完整动作范围不能由上述五条链外推。实际浏览器WASD/J/K拾取/投掷E2E由root独立执行。

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_held_auxiliary_input.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/capture_auxiliary_carry_chain.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/audit_auxiliary_heading.py
source scripts/env.sh
go test ./games/basketball/engine -run 'TestOriginalHeldAuxiliary|TestHeldAuxiliaryInputUses|TestOriginalActiveWorldHumanInput|TestWorldHuman|TestWorldInputs|TestOriginalAuxiliaryPickupAndThrowActionChain|TestWorldActionsEntireCapturedCorpus|TestWorldPreparationEvery' -count=1 -json
go test -race ./games/basketball/engine -run 'TestOriginalHeldAuxiliary|TestHeldAuxiliaryInputUses|TestOriginalActiveWorldHumanInput|TestWorldHuman|TestWorldInputs|TestOriginalAuxiliaryPickupAndThrowActionChain|TestWorldActionsEntireCapturedCorpus|TestWorldPreparationEvery' -count=1 -json
task_go_root="$(go env GOROOT)"
GOOS=js GOARCH=wasm go test -exec "$task_go_root/lib/wasm/go_js_wasm_exec" ./games/basketball/engine -run 'TestOriginalHeldAuxiliary|TestHeldAuxiliaryInputUses|TestOriginalActiveWorldHumanInput|TestWorldHuman|TestWorldInputs|TestOriginalAuxiliaryPickupAndThrowActionChain|TestWorldActionsEntireCapturedCorpus|TestWorldPreparationEvery' -count=1 -json
```
