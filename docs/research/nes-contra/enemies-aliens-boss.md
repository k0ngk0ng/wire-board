# 日版最终关守卫与心脏 Boss

已恢复第 8 关的守卫 `0x10` 和心脏 `0x16`，分别提供 `StepAlienGuardian`、`StepAlienHeart`。独立实现通过 617 次原版边界调用，以及 2,200 次连续原版敌人阶段对照。守卫是关中敌人，心脏是最终 Boss；二者的清场延迟不直接构成平台通关结果。

输入为固定日版 ROM SHA-256 `6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe`。复用已匹配身份的 REA 分析，并在独立会话补充绘制与清墙助手；35 个相关入口、原表字节及完整证据包见 [REA 索引](evidence/enemies-aliens-boss-index.json)。自动反汇编会把 `$BD59` 开始的数据表解释成指令，恢复时使用独立绑定的原版表数据，不执行这些误识别的「代码」。

## 规则与平台边界

| 内容 | 日版行为 |
| --- | --- |
| HP 初始化 | 武器强度和周目各先按字节左移 4 位，再加 `$37`；进位或达到上限时设为 `$A0` |
| 守卫开闭 | 顶部两个超图块与底部一个超图块分别保存重试标志；口型计时与绘制接收状态各自推进 |
| 守卫生成 | 倒计时最后两次生成 `0x11` 胎体；武器强度至少为 `3` 时提前再生成一次，使用共享 16 槽池 |
| 守卫死亡 | 日版请求声音 `$75`，逐次生成爆炸，然后清除身体、墙面及碰撞；没有心脏的 Boss 标志与音频重置 |
| 心脏跳动 | 等待长度由当前 HP 右移一位决定，最少为 `1`；上下半部图形分别提交，失败按原版路径重试 |
| 心脏死亡 | 将调色循环索引 1／3 写为 `$4B`，呈现节奏 `$5D` 写为 `3`；先重置音频，再请求 `$7B` 并标记 Boss 死亡 |
| 公共爆炸 | `$B6D7` 按 5 tick 间隔推进，原表第 0 项不在后续序列中使用；申请不到槽位也按原版推进计时 |
| 公共清场 | `$BD27` 扫描槽 15 到 1，按残留类型写入各自销毁阶段；即使原槽已不活动也会写入，槽 0 不参与 |
| 收尾 | 最后请求 `BossExitReady`，延迟 `$A0`，随后移除守卫／心脏；World／Campaign 决定之后的场景推进 |

`AlienBossContext` 借用 World 的 `AutoScroll`、`PresentationPace` 和碰撞写入所消费的来源值 `CollisionAddress`。调色循环索引使用 `EnemyStepContext.Scene.Header`。`OnBossControl` 同步处理 `BossAudioReset`、`BossBeginDefeat`、`BossExitReady`，不能把心脏声音统一替换成其他关的 `$77`。

外层敌人阶段收到 `BossAudioReset` 时，需要丢弃前面槽位已积累的声音请求；心脏方法只清理自己的临时请求。原版这里只改调色字段，不立即加载所有颜色；正式 World 仍按自己的原版呈现阶段更新调色输出。

## 绘制拒绝后的碰撞来源

守卫一次绘制会依次申请右、左两块。无论第一块是否成功，都会尝试第二块，并以第二次结果更新重试标志。每次调用之前保存的相对 X 也会成为本阶段的生成偏置，不能只在成功绘制时更新。

清墙的 `$B686` 不以绘制成功为前提。图形队列拒绝时，它仍消费先前留下的值，因此该输入不一定来自最近一次图形定位。已确认的生产者见 [来源清单](evidence/enemies-aliens-boss-locator-sources.json)：

