# 第五关雪地敌人与 Boss

已恢复日版雪地的专属类型 `$10..$16`：冰雷发生器、冰雷、坦克、冰管连接件、母舰、子飞碟及下落炸弹。规则位于 `enemies_snow*.go`，使用语义实体、现有武器／碰撞／图形队列及世界状态接口，不执行 ROM 或模拟 CPU。

输入绑定日版 SHA-256 `6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe`。真实分派表为固定 `$E62D`；程序窗口是 bank 0/1 与固定 bank 14/15。完整 [主 REA 证据](evidence/enemies-snow-rea.json.gz)、[辅助入口证据](evidence/enemies-snow-helpers-rea.json.gz)、[原版表数据](evidence/enemies-snow-tables.json)及 [验收记录](evidence/enemies-snow-verification.json)已保存。

## 实现前失败场景

- 使用美版地址、声码、停留时长或飞碟落点。
- 把坦克的高位 X 当成小数，或把背景坦克当普通移动精灵。
- 延后同槽内的滚屏、调色暂停、颜色索引和淡入变更。
- 新生实体分配次序、源槽重用、全池失败和同帧后续槽执行不一致。
- 忽略队列拒绝，提前完成死亡、地形变化或 Boss 退出。
- 将范围爆炸误当无碰撞视觉，或丢失声音／画面／伤害的阶段次序。
- 正常包构建未包含代码，却仅靠手动指定文件的测试声明完成。

先记录原版输入／输出及失败场景，建立 red 测试，再编写对应规则。整个敌人阶段的第 5 关集成也使用已存在的真实原版夹具验证。

## 日版行为与接入

| 类型 | 实现入口 | 关键行为 |
| --- | --- | --- |
| `$10` | `StepIceGenerator` | 滚动到 X<200 后启动；每 128 tick 尝试分配冰雷，保留全池失败和源槽被重用的语义 |
| `$11` | `StepIceGrenade` | 日版声码 `$30/$3D`；重力、地形碰撞后进入可伤人的 `ExplosionMortarStart`，再经过爆炸及移除阶段 |
| `$12` | `StepSnowTank` | 背景坦克与 16 位扩展 X；移动、停止射击、继续移动、分块毁坏和离屏清理 |
| `$13` | `StepIceJoint` | `JointScroll` 非零时，真人造成滚动就只减 X 一像素；不能按滚动总量减多次 |
| `$14` | `StepSnowCarrier` | 出现／淡入、舱门、子实体生成、消失重定位、清场、爆炸、出口图块和延时 |
| `$15` | `StepMiniUFO` | 横向展开、下降、底部横移、命中爆炸；日版阈值为 Y≥`$A9`，调整至 `$AA` |
| `$16` | `StepSnowBomb` | 原版重力及高度阈值，之后进入共用爆炸链 |

坦克停止时长来自日版 `$A50B` 的 8 字节表：`00 00 00 40 F4 F4 F4 34`，索引为 `(Attributes & 1) × 4 + WeaponStrength`。不能用美版的两项 `00/F8`。坦克移动声为 `$36`，坦克／母舰毁坏声为 `$75`。

坦克使用 `Enemy.FractionX` 作为有符号的屏幕高位，而不是小数累积。高位进入／离开可视区时按原版翻转碰撞状态。额外滚屏通过现有 `EnemyStepContext.TankScroll` 指针更新，调色暂停和淡入使用世界唯一的 `PaletteOutput`，颜色索引使用现有 `PaletteState`。冰管共享 `EnemySystem.JointScroll`，没有复制一份世界标志。

坦克使用的动态 palette ID 为 `$3F/$41/$5F/$60/$61`，已由公共资源导出器从日版原表补齐。每次真实 `$CEE3` 调色刷新都调用现有 `RefreshBackgroundPalette(16)`，并传播其 `$06` 兼容工作值；没有用虚构的回调结果绕过资源缺失。

图块变更通过真实 `GraphicsQueue` 回调，包含预算拒绝。母舰的绘制助手会在请求被拒绝时仍推进某些绘制计数；死亡爆炸的拒绝分支则恢复先前坐标。两种行为分别保留，没有统一改成「拒绝就完全不动」。

母舰在死亡链开始时独立发出 `BossAudioReset`；最终出口阶段才发出 `BossBeginDefeat` 和 `BossExitReady{Delay:48}`，随后移除实体。它不直接写对局胜负或平台积分。原名为「two explosion」的共用助手实际只分配一个 type 2 效果实体，也按真实日版执行保留。

## 验证结果

本子系统的正常包路径限定 race 已通过，覆盖 19,090 次原版实体调用；另通过 51 个第 5 关完整敌人阶段输入。

| 证据 | 范围 |
| --- | --- |
| 自然按键轨迹 | 4,501 次冰雷发生器／冰雷调用；起点是已声明的受控第 5 关初态，之后只使用双人按键 |
| 小实体边界 | 104 次原版调用，含分配失败、落水／离屏、joint 滚动和飞碟边界 |
| 坦克／母舰边界 | 182 次原版调用，含真实 palette 刷新、图形预算拒绝、瞄准借位、子槽分配和最终延时 |
| 移动坦克连续链 | 9,801 次雪地实体调用，其中坦克自身 1,217 次；持续移动、停留、射击及离屏清理 |
| 被毁坦克连续链 | 4,337 次雪地实体调用，其中坦克自身 499 次；毁坏图块、爆炸、滚屏和 palette 恢复 |
| 母舰死亡连续链 | 95 次目标调用，保留音频重置、分块爆炸、出口变化和最终延时 |
| 母舰真实队列争用 | 70 次目标调用；首帧由其他武器盒真正占满队列，发生 1 次拒绝并验证坐标回退 |

连续链仅在起点设置实体场景及玩家 B 护盾，之后没有逐帧补写实体、计时器、队列或结果。最初的母舰死亡链未产生预期的队列争用，因此保留它并额外建立了一个从爆炸阶段开始的最小争用起点；没有降低断言或改写原版输出。四条链都完成到原版实体移除，目标调用之间的状态连续一致。

自然、受控与连续录制均重复执行，调用记录及完整 RAM 摘要一致。范围必须分别理解：自然按键路线在首屏全灭，未自然到达坦克或母舰；坦克／Boss 连续链为明确的受控初态，不代表已经自然通过第 5 关或八关战役。

早期录制的 `world_fields` 中，地址 `$0B` 曾使用诊断标签 `generationBias`；原始记录未重写，但它不参与生产兼容工作值处理。后续复杂／连续夹具中的 `work06` 才是实际 `$06`，测试单独比较其结果。

## 复查

```sh
source scripts/env.sh
go test -race ./games/contra/engine -run '^TestSnow|^TestOriginalAllStageEnemyPhaseDispatch$/stage5$' -count=1 -v
```

静态数据与入口可重建：

```sh
python3 -B scripts/contra/enemies_snow_prepare.py
node scripts/contra/enemies_rea.mjs .local/contra-enemies-snow/spec.json
python3 -B scripts/contra/enemies_snow_tables.py
```

原版录制脚本为 `enemies_snow_capture.py`、`enemies_snow_cases.py`、`enemies_snow_complex_cases.py`、`enemies_snow_sequences.py`，均接受新的运行标签，拒绝覆盖既有记录。需要 Pillow 时使用仓库现有 `.local/board-art-env/bin/python -B`。初态来自 `.local/contra-campaign/states/stage-5-v2.fcs`，源摘要随夹具保存。

只修改雪地实现、相关测试／复查脚本及证据。共享 properties、出生与主分派由父任务维护，其他关卡文件未改动；没有注册、提交或发布游戏。
