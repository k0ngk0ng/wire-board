# 世界前置阶段组合验证

实现前列出失败场景：

- 人物刚切换动作、尚未进入时，将上一动作的动画游标当作新动作碰撞帧；原搜索对此类人物直接跳过。辅助对象的目标搜索规则不同，不能连带跳过，也不能把已进入人物的缺失几何默认为空。
- 原八个角色类型共用动作／接收标志及形状选择表，不能因为某队未在既有记录出现过而拒绝已恢复动作；球和道具不得连带当作人物处理。

- 接触重置、搜索、骑乘、战斗／接球、区域缓存、比分等级、队伍策略和倒序篮板阶段顺序改变。
- 先算篮板再算区域，或过早同步 `PreviousHolder`，改变 AI 输入。
- 在组合状态重复存储依附或所有者，导致阶段写回丢失、互相覆盖。
- 攻击／接球后仍使用旧人物动作生成队伍上下文。
- 混淆 `SelectedHoop` 的原对象编号 7～12 与组件索引 0～5。
- 篮圈使用错误的最后投手、投篮技能、得分加成、起投位置或共享 RNG。
- 纽约 `$06D8` 同时保存为人物通道状态和篮板切换状态，形成两份真值。
- 重挂篮筐时猜测 mount owner；不能以布尔可接球代替原 owner 身份。
- 只核对最终分数，遗漏接触轮换、候选、动作请求、所有权、区域和声音顺序。
- 子阶段失败时留下部分攻击、分数、随机数或已发声音。
- 把 1,997 个独立原始输入的完整阶段测试称为单一初态的连续整场回放。

使用 `.local/nes-basket/runs/world-preparation-cpu-long` 已捕获的原 `$D317 → D326` 完整阶段。测试先于组合实现；比较所有纳入语义状态的字段及有序声音。只支持已核验的纽约场地编号 1；其他场地复用 RAM 的含义尚未组合，明确返回错误。

## 实现和状态边界

`StepWorldPreparation` 依次组合现有的 `BeginOriginalContacts`、`OriginalContactProfile`／`FindObjectContacts`、`StepOriginalRiderLinks`、`StepOriginalInteractions`、`CourtZoneForActor`、`OriginalScoreDifference`、`StepOriginalTeamAI` 和 `StepBoardContacts`。组合层不复制这些规则。

`WorldPreparationState.Actions` 是人物、辅助物、球、篮筐、依附和所有权的唯一持久来源。`Contacts` 只保留轮换、响应标记、候选、起始球动作及奇偶值；阶段临时组件需要的 `Links` 从 `Actions.actionLinks()` 投影，写回使用 `setActionLinks()`。

`PreviousHolder` 在队伍策略调用成功后才同步。区域缓存和比分等级使用篮板处理**之前**的状态，不能按本阶段结束后的坐标和分数重算。篮圈上下文由当前 `Actions.LastShooter`、其人物动作和角色投篮参数、`ShotOriginX`、`SelectedHoop` 构造。`SelectedHoop` 保留原对象编号，只有传给篮圈组件时将 7～12 转换为 0～5。

纽约场地的 `$06D8` 只持久存储为 `Actions.Actors[1].Player.NewYorkPassage`。临时 `BoardContactState.CourtToggle` 从该字段读取并写回，组合状态不建立第二份值。当前原阶段没有篮筐 owner 字节变化；篮筐布尔组件不能反推出全部 mount 身份，因此适配器保留原 owner，避免无关写回将未识别编号归一化成 15。

`WorldPreparationContext.Actions` 提供原角色参数和静态场地配置。`Kinds` 来自 `$03D6..03DC`，`Suppressed` 来自上次投影 X 高字节 `$03F4..03FA != 0`；不能用当前世界位置或宽松的绘制可见区替代。`AttackGrades`、真人战术／命令／状态和篮板强度均显式传入。恢复阶段不读取 ROM、RAM、文件或网络。

任一子阶段报错时返回完整原输入状态，声音列表为空；只有全部阶段成功才返回有序声音请求。首次比赛初始化直接进入人物输入，调用方不能给首步额外补跑本阶段。

## mode 11 队伍策略缺口

组合原记录在第 1,369 个阶段（原帧 7614）暴露原 `StepOriginalTeamAI` 对已进入 mode 11 地面持球者过早报错。复用 REA 已保留的 `$9CF8` 证据 `ev_07eaf15f099c93a5a0f6613a4e0d132dbdd15765b5a764f002e85ad934bec0fa`，并核对 `$9D43..9D9C` 与 `$B2F4` 指令后，补齐该分支：

- 零高度、负高度、任一篮筐不为已进入的 ready，或场地表不允许时，正常递减刷新计数。
- 六个篮筐都精确为原动作 `$81`、持球者地面正高度且场地表允许时，保留刷新计数，并按原角色排序分配模式 28／29。
- 对手二人已为互补的 28／29 时保留原状态，不重复重置。

`TeamAIContext.HoopEntered` 单独补足原 `$81` 判断所需信息，调用方从同一份篮筐状态构造。原帧 7614 自然覆盖了零高度的正常递减；正高度分派另有指令支持的针对性测试，不能声称本次完整比赛记录自然覆盖了所有分支。

## 验收结果和复现

1,997 个完整阶段的全部语义状态与原始记录一致，45 条声音请求的数量和顺序一致。原采样的 6,000 帧 RAM 已再次与 `hoop-control-cpu-long` 比较，逐字节相同。这里每个阶段都使用其独立的原始输入，不是从一个初态推进全场的连续验收。

源数据由 `scripts/basketball/extract_world_preparation.py` 使用已有 `decode_world_action_state` 统一解析，输出 `engine/testdata/world-preparation.json.gz` 和来源摘要。原始 RAM 不进入生产 Go 代码。

```sh
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/extract_world_preparation.py
go test -race ./games/basketball/engine -run 'TestWorldPreparation|TestOriginal.*Team' -v
```

阶段验证、错误回滚、状态序列化、篮筐 owner 适配回归及原队伍策略测试通过。原生竞态、Wasm 记录与首次失败日志保存在 `.local/basketball/rea/world-preparation/`。组合层仍需接入真正的连续世界循环，与相机、时钟、输入、动作和场地后处理共同验收。
