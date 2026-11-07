# 日版瀑布敌人、潜水兵与龙形 Boss

已将第 3 关的移动平台、火焰、落石洞、落石、Boss 嘴部和两条龙臂恢复为独立 Go 状态机，并恢复第 3／5 关共用的潜水兵。规则使用共享敌人池、碰撞窗口与图形队列，不执行 ROM 或模拟 CPU。当前证据覆盖独立敌人阶段；完整 World、自然 Boss 战、八关通关和联机验收仍由上层集成完成。

本记录绑定日版 ROM SHA-256 `6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe`。REA 6.5.0／Ghidra 12.1.4 采用独立 stdio 会话，分析 bank 0／1 与固定 bank 14／15；完整证据、函数入口、汇编限制及原版表字节见 [REA 索引](evidence/enemies-waterfall-index.json)。早期探索中的 `$9AEA`、`$EAC0` 不是已验证入口，未收入交付证据。

## 范围与原版入口

| 类型 | 独立接口 | 日版入口／关键行为 |
| --- | --- | --- |
| `0x10` 移动平台 | `StepWaterfallHazard` | `$9804/$9837`；四组横向定点速度和边界，先移动再检查反向 |
| `0x11` 移动火焰 | 同上 | `$985B`；沿用平台运动，按原版帧 bit 3 改变翻转 |
| `0x12` 落石洞 | 同上 | `$9878/$987E/$9886`；等待后从共享池生成 `0x13` |
| `0x13` 落石 | 同上 | `$98A4/$98A9/$98E9`；晃动、开放碰撞、旋转、落地反弹和公共爆炸 |
| `0x14` Boss 嘴部 | `StepDragonMouth` | `$9945..$9A7C`；开闭、三向火球、独立剩余 HP、死亡背景序列 |
| `0x15` 龙臂 | `StepDragonArm` | `$9AB7..$9F0F`；肩部生成四个子球、伸展、五种攻击模式、链式位置与销毁 |
| 公共 `0x0C` 潜水兵 | `StepScuba` | 固定 `$F0AD/$F0B2/$F0E9`；浮潜、射击、隐藏与公共爆炸 |

属性初始化继续复用公共 `EnemySystem.Initialize`。潜水兵的迫击炮弹是共享池内的 `0x0B`，生成位置为源实体 X `+5`、Y `−24`；后续弹道由公共对应类型实现负责。

## 必须保留的语义

- 落石只对原版查询的 carry 作反弹判断。普通碰撞码 `1` 设置该 carry，实体墙码 `$80` 不设置；`Y+8` 溢出时，查询提前返回仍保留 carry。反弹使用日版声音 `$0D`、速度 `$FEC0`，然后在同一调用中加重力 `$10`。
- 洞口离屏后，本槽可能成为最高空槽并被自己生成的落石复用。原版先初始化新实体，再读取源位置，因此同槽复用时源位置已被清理。不能提前缓存位置，也不能在离屏后跳过调用余下代码。
- 嘴部绘制分两次请求图形队列。上半部可以成功、下半部可以失败；失败后以原版延迟重试，不把它改成一次原子绘制。验证使用第 3 关真实替换图块与 `GraphicsQueue.RequestSupertile`，没有按期望结果伪造接受状态。
- 嘴部在张开时恢复保存的 HP，关闭时保存当前 HP 并改为不可正常受伤的 `$F1`。一次攻击按 `$84/$86/$88` 的顺序生成三发；两只手被毁后速度代码由 `6` 变为 `7`。
- 手部被毁时，立即增加共享 `$85`，再向肩部安排公共销毁阶段；后续槽位在同一个敌人阶段内必须读取更新后的值。龙臂的五个实体始终占用同一个 16 槽池，生成、动画与销毁不能复制成另一套私有实体。
- 最后一段嘴部死亡背景请求完成后，才发出 `OnClearDelay(0x60)` 并移除嘴部；Boss 死亡开始调用 `OnDefeated`，对应音频重置、声音 `$77`、低延迟 `$FF` 和 Boss 标志。低延迟 `$60` 的收尾不能替换成第一关的 `$30`，也不能直接发放平台通关积分。
- 龙臂链中的负值表示端点。独立代码拒绝越界或循环链，避免损坏存档使服务器挂起；没有为畸形链编造胜利。公开美版注释提到的「伸展时击毁手部」问题尚无本项的日版动态复现，不能把该注释作为已观测的日版事实。

