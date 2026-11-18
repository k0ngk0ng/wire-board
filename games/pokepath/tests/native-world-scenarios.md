# 世界能力失败场景与验收边界

在实现 `engine/world_abilities.go` 之前登记。基线为 62 个能力 ID、50 个唯一 WorldHook；机器清单保存在 `.local/pokepath/native-world/audit.json`。公开规则文本与原程序的精确数值只作为行为事实，最终规则全部由本仓库 Go 状态与函数实现。

所有场景必须覆盖真实 `Apply → Advance → fire/hit/condition/heart/movement` 路径；只验证 AttackProfile 标记不算完成。无目标、越界、隐形／无敌、目标死亡、回收／移图、重复触发、持续效果过期、暂停、拆步、克隆与保存恢复均须保持正确。世界状态禁止闭包、Go 指针引用环和进程级可变缓存，全部用稳定实体 ID 与显式计时器序列化。

受控战斗夹具仅使用真实目录物种、技能与地图，直接摆放测试队伍、静止且耐久的目标用于隔离空间与时序；不修改目录规则。它们不写入平台存档、不构成正式难度或作弊入口，产物明确标记 controlled-fixture。

| Hook | 能力 ID | 必须观察的行为与失败边界 |
| --- | --- | --- |
| `addHoneyOnAttack` | `honeyGather` | 每次真实物理命中加 1 共享蜂蜜；所有塔获得相应威力，花蜜为 3 倍、单纯持花蜜为 4 倍；波次与地图重置。 |
| `allBurnUnitsSun25` | `solarPower` | 阳光下所有灼烧系真实命中已灼烧目标 +25%，其他系／非阳光不加。 |
| `allyIdleTarget` | `squishyToy` | 无有效敌人时实际向诱饵发射，消费正常冷却并触发适用攻击计数，不当敌人发金币。 |
| `attackWithFutureParadoxAlly` | `aggressiveNature` | 附近未来悖论真实发射才联动；范围／无目标／多来源事件不会递归无限发射。 |
| `beamOverFiveLoseHeart` | `steelBeam` | 同一束同时命中超过 5 个才扣 1 且不能致死；不是每目标扣，也不是非同时累计。 |
| `brutalFauna` | `naturesJudgement` | 真实 120px 场域，无甲目标承伤 ceil x1.2，同类场域不叠。 |
| `burnDeathExplosion` | `magmaStorm` | 被灼烧敌人的真实死亡触发熔岩爆炸，非死亡／非灼烧不触发，连锁各敌至多一次。 |
| `chance25Area` | `spikyShield` | 命中后的触发概率从 0 开始，失败加 0.25，成功清零；落点生成半径 60、持续 2600 ms、每 500 ms 普通直伤区域，去重 32 px／250 ms。 |
| `chargeBoostMeter` | `dauntlessShield`, `intrepidSword` | 每命中 +2%，满后 10000ms 强化逐渐耗尽，期间不能再充；真实伤害／范围／射速改变。 |
| `chargeOnNearbyStun` | `lightningRod` | 只因范围内新眩晕充能，每层 +10% 威力，10 层下次攻击真实范围伤害后清空。 |
| `commanderStats` | `commander`, `mount`, `supremeCommanders` | 仅吃吼霸／对应变身承载米立龙，一背一客；实际攻击间隔、伤害向上取整与暴击改变，其他乘客拒绝。 |
| `continuousRangeConditions` | `blizzard`, `flameBody` | 范围内持续减速／灼伤与禁疗，离开即停止刷新且保留应有残留；抗性与无敌仍生效。 |
| `continuousRangeExposure` | `discharge` | 连续停留满 3000ms 后眩晕 2000ms，每敌一次；离开重置暴露，不因存档重发。 |
| `copyFirstSpeciesAtOwnLevel` | `transform` | 保留百变怪身份与自有等级，复制 DNA 的真实攻击、地形、弹道与图像，克隆后继续一致。 |
| `countdownExplosion` | `explosion` | 3000ms 倒计时，不足 5 个合法目标停在 1000ms；范围内真实伤害／3秒晕，扣 2 心或被 damp 免除。 |
| `crossShapeWidth` | `victoryStar` | 十字／X 实际命中宽度 x1.5，普通圆形不变，不能只画宽而不改变命中。 |
| `deployGlobalStunAndMovementLock` | `heavyMetal` | 每波首次部署全场眩晕 2000 ms，并立即锁定移动／交换；每波刷新，不能重复部署刷晕。 |
| `endWaveHeartChance` | `triage` | 波次结束 5% 恢复 1，按共享 RNG，重复结算／重连不重掷。 |
| `erraticStatic` | `machBolt` | 真实 120px 场域，眩晕目标承伤 ceil x1.2，不把所有命中永久加成。 |
| `finalEvolutionShiny` | `scheme` | 每个真实投射物 1/9000 仅最终进化对象变闪，不能改变原物种或重复计收藏。 |
| `firstHeartLossDoubleHail` | `iceBody` | 每波首次伤害调用即激活，即使结实把损失减到 0；冰雹暴击倍率从 1.25 改为 1.5，波次重置。 |
| `firstWaveHeartLoss` | `sturdy` | 首个伤害调用减 1，每波一次；减到 0 仍激活冰冻之躯，但不激活需要实际损失的树果。 |
| `flyingTowerMovement` | `aerialAce`, `stalwart`, `unburden` | 真实塔身位置改变、部署原点与移动目标不同；圆周／追击／漂移参数精确且可存档。 |
| `globalBerryDuration` | `harvest` | 任意 harvest 使树果时间翻倍，多个不再翻，回收后新触发不沿用旧全局状态。 |
| `globalBerryEffect` | `unnerve` | 全局 unnerve 对真实四树果加成 x1.1，含ripen/simple相乘与取整。 |
| `globalBerryWaveStart` | `gluttony` | 开波刷新所有持有树果的真实计时，18000ms／harvest 36000ms；乘客→莓袋宿主单向传播。 |
| `halfDamageExecuteAura` | `destructiveMaw` | 活敌进入射程且生命低于一半时，独立于普通冷却发射一次半威力投射物；离区、回血到一半或死亡后解除该目标标记。 |
| `idleRechargeCharges` | `bulletPunch` | 无敌人也按充能间隔积累到 10，满后真实连射消费；不因无目标直接丢弃蓄力。 |
| `idleRechargeChargesAndKillRefill` | `bulletPunchMega` | 5 层蓄力与击杀回满，延迟连射、杀死／漏出目标、克隆之后次数一致。 |
| `ignoreItemPowerPenalty` | `defiant` | 道具负面威力被抵销且加 500 的结果体现在真实命中，不只在面板 profile。 |
| `impactSkyArrows` | `spiritShackle` | 先发生原箭命中，再创建 3 支落箭并真实范围命中；不能开火即先扣血。 |
| `justiceAllies` | `heartOfSteel`, `sacredSword`, `swordsDance` | 仅圣剑士获得全局对应加成，伤害后心之钢三倍；非圣剑士和回收后的增益不保留。 |
| `onlyUnheldAllies` | `helpingHand` | 全局每个帮助使未持物塔 +50，持物不享受，不能被普通近距 aura 再算一次。 |
| `oppositeChargeLink` | `minus`, `plus` | 仅异号连接、同号不连，敌人必须实际穿过线段才受伤；端点移动／回收／死亡后更新。 |
| `passengerPlacement` | `grassPlatform`, `icePlatform`, `rest` | 同一真实建造格的宿主／乘客合法性、最多一名乘客、加成地形与真实发射节奏；非法搭乘不改变布局。 |
| `persistentWebZone` | `stickyWeb` | 真实落点 r30 网持续 4100ms、500ms 一次伤害／.5速度；重复网、离区、失效、阿利多斯来回位移。 |
| `preventEnemyWeatherBonuses` | `cloudNine` | 普通敌人不从天气获益，首领例外；塔负面天气削弱和地图强制天气边界正确。 |
| `preventExplosionHeartCost` | `damp` | 场上任意 damp 免除自身爆炸失心，不免除正常敌人漏怪。 |
| `pullToProjectile` | `fairySingularity` | 真实投射物落点 1000ms 延迟后牵引附近敌人并爆炸；不在创建时提前命中。 |
| `ragingBlossom` | `charmedCharm` | 真实 120px 场域，只提升指定治疗系攻击承伤 ceil x1.2，同类不叠。 |
| `randomControlCondition` | `triAttack` | 共享 RNG 30% 选择烧／缓／晕，真实落点受抗性限制，重复重放保持同选择。 |
| `rangeSandDamage50` | `sandStreamMegaTyranitar`, `sandStreamTyranitar` | 沙暴每 1000 ms 伤害在每个实际覆盖范围内乘 1.5；多个扬沙来源按原事实相乘，离区不加。 |
| `sereneWaves` | `spiralDrain` | 真实 120px 场域，减速量 x1.2（速度因子 .5→.4，下限 0），非范围／非减速不改。 |
| `spreadPrimaryConditions` | `synchronySplash` | 爆炸真实命中后扩散主目标状态至周围活敌，保留持续时间、抗性与护盾规则。 |
| `storeAmnesiaPower` | `amnesia` | 50% 忘记攻击使后续 +100% 可叠且跨波保持；拖后腿尾不发射，不能每帧无冷却叠层。 |
| `teleportDestinations` | `teleport` | 战斗中无射程内目标时累计闲置，每 10000/3 ms 移到合法空位或空载平台；真实移动叠加下一次命中威力，首次命中消费；弯曲汤匙留在原位，逃脱按键减半周期。 |
| `tetherTargetsInRange` | `anchorShot` | 最多 6 条独占活链，每 500ms 真正命中；释放阈值 range+40、诅咒 range+100，目标死立即清理。 |
| `virulentReef` | `corrosionMega` | 真实 120px 场域内可中毒敌每个治疗事件加 1 层 .1%MaxHP 毒，含满血回复且不重复算同事件。 |
| `waveStartHeartCost` | `shellSmash` | 开波且生命大于 1 时调用普通伤害 1，该波威力乘 1.5、间隔乘 0.75；结实可减到 0 但仍激活，不能扣死或跨波遗留。 |
| `windDisplacement` | `bleakwindStorm` | 振幅 32／空间波长 95、敌人 300ms 命中冷却、推 15px／50px每秒；不瞬移进出口。 |

