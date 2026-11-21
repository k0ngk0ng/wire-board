# 回合、目标与电脑对照记录

规则对照使用 ReGameDLL_CS 固定提交 `1f16a7f25c14225931f8df1abc51f68d18ded839`，工作区来源与摘要保存在 `.local/cs16/research/reference-source.json`。公开重实现包含可选修复和扩展；以下内容是所选默认规则的静态对照，不代表原版运行逐位等价。

## 原版规则入口

| 内容 | 对照位置 | 当前规则 |
| --- | --- | --- |
| C4 下包 | `dlls/weapons.h` 的 `C4_ARMING_ON_TIME`、`dlls/wpn_shared/wpn_c4.cpp` | 地面包点持续下包 3 秒，中断后重新计时 |
| 引信与拆包 | `dlls/game.cpp` 的 `mp_c4timer`、`dlls/ggrenade.cpp` 的 `DefuseBombStart` 与 `C4Think` | 默认 45 秒，有钳 5 秒、无钳 10 秒；有效拆包与引信同帧时沿用参考条件的拆除优先 |
| 爆炸衰减 | `dlls/basemonster.cpp` 的 `RadiusDamage`、`dlls/combat.cpp` 的 `RadiusDamage` | 默认伤害 500、半径 1750 原单位，线性距离衰减；该 CS 分支不采用 CZero 的可见面积计算 |
| 回合经济 | `dlls/gamerules.h` 的 `RewardAccount`、`dlls/multiplay_gamerules.cpp` | 爆破图歼灭 3250、爆炸 3500、拆除 3250；营救基础 2500，按救出人数另给 CT 团队 750；营救者立即获得 1000 |
| 连败补偿 | `dlls/multiplay_gamerules.cpp` 的 `RestartRound` | 初始基数 1400，打破多次连败后基数 1500，连续失败每次增加 500，上限 3000 |
| 人质模型原点 | `dlls/hostage/hostage.h` 的 `VEC_HOSTAGE_HULL_MIN/MAX` | 原始原点在脚底，原始身体范围为 `(-10,-10,0)` 至 `(10,10,62)`；不会把脚底当玩家中心 |
| 人质移动 | `dlls/hostage/hostage.cpp` 的 `Spawn` 与 `DoFollow` | 使用独立跟随策略和真实世界碰撞；垂直运动采用 800 原单位每秒平方的整数重力，保留 18 单位台阶和连续扫掠 |
| 营救范围 | `dlls/hostage/hostage.cpp` 的营救检查、`MAX_HOSTAGES_RESCUE_RADIUS` | 原图没有救援实体时，脚底到任一 CT 出生点的距离严格小于 256 原单位；触发区域图继续采用对应区域数据 |
| 蹲跳 | `pm_shared/pm_shared.cpp` 的 `PM_Duck` 与 `PM_UnDuck` | 地面变换原点，空中保持中心；电脑跳边先起跳、再在空中缩小碰撞体 |

`cs_assault` 的 `RescueLegacy` 来自 BSP 实体清点。电脑选择 NPC 路线可达且在真实营救球体内圈的导航点，不扩大实际触发范围。

## 独立策略与边界

平台席位的电脑策略独立实现，不声称恢复原版电脑源码。导航数据来自已登记的 PODBot 第三方路线，并结合原 BSP hull 检查。玩家和人质的私有有序路径只用于规划，实际位移仍通过普通输入或 NPC 的世界扫掠推进。NPC 不能借路径直接修改位置到某个导航点。

`StepBotNavigation` 只维护私有路线元数据，不改变手动玩家输入、位置、资源或得分。服务端仅对显式电脑和主动托管调用普通电脑输入。人质私有路径和垂直速度保存在 `State.HostageRoutes`，不加入公开人质投影；恢复后继续同一路径和重力。

取人质与护送采用不同可行性约束：CT 本人可以使用蹲跳路径，人质撤离禁止蹲点、梯子和记录的跳边。移动中的领队不会让正在下降的人质重插已经无法到达的高点。敌人视线使用统一的墙体与烟雾遮挡；CT 未观察到 C4 时搜索静态包点，T 未观察到人质移动时守原图位置。

NPC 行走碰撞目前复用站立 world hull，射击和交互使用人质原始身体高度；没有声称还原 GoldSrc 全部任意尺寸实体 hull 选择、推挤或原版 NPC 动画时钟。

## 可复跑证据

先行失败场景见 `FAILURES.md` 和 `OBJECTIVE_FAILURES.md`。`objectives_e2e_test.go` 覆盖目标中断、回合幂等、经济常量、生命代次、烟雾、不可见目标、隔墙拆包、私有路径恢复、蹲跳和原图人质全过程。

`TestOriginalAssaultHostageEscortRouteE2E` 从正式 `New` 开始，仅通过 `QueueInput` 和 `Advance` 驱动四名人质的呼叫、跟随和营救，再正常结束回合；不直接设置角色位置、胜负或比赛时钟。隔离边界夹具另行设置初始局面，以验证明确的失败场景。

修复前后的 JSON 记录位于 `.local/cs16/verification/`。全四人质闭环首次通过记录为 `hostage-committed-drop-progress.jsonl`：最后营救发生在帧 29978，正常 5 秒结算后于帧 30478 完成。最后交付以同一源码摘要绑定的整包与浏览器验收记录为准。