`WaterfallBossContext` 借用真实 World 的 `DestroyedArms *uint8`，传入 `ScrollComplete`、`PaletteTimer`，并通过 `OnDefeated`／`OnClearDelay` 更新同一份战役状态。上层应在每个实体调用时重新投影可变输入，并在事务副本上执行整帧。没有增加另一份 Boss／关卡状态，也没有修改平台积分。

## 验证结果与限制

| 验证 | 范围 | 结果 |
| --- | --- | --- |
| 单次原版边界 | 234 次平台、火焰、落石及潜水兵调用；含满池、自槽复用、溢出和阶段优先级 | 全 16 槽与声音一致 |
| 原版控制器路线 | 5,563 次调用，覆盖洞口、火焰、落石和潜水兵；普通按键推进到 screen 2 | 全 16 槽与声音一致 |
| Boss 边界 | 673 次嘴部／龙臂调用，包含图形拒绝、开闭、共享手部计数和各攻击模式 | 实体、图形、碰撞、声音、共享字段和生成偏置一致 |
| 连续完整敌人阶段 | 2,400 次 `$E54F`，19,511 次实际实体调用，20 次图形拒绝 | 独立池持续推进，全程一致 |
| 独立重录 | 上述四份数据分别再次运行 | 输入、完整 RAM 和原版调用记录逐字节一致 |

连续验证只在开始时构造两肩和嘴部，之后独立系统保留自己的整个敌人池；不会用原版 after 记录回填。按已声明的外部边界，在第 1,000／1,600 tick 触发两只手的销毁阶段，在第 2,000 tick 触发嘴部销毁阶段。双方共享池生成的敌弹与爆炸也按倒序槽位真实执行，末态核对双手计数 `2`、一次音频重置、低／高结束延迟 `$60/$00` 和嘴部移除。

这不是自然击杀 Boss：边界中的销毁阶段是明确的受控输入，玩家具有出生保护且没有玩家弹丸。原版控制器路线也从已有的「受控第 3 关初始化」存档开始；该初态只修改一次关号后运行原版初始化，不代表从第一关自然抵达瀑布。相关来历见 [关卡初态记录](evidence/campaign-controlled-entries.json)。

复杂规则先建立原版 fixture 与失败场景，再编写 Go 实现。首次缺少实现的 red、限定 race 检查、源码摘要及重复录制记录保存在 [验证结果](evidence/enemies-waterfall-verification.json)。其他游戏没有纳入本项测试；并行开发中的其他测试也没有冒充本项验收。

## 重复执行

```sh
python3 -B scripts/contra/enemies_waterfall_prepare.py
node scripts/contra/enemies_rea.mjs .local/contra-enemies-waterfall/spec.json
node scripts/contra/enemies_rea.mjs .local/contra-enemies-waterfall/details-spec.json
node scripts/contra/enemies_rea.mjs .local/contra-enemies-waterfall/arm-spec.json
node scripts/contra/enemies_rea.mjs .local/contra-enemies-waterfall/arm-detail-spec.json
node scripts/contra/enemies_rea.mjs .local/contra-enemies-waterfall/scuba-spec.json
.local/board-art-env/bin/python -B scripts/contra/enemies_waterfall_capture.py --name waterfall-hazards-recheck
.local/board-art-env/bin/python -B scripts/contra/enemies_waterfall_capture.py --name waterfall-natural-recheck --natural
.local/board-art-env/bin/python -B scripts/contra/enemies_waterfall_capture.py --name waterfall-boss-recheck --boss
.local/board-art-env/bin/python -B scripts/contra/enemies_waterfall_sequence.py --name waterfall-sequence-recheck
python3 -B scripts/contra/enemies_waterfall_verify.py
```

原版录制拒绝覆盖运行名称；再次执行时更换名称。保留主录制、完整 REA 证据和原版 fixture，清理仅限本项重复捕获、失败探针及编译缓存，详见 [清理记录](evidence/enemies-waterfall-cleanup.json)。未改写用户 ROM、其他游戏或其他任务目录；未注册、上传、提交或发布游戏。
