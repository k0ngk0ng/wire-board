# 篮球与道具接触

实现前核验：把候选道具改成玩家；忽略原特殊门槛而总是反弹；将球 8／10 对地面或深处道具的无反应误判为漏实现；把有符号重力整数高字节当整个值判断；提前改变球速；错误清球权、最后触球人或接球排除；道具朝向使用小数／有符号 X 而非原整数比较；道具动作 6 的特殊持有人未解除；无效对象引用被静默跳过。

原 bank 5 `$84C8` 在检查球权之前分派候选 4／5 到 `$8596`。球动作 8／10 仅在道具动作 0、整数高度非零且整数纵深小于 71 时继续；其他球动作直接进入反应。反应把球置为动作 7，按原高字节条件修正重力，再调用既有 `$87EA` 道具反应，不在本阶段直接反转球速。

## 恢复范围

新增 `applyOriginalBallPropContact`，在 `StepOriginalBallContact` 中按原顺序处理候选道具，再进入玩家接球分支。反应复用已恢复的 `requestOriginalPropReaction`：道具动作 6 会先把特殊持有人的人物动作置为待机并清特殊持有人；道具改为动作 7，朝向由球与道具的无符号整数 X 比较决定。Owner、Attachment、球权、最后触球人、接球排除和速度均保留，后续动作阶段才初始化反弹运动。本分支不新增声音。

原重力门槛只检查有符号整数高字节：小于等于 0 时写成 1 像素，正整数重力保留小数。球 8／10 的受限分支还保留高度只含小数时视为 0、纵深 71 恰好排除等边界。

候选不属于道具 4／5，以及需要解除但不属于人物席位的特殊持有人，仍明确报错并回滚整个交互状态。单独的 `ResolveAutomaticCatch` 不处理道具，调用者必须经过交互分派，不能借其返回值跳过真实反应。

## 证据与验收

REA 批次证据为 `ev_99118692809de381bf5d779cee8e1ce92973783d84f3140a04725faa8e070f48`，包含 `$84C3`、`$8596`、`$87EA`。3,072 次受控原指令调用逐字段核对，覆盖球类型、道具动作／高度／纵深、重力高字节、两件道具和严格／相等整数 X 比较。来源为固定 RAM 基底上的原指令执行，不计作自然完整比赛。

真实故障快照来自 `court3-roster2-controls0--1-2--1.json`，原 video 4544／logic 1512。测试先复现同一错误，再从不变前态推进 24 个视频更新至 4568。本次真实候选是地面闲置道具与球 8，原 `$85B9` 明确不产生反应；解除错误是恢复原分支，不是删去交互。完整比赛由主任务继续验收。

定向 race 和 Wasm 全部通过，包含新分支、旧接球／道具／特殊球交互，以及原 1,998／57 次连续轨迹；race 包级耗时 9.657 秒。既有回滚测试原先用合法候选 4 触发未实现错误，现改为实际越界对象 13，保留原子回滚断言。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/extract_ball_prop_contact.py
source scripts/env.sh
go test -race ./games/basketball/engine -run 'Test(OriginalBallProp|BallProp|AutomaticCatches|CatchEligibility|AttachedAndOccupied|OriginalContactCandidates|OriginalSpecialBallContact|SpecialBallContactFailure|OriginalPlayerProp|OriginalCompleteInteraction|OriginalPlayerCatchFlags|Interactions|WorldLoop)'
```

证据保留于 `.local/basketball/rea/ball-prop-contact/`：`before.log`、`after.log`、`race.json`、`wasm.log` 和 REA 导出。两份夹具各有来源摘要，真实 Go 快照与原指令矩阵明确分开。
