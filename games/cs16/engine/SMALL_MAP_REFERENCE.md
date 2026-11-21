# 冰雪世界与血战回廊规则接入

本轮新增 `fy_iceworld` 与 `cs_bloodstrike`。地图性质来自原 BSP 实体清点，不由 `fy_`、`cs_` 文件名前缀推导。原图、NAV 和转换身份由资源记录保留；公开 ReGameDLL_CS 固定参考用于兼容实现，不冒充原版运行证据。

## 原图事实

| 项目 | fy_iceworld | cs_bloodstrike |
| --- | --- | --- |
| 地图模式 | 回合歼灭 | 回合歼灭 |
| T／CT 出生 | 各 12 | 各 10 |
| 爆破、人质或其他目标 | 无 | 无 |
| 原地面武器 | 24 个 armoury_entity，12 类枪械各 2 | 无 |
| 显式买区 | 一个双方可用的中央买区 | 无，采用默认本队出生购买范围 |
| 出生装备覆写实体 | 无 | 无 |

两图均保留原有 800 初始资金、T 的 Glock 与 CT 的 USP、默认备用弹和小刀。死亡后等待下一回合，没有即时复活。存活者装备按既有规则跨局保留。

## 规则兼容依据

参考提交为 `1f16a7f25c14225931f8df1abc51f68d18ded839`。

- `multiplay_gamerules.cpp` 的 `CheckWinConditions` 保留双方歼灭判断；`CheckRoundTimeExpired` 在没有目标且默认 `mp_roundover=0` 时不凭空判胜或和局。本实现以 `mode="elimination"` 表示，实战 `PhaseEndsAt=0`，冻结与局末计时保持原机制。
- `player.cpp` 的 `HandleSignals`、`OLD_CheckBuyZone` 与 `mapinfo.cpp` 的默认购买策略区分显式买区和出生范围。冰雪世界不回退出生购买，也不按文件名全图禁购；血战回廊没有显式买区，继续使用既有默认出生购买规则。
- `weapons.cpp` 的 `CArmoury::Spawn`、`Restart`、`ArmouryTouch` 和 `armouryItemInfo` 规定触碰、数量、原物品与额外备用弹。原实体坐标保存在 `WorldWeapons`，实际枪通过原世界 trace 落地，不能悬浮在 BSP 原始放置高度。
- `util.h:86-87` 的实体蹲姿边界为 `(-16,-16,-18)` 至 `(16,16,32)`，用于 armoury 触碰；它与 PM 世界蹲姿预扩张 hull 的高度约定分开，未修改玩家运动 hull。

冰雪世界物品为 MP5、TMP、P90、MAC-10、AK-47、SG552、M4A1、AUG、AWP、M3、XM1014、M249 各两件。枪械弹匣全满，额外备用弹按源表发放并累加到共享弹药池，不能覆盖玩家原有备用弹。枪械购买阵营限制不用于拾取地图枪。

## 状态与电脑

原图枪复用公开 `Drops`，用 `SpawnID`、`Remaining`、`Yaw` 保留来源、数量与方向。消耗状态参与存储与恢复，每个新回合从原地图实体重建；普通丢弃物仍独立限制为 64 件，不淘汰原图物品。

电脑无目标时只依据公开原图出生区域和可达导航巡逻，缺主枪时通过普通移动触碰原枪。落点与 NAV 中心不重合时，仅在原人物 hull 证明可直达的最后一段直接行走。未读取墙后敌人的精确位置，没有直接写入库存或比赛结果。

本轮真实歼灭对局暴露旧电脑在冷却中首次装填按下被拒后持续按住的问题，以及空主枪不换有弹副枪的问题。修复仅调整电脑的合法按下／松开与选枪输入，不修改武器装填、弹药、伤害或人类操作规则。

## 验证

先行失败清单见 `SMALL_MAP_FAILURES.md`、`MAP_WEAPONS_FAILURES.md` 与电脑装备失败清单。`small_maps_e2e_test.go`、`map_weapons_e2e_test.go`、`bot_inventory_e2e_test.go` 和服务端 `small_maps_test.go` 分别验证原图事实、回合／拾取、合法电脑操作与规则身份。

`.local/cs16/verification/small-maps/second-natural.jsonl` 的定向结果包含两图十席、两局自然比赛：冰雪世界在帧 16338 完成，有 19 次拾取、15 次击杀、278 次射击；血战回廊在帧 4428 完成，有 15 次击杀、285 次射击。所有局末由真实歼灭产生，没有通过计时结算。边界夹具另行设置初始场景，不作为自然对局证据。

冻结源码、检查范围和结果摘要见 `.local/cs16/verification/small-maps/source-frozen.json`。WASM、真实 HTTP／WebSocket、浏览器和发布验收由各自入口提供，不能以本层规则模拟替代。