- 完整地形查询留下低字节 `TerrainSample.Column`、高字节 `TerrainSample.SourceIndex`，包括 Y 越出有效显示区后返回空地形的查询。
- 敌人探测在 `Y+dy` 溢出而提前返回时，只把高字节改为探测 X，低字节保持。
- 接受的图形请求留下 nametable 定位地址；拒绝不会执行这个定位过程。
- 公共碰撞清除 `$E989` 在计算索引时将地址低字节左移 5 位。守卫清相邻块前恢复原低字节并加 `4`，最后保留第二次清除产生的值。

完整敌人阶段必须按真实顺序传递这些明确的计算结果，并从角色／武器／滚屏阶段接收入口值。不能把 `$12/$13` 默认初始化为零、仅当作上一次图形地址，也不能通过通用 RAM 模拟掩盖遗漏的生产者。本项连续序列验证守卫自身、胎体和爆炸的闭环；随后补充的 [74 个混合阶段场景](enemies-aliens-locator.md) 已验证普通查询、高 Y 查询、溢出探测与有效图形结果被后续查询覆盖的接力，其他完整世界来源继续由上层验收。

## 原版验证

| 数据 | 覆盖 | 结果 |
| --- | --- | --- |
| 617 次边界调用 | 守卫 13 个阶段、心脏 8 个阶段、满池／自槽复用、坐标回绕、绘制拒绝、碰撞与清场 | 全 16 槽、图形、碰撞、声音、共享字段及生成偏置一致 |
| 守卫连续序列 | 1,200 次 `$E54F`，3,063 次实体调用，19 次图形拒绝 | 独立敌人池、地形及共享字段持续推进，一致 |
| 心脏连续序列 | 1,000 次 `$E54F`，1,179 次实体调用，22 次图形拒绝 | 一次音频重置、调色字段、清场与最终 `$A0/$00` 延迟一致 |
| 三份独立重录 | 输入、完整 RAM、原版调用记录 | 逐字节一致 |

连续序列在开始时设置一个守卫或心脏，并分别在第 700／600 tick 明确触发销毁阶段。守卫生成的胎体使用实际独立胎体规则，爆炸使用公共规则，均按原版倒序槽位执行；测试从不把原版 after 记录回填为下一帧状态。玩家具备出生保护，玩家弹丸为空，因此不把这些序列称为自然击杀或完整游戏 E2E。

原版起点来自受控第 8 关初始化存档，来历见 [关卡初态记录](evidence/campaign-controlled-entries.json)。整体世界、自然最终战、关间／结尾演出、双人网络和平台积分结算不在本项验收范围。

先记录失败场景并建立原版 fixture 与缺少实现的 red，再编写规则。限定 race 检查、源码摘要、重复录制及日志见 [验证记录](evidence/enemies-aliens-boss-verification.json)。

## 重复执行

```sh
python3 -B scripts/contra/enemies_aliens_boss_prepare.py
node scripts/contra/enemies_rea.mjs .local/contra-enemies-aliens-boss/main-spec.json
node scripts/contra/enemies_rea.mjs .local/contra-enemies-aliens-boss/helpers-spec.json
node scripts/contra/enemies_rea.mjs .local/contra-enemies-aliens-boss/draw-spec.json
.local/board-art-env/bin/python -B scripts/contra/enemies_aliens_boss_capture.py --name aliens-boss-recheck
.local/board-art-env/bin/python -B scripts/contra/enemies_aliens_boss_sequence.py --name aliens-boss-guardian-recheck --boss guardian
.local/board-art-env/bin/python -B scripts/contra/enemies_aliens_boss_sequence.py --name aliens-boss-heart-recheck --boss heart
python3 -B scripts/contra/enemies_aliens_boss_verify.py
```

每次原版录制使用新名称。最终证据和主录制保留，清理范围仅限本项缓存、重复捕获和废弃中间序列，见 [清理记录](evidence/enemies-aliens-boss-cleanup.json)。没有覆盖一般异形敌人的导出表，没有改写用户 ROM，也没有注册、上传、提交或发布游戏。
