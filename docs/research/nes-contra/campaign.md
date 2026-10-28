# 日版战役、续关与第一轮结算边界

`games/contra/engine/campaign*.go` 实现战役资源、决策时序、续关菜单、暂停与终局命令。它不直接清理玩家、敌人或积分流水；World 执行带身份的语义命令，平台统一计分策略负责最终全员加减分。

本项依据日版 ROM、45 个 REA 片段和 117 组明确受控边界输入。199 条原版决策记录与 Go 结果一致，两次录制的完整 RAM／调用记录逐字节相同。这不是自然八关通关验收。Intro／Entrance／Interlude／EndSequence 的独立场景驱动已在后续研究中完成，见 [日版场景执行](campaign-scenes.md)；完整 World 与房间验收分别维护。

## 原版事实

| 内容 | 日版证据与行为 |
| --- | --- |
| 普通起始生命 | `$C2E9` 给活跃角色 reserve lives 设为 2，加上当前生命共 3 条；单人模式 P2 的 GameOver 为 1 |
| 续关机会 | `$C2C6` 初始化 `$3A=3`；进入 Game Over 菜单时先递减，非负才给出续关选择，因此可确认续关 3 次 |
| 全灭延迟 | 所有角色 GameOver 字节按位 AND 非零，设 `$29=96`，进入日版关卡阶段 `$0B`；不是美版 `$0A` |
| 继续当前关 | `$CDAF→$C2CF` 清两个局内分数、把两个增命阈值设回 200 个百位单位，活跃角色补 reserve lives 2；高分、当前关号、周目和已扣的续关字节保留 |
| 武器清除时机 | 全灭延迟结束后，阶段 5 已先清理临时运行态和武器；确认续关的 `$C2CF` 本身不再次清武器 |
| 普通过关 | `$CD1F` 保留 reserve lives、GameOver、武器、局内分数、高分与增命阈值；清理角色／场景运行态，关号加 1。已经 GameOver 的队友不会免费复活 |
| 第一轮通关 | 阶段 5 真正完成第 8 关时，关号 7→8、GameRoutine 5→6、周目加 1、DelayLow 设为 128；这里才发出一次 `cleared` |
| 原版下一周目 | bank 8/9 `$8498` 有条件清关号与子阶段，再把 GameRoutine 6→5。平台第一轮挑战在此前终局冻结，不再次进入同局计分 |
| GameRoutine 7 | 日版为声音测试菜单；不能当成正常战役终局或下一周目入口 |

默认标准规则使用普通生命数。`CheatLives` 参数仅对应原版已有的 30 命变体（reserve 29）；模块若开放该变体，必须提前标记练习，不得在计分对局中由客户端任意改变。本文未声明已恢复输入秘籍或声音测试产品入口。

`Continues` 保留原版字节语义：初始 3，第一次菜单中已经变为 2，耗尽为 `$FF`。界面和统计须调用 `RemainingContinues(LevelRoutine)`，它把当前尚未确认的续关机会计入剩余数，避免把 `$FF` 显示成 255 或把一次菜单展示算成已使用续关。

## 决策顺序与优先级

日版阶段 4 先处理暂停，再看 Boss 标志。Boss 已倒下时先切到阶段 8，本帧不直接结算；下一帧阶段 8 仍运行真正的角色／弹丸阶段，再检查全灭。全灭会转入 `$0B` 并优先于存活人数检查，不能让同帧的 Boss 标志直接发放胜利。

阶段 8 的结束等待有两个重要细节：低计时字节 `$FF` 是等待哨兵；通过等待后，只统计 `GameOver=0` 且角色 `State=1` 的人。正在死亡或入场的人不会被当作正常存活角色，至少一人恢复到正常状态后才进入阶段 9 的出口演出。

阶段 9 运行出口演出后，仍运行角色／弹丸和出口专用敌人阶段。它不经过阶段 4／8 的同一个全灭判断，World 不能在所有阶段前套一个通用「全灭立刻失败」分支。最后的 `cleared` 只来自实际阶段 5 的最后一关推进，不能用 `BossDefeated` 或一个前端「动画完成」布尔值代替。

原版 `$C177` 也不是普通的 16 位整数减一：低字节减到 0 时才递减高字节，本次仍返回未完成；后续零／零调用才返回完成。阶段 8 另对低字节 0／255 有前置判断。代码按这些字节状态运行，没有用近似秒数或固定延时替代。

## World 接口

`OutdoorWorld` 继续唯一保存 `GameRoutine`、`LevelRoutine`、`Level` 和 `BossDefeated`。`Campaign` 保存续关资源、计时、选择、子阶段、暂停、终局和命令序号。

```go
plan, err := campaign.BeginTick(simulationTick, &frame.World, input)
// 根据 plan.Work 执行实际 World 工作，然后调用匹配的 After 方法。
plan, err = campaign.AfterCombat(&frame.World, observedPlayers)
plan, err = campaign.AfterEnemies(&frame.World, observedPlayers)
plan, err = campaign.AfterPresentation(&frame.World, completed)
```

`simulationTick` 是持久化的 `uint64` 模拟序号，不能使用会回卷的原版 `World.Frame` 字节，也不是网络 nonce 或当前时间戳。重复的已完成 tick 不会再发命令；终局在保存／恢复后仍冻结。World 应在副本上完成整帧，执行命令后才提交。持久化和恢复前调用 `Validate(world, true)`，不保存尚有 `Pending` 工作的中间状态。