地图 35 的 16 个棋子另行覆盖：原 24px 格左上角归一为中心，部署足迹在 50ms 后统一激活／失活，活动数参与机关效果；不把五种战斗场域误当棋盘棋子。

事实仍不明确的参数保留未覆盖标记；`SupportedWorldHooks()` 只能列入已有真实行为 E2E 的 hook。普通线段、扇形、穿透、反弹和基础 Burst 碰撞由核心负责，世界模块不另建一套碰撞规则。

命令：`. scripts/env.sh && go test -race -count=1 ./games/pokepath/tests -run '^TestNativeWorld'`。每个受控场景保留初始 seed、事实、动作与观测产物到 `.local/pokepath/native-world/`。

## 当前验收记录

`native_world_abilities_e2e_test.go` 包含 49 个主测试，另含场域、树果和存档破坏的表驱动子场景。测试通过真实 `Apply`、固定步 `Advance` 和普通命中结算，检查实际伤害、状态、轨迹、共享生命与存档恢复。受控队伍、敌人和空间边界仅用于隔离能力，不作为正式难度通关证据。

初始审计的 62 个能力 ID、50 个唯一 hook 中，49 个 hook 已有实效测试。`amnesia` 没有任何目录物种引用，正常 API 无法抵达；计时、叠层和首次命中消费代码已写入，但 `storeAmnesiaPower` 未列入支持清单，也不计为验收通过。新增的 `followingNightmareZone` 另有影区跟随、延迟伤害和梦魇测试。状态扩散沿用 `projectile_links_e2e_test.go` 的共享命中 E2E。

可重复执行的记录保存在 `.local/pokepath/native-world/`：`coverage.json` 为能力与测试清单，`world-race.jsonl` 为 `-race` 测试结果，`lint.json` 为限定检查结果，各命名 JSON 为对应受控场景的规则事实和最终投影。`coverage.go` 可重新生成覆盖清单。记录只证明本世界能力范围，不代表 UI、浏览器性能、全部道具或整款游戏已完成验收。
