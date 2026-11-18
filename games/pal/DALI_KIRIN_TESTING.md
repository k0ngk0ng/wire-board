# 大理与麒麟洞验收

第 19 段从第 18 段实际终点开始。根档为 `campaign-sacred-tree-state-1.json`，SHA-256 为 `830bc1cd77291a24db93a142da10f98ca2fffdf292344702d55084421a59a96d`。原版主线包含洞口守卫、阿奴独自见族长、军情会议、与逍遥会合、麒麟洞、麒麟战斗、两件圣物及女娲神殿指引。返回大理后停止，女娲神殿与十年前属于下一段。

原版只读核查保存于 `.local/pal/verification/dali-kirin-source-review.json`。坐标、事件及脚本编号只供测试路线使用，不进入生产托管。所有输入沿用正常移动、搜索、阅读及原版战斗菜单，不修改原生核心、事件、碰撞、角色、道具或结算。

## 实现前失败场景

1. 从独立麒麟战斗夹具或调试检查点拼成整段通过，未从实际第 18 段终点重跑。
2. 初到大理时绕过洞口守卫；军情会议尚未发生，守卫或洞口却提前开放。
3. 阿奴独自见族长时仍保留逍遥在队，或临时角色切换后账号失去控制；会合、麒麟演出后的顺序错误。
4. 族长、盖罗娇、唐钰的会议、战争与水灵珠交代遗漏；会合及进入麒麟洞的指引没有实际显示。
5. 地图规划直接移位或穿过原版碰撞，普通敌人与麒麟被跳过；全队败北或逃跑被记作获胜。
6. 火灵珠与麒麟角提前出现、漏发、顺序颠倒或重复领取；凤凰蛋壳或水灵珠被提前加入背包。
7. 取得火灵珠时、麒麟变人时、赠角时及结束时保存丢失脚本续行，重播奖励或改变队伍。
8. 战斗已接受行动中的保存清掉有效指令，补跑离线时长，或续档后仍接受旧输入。
9. 已结束的麒麟交互仍能重新开战或重复取得奖励；重复确认未真正提交普通输入。
10. 首次场景或奖励边界存档发生在下一次输入之后，保存清掉了测试尚未消费的操作。
11. 浏览器使用其他运行的剧情存档或陈旧结果；没有终端 PASS、同次入口摘要及可核验归档。
12. PC／手机不能阅读原版麒麟对白、手动操作归队队员或个人续档；虚拟按钮、状态行使画布缩小。

调试续跑须标记 `resumedTestRoute: true`，不能计入主线完成数。完整重跑通过、同次剧情入口完成 PC／手机核验、归档摘要验证后，才增加开发测试进度。HP9999 仅用于开发测试，不增加原版难度 4/24。

```sh
. scripts/env.sh
PAL_CAMPAIGN_VARIANT=scene-wave PAL_DEVELOPMENT_HEALTH=9999 PAL_DALI_KIRIN=1 go test -v ./games/pal/tests -run '^TestOriginalDaliMeetingKirinSacredRewardsAndSave$' -count=1 -timeout=45m
```

浏览器入口取同次完整重跑的 `dali-kirin-human-form-reading-state-1.json`。保存时火灵珠已按原版取得，阿奴暂以场景人物出现，队伍仅留逍遥；阅读结束后恢复阿奴／逍遥顺序。须核对入口 SHA-256、原版阿奴头像、赠角与神殿指引、实际触摸、原版队伍、个人续档及最大比例画布。

```sh
. scripts/env.sh
PAL_BOARD_BROWSER=1 PAL_BOARD_ENTRY=solo-kirin-human PAL_BOARD_VARIANT=scene-wave go test -v ./internal/server -run '^TestPALBoardBrowserFixture$' -count=1 -timeout=20m
```

在另一终端执行：

```sh
. scripts/env.sh
PAL_BOARD_VARIANT=scene-wave python3 -B games/pal/tests/solo-board-browser.py --kirin-human
```

当前完整重跑已通过，`resumedTestRoute: false`：105 次阅读、42 场正常胜利、102 次新运行器续档，终点为第 207 场景、第 163 张地图，阿奴／逍遥均由本人控制，现金 68,504 文。火灵珠和火眼麒麟角各 1 件，面对原麒麟交互位置的 3 次确认没有重复剧情、战斗或奖励；凤凰蛋壳、水灵珠仍未取得。两次临时单人队伍、战斗已接受行动及两次奖励提示的续档均通过。

同次浏览器入口 SHA-256 为 `d03f08d7583b258e6e240b0daa1f1c78af8252e519cd5e39ab9cf1f26014c8a7`。8 项检查、40 次手机阅读、RGM 28、原版归队、键鼠／触屏、私人续玩、陌生账号拒绝和最大比例画布通过，截图已复核。原版参考源码 60 个文件、资源 104 个文件及当前测试核心摘要核对一致。摘要与归档为 `.local/pal/verification/dali-kirin-stage.json` 和 `development-health-stage-19/`；原版难度仍为 4/24，大理可选收集单独验收。
