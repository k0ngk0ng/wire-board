# 锁妖塔前半验收

本段使用第 15 段实际入塔终点，不修改原生存档、角色属性、碰撞、事件或脚本。场景和事件编号仅用于测试路线，不进入生产托管策略。开发加血结果不增加正式原版难度完成数。

## 实现前失败场景

1. 入塔后跳过镇狱明王幻影，或错误提前触发塔底灵儿重逢和真正的镇狱明王战。
2. 普通移动不能通过原版楼梯、机关或铜门；测试通过改坐标、关碰撞或改脚本绕过阻挡。
3. 七星剑提示与实际库存不一致，亡魂战没有正常结算，姜婉儿对白或离开被跳过。
4. 七星剑取得提示中、战斗选招中或姜婉儿对白中保存后丢失续行位置、重新开战或重复发剑。
5. 天鬼皇坛子的「是／否」光标未恢复，点击拒绝仍开坛，或同一输入因恢复重复确认。
6. 天鬼皇战和报恩对白不能继续；铜门未执行原版破门演出，保存后门重新关闭或奖励重复结算。
7. 未问沉思鬼就允许带走书中仙；带走后没有天书，重复搜索再次发放，或沉思鬼仍堵路。
8. 沉思鬼解答前后、书中仙跳入行囊和机关动画期间续档改变公开画面、原版事件状态或继续位置。
9. 战斗或场景切换丢失逍遥／月如队伍、手动控制权、现金、物品、随机序列；离线 24 小时仍推进，旧输入可修改恢复局面。
10. 手机恢复后仍需虚拟按钮，点击画面不能读完对白，触控移动不释放，或 PC／手机画布被控制面板缩小。
11. 调试检查点续跑被计作整段完成；没有保留可重跑命令、原档摘要、关键截图和结果。
12. 主线路过的可选宝箱被误标为全部收集，或开发加血分支被计入原版体力隐藏物品数量。

## 断言与边界

使用普通方向、搜索、菜单确认和原版战斗接口。每次保存冻结规则、离线 24 小时、创建全新运行器并恢复，逐项比较公开局面并拒绝旧代次输入。七星剑和天书使用原版道具菜单核对实际数量，取得后各重复搜索 3 次。主线终点为第 146 场景的塔底入口，灵儿重逢另属第 17 段。

天鬼皇初战按原版脚本结束。对象 529 的战斗脚本从 42208 开始，经 42232 的「等等～！」对白，在 42235 使用 `0x0089` 结束战斗，不发普通胜利现金；随后继续说理、报恩和铜门事件。验收须保留该结果和选招中的续档，不能把它计成普通战胜或补发奖励。通用选择框使用原版菜单类型 12，测试须按实际条目选择，不能只放行该类型后一直等待场景状态。

书中仙的路线曾在 `976, 696` 与 `992, 688` 往返。测试规划器每步刷新碰撞，忘记上一步公开视区中的阻挡，重新使用粗粒度静态瓦片后选择了相反方向。本段显式开启 `rememberCollision`，保留已观察到的公开碰撞及触发范围，持续用当前视区的原版答案更新或清除。原版触碰怪物同时占用碰撞格时，允许测试规划器向其公开触发范围接近，由原版触发战斗、释放道路；不能把可战斗的守路怪物当作永久墙而在死路往返。默认路线规划行为不变。该选项、缓存和诊断只供测试使用，不写入游戏地图或生产托管策略。

完整重跑必须从 `campaign-shu-tower-state-1.json` 开始；调试入口须保留 `resumedTestRoute: true`，不能替代整段验收。真实页面使用本次重跑的原版书中仙对白存档，另外验证本人续玩、陌生账号拒绝、画面点按、触控／键盘和最大比例画布。

生产战斗策略的定向回归从同次第 9 场遭遇战的真实存档开始，只记录公开战斗状态和普通输入。失败包括取消／打开菜单形成循环、对空位反复选敌、召唤后目标漂移、恢复后策略记忆失效和战斗长时间不结束。最初的 2,000 次计时诊断尚未结束；主线随后观察到该战正常获胜，公开轨迹显示敌人持续互相增援，不能据此声称引擎或策略死锁。定向完整回归预算为 6,000 次，须保留原诊断及最终输入轨迹、公开画面和原始快照；不得改写敌人体力、队伍或战斗结果。

天书取得提示另从实际「跳入行囊」存档重放 40 次计时，观察到原版黑字提示。公开中文绘制字形提取结果为「得到天书」，不包含脚本文本中的 ASCII 引号；剧情断言按实际中文绘制结果匹配，实际库存仍须独立核对。不能因文本提取方式而修改原版提示或补发物品。

第 157 场景使用原版第 156 张地图。普通路线先触发 2534、2527、2541 三处机关，再离开本层；对应的 10 个桥段须实际到达开启帧 0。机关按下和桥段开启时分别保存，恢复后继续原版自动脚本。原始瓦片本身阻挡时，规划器不能因为附近有怪物触发范围就将其视为可走格。

## 重跑入口

先恢复第 15 段实际终点、相同的场景波动核心和原版资源，再执行：

```sh
. scripts/env.sh
PAL_CAMPAIGN_VARIANT=scene-wave PAL_DEVELOPMENT_HEALTH=9999 go test -v ./games/pal/tests -run '^TestOriginalTowerSwordWanerGhostSageRiddleAndSave$' -count=1 -timeout=40m
PAL_CAMPAIGN_VARIANT=scene-wave PAL_DEVELOPMENT_HEALTH=9999 PAL_TOWER_DECLINE=1 go test -v ./games/pal/tests -run '^TestOriginalTowerGhostRescueDeclineAndSavedChoice$' -count=1 -timeout=5m
PAL_CAMPAIGN_VARIANT=scene-wave PAL_DEVELOPMENT_HEALTH=9999 PAL_TOWER_BOOK_NOTICE=1 go test -v ./games/pal/tests -run '^TestOriginalTowerBookNoticeFromSavedBag$' -count=1 -timeout=3m
PAL_CAMPAIGN_VARIANT=scene-wave PAL_DEVELOPMENT_HEALTH=9999 PAL_TOWER_BATTLE_DIAGNOSTIC=1 go test -v ./games/pal/tests -run '^TestOriginalTowerSavedEncounterProductionBot$' -count=1 -timeout=8m
```

普通遭遇战回归绑定保留的 `tower-encounter-bot-source-state-1.json`，SHA-256 为 `12b7239709755d5fd03a8a1267a40cf6e81b6dcfd71ea4950767dc68f0510bbd`；主线重跑不覆盖该原档。实际使用 2,951 次计时后正常获胜，现金由 37,819 变为 38,775，没有修改战斗策略或原生规则。

调试可设置 `PAL_TOWER_UPPER_CHECKPOINT=step-N` 或 `failure`；可选 `PAL_TOWER_WALK_TRACE=1` 记录测试规划方向。分段调试到达塔底的报告必须保持 `resumedTestRoute: true`，不能计入剧情完成数。

真实页面使用两个终端，两边均设置场景波动候选：

```sh
. scripts/env.sh
PAL_BOARD_BROWSER=1 PAL_BOARD_ENTRY=solo-tower-sage PAL_BOARD_VARIANT=scene-wave go test -v ./internal/server -run '^TestPALBoardBrowserFixture$' -count=1 -timeout=20m
```

```sh
. scripts/env.sh
PAL_BOARD_VARIANT=scene-wave python3 -B games/pal/tests/solo-board-browser.py --tower-sage
```
