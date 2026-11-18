# 十年前余杭与水灵珠验收

第 21 段从第 20 段实际终点开始，根档为 `campaign-temple-past-state-1.json`，SHA-256 为 `1ce17774473cf98aad2810bcc644ba1d1d0035893c9163b715b0623e79ebe566`。范围包含山神庙的幼年灵儿、姥姥与凤纹手绢、港口船商及灵月宫主、治伤与告别、秀兰线索、童年逍遥的玩具箱、木剑交换水灵珠、返回现世及阿奴交付凤凰蛋壳。终点为现世女娲神殿，圣药交付属于下一段。

原版只读核查位于 `.local/pal/verification/old-yuhang-source-review.json`。静态事件编号、脚本与坐标仅用于测试规划，实际执行普通移动、搜索、阅读和原版道具菜单。生产托管不得使用这些未公开信息。原版 `39831` 要求面对船商才消费手绢；`34918` 消费木剑，`34927` 才发放水灵珠；返回现世后 `30511` 才交付蛋壳。

## 实现前失败场景

1. 未从第 20 段实际终点开始，或将调试续跑标为完整重跑。
2. 未见幼年灵儿、未经历火法术及姥姥求助，便提前取得手绢或水灵珠。
3. 手绢重复领取、在错误场景或朝向使用时被消费，或没有实际在船商前通过原版道具菜单使用。
4. 手绢选中、消费、船商现身或场景切换中保存丢失续行，恢复后重复消费或重复发奖。
5. 灵月宫主治伤、灵儿拜师、港口告别或李大娘找人的剧情缺失。
6. 不与秀兰交谈便显示玩具箱，遗漏原版可见性条件；秀兰移动后仍只搜索初始位置。
7. 玩具箱、童年逍遥、木剑约定被跳过；将现金、真剑或无道具直接换成水灵珠。
8. 读取道具提示时误认为已经发奖，或木剑未实际减少一把，水灵珠和蛋壳被提前发放。
9. 木剑交换前／交换中／返回现世后保存丢失阅读位置或随机状态；重复确认导致道具重复。
10. 山道的原版战斗被跳过，败北或逃跑计作正常胜利，金钱结算重复。
11. 幼年灵儿、姥姥或灵月宫主错误加入战斗队伍；现世阿奴归队顺序改变或失去本人控制。
12. 已有火灵珠、风灵珠和麒麟角无故消失或变化；为路线通过改写原版碰撞、地图、脚本、角色或存档。
13. 保存清掉尚未消费的下一步输入，测试继续使用旧动作代次；离线一天后仍有输入回放。
14. PC／手机不能通过画面点击推进交换剧情、使用原版菜单或完成个人续档；虚拟按键或界面挤小游戏区域。
15. 用不同次运行的浏览器入口、陈旧报告或不匹配的核心作为验收证据。
16. 姥姥和幼年灵儿相邻，宽搜索范围先命中灵儿而反复对话。测试须从姥姥正前方普通接近，再按原版搜索顺序交谈，不改角色位置或触发方式。

完整重跑、关键 PC／手机页面及产物校验通过后，才增加开发进度。HP9999 仅用于开发测试，不增加原版难度 4/24；旧余杭的可选收集另行验收。

```sh
. scripts/env.sh
PAL_CAMPAIGN_VARIANT=scene-wave PAL_DEVELOPMENT_HEALTH=9999 PAL_OLD_YUHANG=1 go test -v ./games/pal/tests -run '^TestOriginalOldYuhangHandkerchiefWoodenSwordWaterOrbAndSave$' -count=1 -timeout=60m
```

当前从上述实际终点完整重跑通过，`resumedTestRoute: false`：71 次阅读、8 场正常胜利、88 次新运行器续档及 14 个路线节点。手绢取得 1 条，错误地点使用后保留，船商前正常使用后消费；木剑从 1 把减到 0，把水灵珠带回现世后库存为 1；阿奴按原版交付蛋壳 1 件并恢复 `[0,4]` 队伍。火灵珠、风灵珠与麒麟角各保持 1 件。返回后重复确认 3 次，库存不变。

终点为第 204 场景、第 175 张地图，现金 69,200 文，所有队员归本人操作。终点存档 SHA-256 为 `b191bb4254cfd8bbb9a51459d5b9da8c3a78b7aee67dca5669f0c0268f0e9ce3`。原版源码 60 个文件、资源 104 个文件与开发核心摘要未变化。摘要为 `.local/pal/verification/old-yuhang-stage.json`，归档为 `development-health-stage-21/`。

同次手绢识别入口为 `old-yuhang-cloth-recognized-reading-state-1.json`，SHA-256 为 `d266d35bc19e8bf19ea7504777d761a5d4e9a407608c7c95adbee4b6fec74142`。交换入口为 `old-yuhang-exchange-reading-state-1.json`，SHA-256 为 `9a4a632a7c65da6bcade044df3d56934a8b2cf8b6d8e2ca9f162840ccb4c04be`。两组真实 PC／手机共 17 项检查、39 次手机阅读通过，截图已人工检查；交换组在返回现世的对白中另保存、恢复并比较完整公开边界。PC／手机画布保持 1280×800／320×200，无虚拟操作按键。

```sh
. scripts/env.sh
PAL_BOARD_BROWSER=1 PAL_BOARD_ENTRY=solo-lingyue PAL_BOARD_VARIANT=scene-wave go test -v ./internal/server -run '^TestPALBoardBrowserFixture$' -count=1 -timeout=20m
```

另一终端执行：

```sh
. scripts/env.sh
PAL_BOARD_VARIANT=scene-wave python3 -B games/pal/tests/solo-board-browser.py --lingyue
```

交换入口将 `solo-lingyue` 和 `--lingyue` 分别换为 `solo-water-orb` 和 `--water-orb`。入口在原版「好～交换」对白，保留 RGM 85，随后经普通画面点击执行水灵珠提示和现世返回；不把计时物品提示错误当作需要点击的对白页。