| Work | World 的责任 | Campaign 后续 |
| --- | --- | --- |
| `CampaignCombat` | 真正调用 `CombatFrame.AdvancePlayersAndProjectiles` | `AfterCombat` 决定全灭或是否继续敌人阶段 |
| `CampaignEnemies` | 按原版顺序处理地形、敌人、出生与必要反馈 | `AfterEnemies` 完成普通帧、过关等待或全灭延迟 |
| `CampaignExitEnemies` | 出口阶段所需的地形／既有敌人更新，不能无条件套普通出生流程 | `AfterEnemies` 完成本帧 |
| `CampaignInitializeLevel` | 准备关卡初始资源和展示 | 完成后 0→1，清子阶段；不提前清 Boss 标志 |
| `CampaignPrepareBattle` | 按日版头部准备场景、玩家使用的地图与图形状态 | 完成后清 Boss／AliveAtExit，播放对应关卡音乐，2→3 |
| `CampaignLevelEntrance` | 执行实际入场背景序列 | 真正完成后 3→4，清子阶段 |
| `CampaignLevelIntro`、`CampaignRunInterlude` | 恢复后的服务器序列代码推进自身状态与关卡阶段 | 通用 `completed=true` 不会替它们跳关 |
| `CampaignRunEndSequence` | 执行真实出口序列，保留它的阶段变化 | 随后仍运行角色／弹丸与出口敌人阶段 |

暂停的 `NoWork` 只表示不推进玩法。原版暂停时仍有 NMI、音频与部分图形活动，呈现时钟不能与玩法工作计划混为一谈。暂停开关按原版玩家一 Start 边沿及忙碌检查处理；平台可以另做明确的合作操作授权适配。

## Reset 命令的保留范围

`CampaignReset` 提供准确的四个角色资源字段及操作种类，World 不应简单重新构造整局。

| Reset | 场景／角色／敌人／弹丸临时状态 | 角色资源 | 分数与阈值 |
| --- | --- | --- | --- |
| `CampaignResetAdvance` | 清理 | State 归 0；Lives、GameOver、Weapon 保留 | 全部保留，包括高分 |
| `CampaignResetGameOver` | 清理 | State 归 0，Weapon 清零；Lives／GameOver 保留 | 菜单展示时仍保留 |
| `CampaignResetContinue` | 前一步已经清理，不再伪造一次全局复位 | 活跃角色 GameOver 清 0、reserve lives 补 2；单人 P2 保持不参战。现有武器字段不额外修改 | 两人局内分归零、NextLife 都为 200；高分保留 |

原版过关清理范围对应 `$40..$EF` 和 `$0300..$06FF`。World 执行语义清理时，要保留原本不在此范围的全局帧、关号和资源；不要顺手重置模拟序号、MatchID 或控制器身份。旧 Boss 标志一直保留到阶段 2 的真正准备操作，不能携带它直接进入下一关阶段 4。

命令中的 `ID` 单调递增。`resetRuntime`、`advanceLevel`、`restartLevel`、`offerContinue`、`stopAudio`、`cleared`、`failed` 都由服务端规则产生。玩家不能直接提交这些结果。结算层读取 `Outcome` 并复用平台既有合作策略，不能由本模块写积分流水。

## 验证与未完成范围

- [原版输入与数值来源](evidence/campaign-source.json)：SHA、切片映射、12 项日版阶段表、默认资源与音乐表。
- [199 条原版边界记录](evidence/campaign-boundaries-original.json.gz)：117 组明确受控输入及真实函数结果。
- [REA 索引与双录制结果](evidence/campaign-index.json)：45 个静态片段、完整 Evidence bundle 和字节一致性。
- [验证记录](evidence/campaign-verification.json)：测试先行的 red／green、源码与产物摘要。

Go 检查覆盖原版边界、完整三次续关机会、第四次全灭后的失败、Boss／全灭优先级、暂停、重复 tick、JSON 恢复、最终通关仅一次，以及错误状态不发生部分修改。测试均在对应实现或修复之前保留了 red；八关准备音乐的扩大对照还发现并修正了误抄值。

后续已完成 Intro／Entrance／Interlude／EndSequence 的独立驱动和原版对照，见 [日版场景执行](campaign-scenes.md)。完整 World、自然八关流程和浏览器验收不由本项替代；平台只依据真实战役终局结算。声音测试没有作为平台游戏入口。

复查命令：

```sh
python3 -B scripts/contra/campaign_prepare.py
node scripts/contra/campaign_rea.mjs .local/contra-campaign-rules/fixed-prepared-spec.json
node scripts/contra/campaign_rea.mjs .local/contra-campaign-rules/ending-prepared-spec.json
.local/board-art-env/bin/python -B scripts/contra/campaign_capture.py --name campaign-recheck-1
.local/board-art-env/bin/python -B scripts/contra/campaign_capture.py --name campaign-recheck-2
python3 -B scripts/contra/campaign_export.py --fixed fixed-prepared-rea --ending ending-prepared-rea --primary campaign-recheck-1 --repeat campaign-recheck-2
source scripts/env.sh
go test -race ./games/contra/engine -run '^TestCampaign' -count=1 -json
```

录制拒绝覆盖已有目录，重复时更换运行名称。本次保留主录制、最终分析和可重复验收记录，清理范围只限自己的失败试验、旧分析迭代和 Swift／Ghidra 临时缓存；原 ROM、root 的八关初态和其他任务产物没有改写。
