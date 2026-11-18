# 开发加血测试

用户已确认，加血仅用于开发测试，便于推进剧情和检查个人存档。正式游戏保留 98 原版数值；本功能不增加正式难度选项。

## 构建与隔离

`build/native.py --test-battle-hp 9999` 生成独立测试核心。每次进入战斗时，将当前队员的体力和体力上限设为 9999；不增加真气，不修改敌人、物品、奖励或剧情脚本。加血只写入生成的战斗测试代码，不改动 `native/upstream/` 或原始资源。

测试产物位于 `.local/pal/test-health/hp-9999/`，不会覆盖 `.local/pal/native/rules.wasm`。测试核心使用实际 Wasm 摘要作为构建身份；正式和测试运行器互相拒绝对方的存档。不能修改存档的构建标识来绕过隔离。

剧情测试须显式设置 `PAL_DEVELOPMENT_HEALTH=9999`。该变量只由 Go 测试辅助代码读取，正式游戏模块不提供加血操作。剧情检查点及报告写入 `.local/pal/verification/development-health/hp-9999/`，不会覆盖原版验收记录。

候选核心回归额外设置 `PAL_CAMPAIGN_VARIANT=<候选名>`，名称仅接受小写英文字母、数字和连字符。核心从 `.local/pal/runtime-candidates/<候选名>/hp-9999/rules.wasm` 读取，证据写入 `.local/pal/verification/runtime-candidates/<候选名>/development-health/hp-9999/`；原版数值候选使用同目录的 `original/rules.wasm`。不同构建从新游戏执行，不能复制旧存档并改写身份。场景波动候选的复现与验收范围见[原版场景波动](SCENE_WAVE.md)。

## 复现

在仓库根目录执行：

```sh
. scripts/env.sh
python3 -B games/pal/build/native.py --test-battle-hp 9999 --battle-fixture 1
python3 -B games/pal/build/native.py --test-battle-hp 9999
go test -v ./games/pal/tests -run '^TestDevelopmentHealth(BattleSaveIsolation|Harness.*)$' -count=1 -timeout=3m
PAL_DEVELOPMENT_HEALTH=9999 go test -v ./games/pal/tests -run '^TestDevelopmentHealthCampaignThroughHiddenDragon$' -count=1 -timeout=25m
# 从其实际隐龙窟存档继续第 6 段
PAL_DEVELOPMENT_HEALTH=9999 go test -v ./games/pal/tests -run '^TestOriginalWhiteRiverReunionGatheringTreatmentAndSave$' -count=1 -timeout=15m
# 第 7 段及玉佛珠装备，按此顺序执行
PAL_DEVELOPMENT_HEALTH=9999 go test -v ./games/pal/tests -run '^TestOriginalJadeTempleBattlesBeadAndSave$' -count=1 -timeout=18m
PAL_DEVELOPMENT_HEALTH=9999 go test -v ./games/pal/tests -run '^TestOriginalJadeBeadEquipmentAndSavedLingRole$' -count=1 -timeout=4m
# 第 8 段从已装备玉佛珠的实际存档继续
PAL_DEVELOPMENT_HEALTH=9999 go test -v ./games/pal/tests -run '^TestOriginalBlackwaterGeneralTombFallAndSave$' -count=1 -timeout=20m
# 第 9 段及土灵珠实际脱离，按此顺序执行
PAL_DEVELOPMENT_HEALTH=9999 go test -v ./games/pal/tests -run '^TestOriginalBloodPoolRedGhostKingEarthOrbAndSave$' -count=1 -timeout=20m
PAL_DEVELOPMENT_HEALTH=9999 go test -v ./games/pal/tests -run '^TestOriginalEarthOrbSelectedSaveAndEscape$' -count=1 -timeout=4m
# 第 10 段使用场景波动候选的实际土灵珠脱离存档
PAL_CAMPAIGN_VARIANT=scene-wave PAL_DEVELOPMENT_HEALTH=9999 go test -v ./games/pal/tests -run '^TestOriginalGhostMountainRescueLingDepartureAndSave$' -count=1 -timeout=25m
# 第 11 段从候选鬼阴山实际终点继续；可选赏金另行验收
PAL_CAMPAIGN_VARIANT=scene-wave PAL_DEVELOPMENT_HEALTH=9999 go test -v ./games/pal/tests -run '^TestOriginalYangzhouThiefTrialRescueAndSave$' -count=1 -timeout=25m
# 第 12 段从候选扬州终点继续
PAL_CAMPAIGN_VARIANT=scene-wave PAL_DEVELOPMENT_HEALTH=9999 go test -v ./games/pal/tests -run '^TestOriginalToadMountainPoisonOrbMiaoPartyAndSave$' -count=1 -timeout=30m
# 第 13 段从候选实际抵京终点继续；隐藏绣花鞋另走可选分支
PAL_CAMPAIGN_VARIANT=scene-wave PAL_DEVELOPMENT_HEALTH=9999 go test -v ./games/pal/tests -run '^TestOriginalCapitalManorIllnessDuelRitualAndSave$' -count=1 -timeout=30m
PAL_CAMPAIGN_VARIANT=scene-wave PAL_DEVELOPMENT_HEALTH=9999 go test -v ./games/pal/tests -run '^TestOriginalCapitalManorHiddenShoesAndSavedNoDuplication$' -count=1 -timeout=6m
# 第 14 段从候选尚书府法事实际终点继续
PAL_CAMPAIGN_VARIANT=scene-wave PAL_DEVELOPMENT_HEALTH=9999 go test -v ./games/pal/tests -run '^TestOriginalCaiyiPoisonForestSacrificeFlightAndSave$' -count=1 -timeout=30m
# 从同次毒林入口走可选收集分支，不并入主线终点库存
PAL_CAMPAIGN_VARIANT=scene-wave PAL_DEVELOPMENT_HEALTH=9999 go test -v ./games/pal/tests -run '^TestOriginalPoisonForestThirteenRewardsAndSavedNoDuplication$' -count=1 -timeout=25m
PAL_CAMPAIGN_VARIANT=scene-wave PAL_DEVELOPMENT_HEALTH=9999 go test -v ./games/pal/tests -run '^TestOriginalShuSaintJadeBeadTeachingGiftsAndTowerSave$' -count=1 -timeout=30m
```

隔离验收结果为 `.local/pal/verification/development-health-isolation.json`，覆盖原版真气与战利品、选招中保存、全新运行器续档、旧输入拒绝及正式／测试存档互拒。各测试构建的 `build.json` 记录实际摘要和 `testBattleHP`。

白河村的真实页面验收使用开发构建中选中的六神丹存档。启动 `PAL_BOARD_BROWSER=1 PAL_BOARD_ENTRY=solo-medicine go test -v ./internal/server -run '^TestPALBoardBrowserFixture$' -count=1 -timeout=35m` 后，在另一终端加载 `scripts/env.sh` 并执行 `python3 -B games/pal/tests/solo-board-browser.py --medicine`；截图和结果位于 `.local/pal/solo-medicine-browser/`。脚本会关闭自己的夹具和浏览器，并清理 profile。

玉佛寺页面同样使用独立终端：以 `PAL_BOARD_ENTRY=solo-jade-bead` 启动上述夹具，执行 `python3 -B games/pal/tests/solo-board-browser.py --jade-bead`。该入口验证原版灵儿单独在场的对白续档、幻境消失和三人队伍恢复，结果位于 `.local/pal/solo-jade-bead-browser/`。

血池入口使用 `PAL_BOARD_ENTRY=solo-blood-pool` 启动夹具，再执行 `python3 -B games/pal/tests/solo-board-browser.py --blood-pool`。入口来自将军战后真实跌落剧情中的逍遥单人对白存档；结果位于 `.local/pal/solo-blood-pool-browser/`，验证个人续档、触屏阅读、三人归队、键鼠／触屏移动和最大比例画布。该测试不代表血池波动效果已完成。

赤鬼王战后的五灵珠对白使用 `PAL_BOARD_ENTRY=solo-earth-orb` 和 `python3 -B games/pal/tests/solo-board-browser.py --earth-orb`，产物位于 `.local/pal/solo-earth-orb-browser/`。选中的土灵珠使用 `PAL_BOARD_ENTRY=solo-earth-escape` 和 `python3 -B games/pal/tests/solo-board-browser.py --earth-escape`，产物位于 `.local/pal/solo-earth-escape-browser/`；该入口验证保存后的原生触屏选物、确认使用及返回墓外。

候选页面在夹具与客户端两边均设置 `PAL_BOARD_VARIANT=scene-wave`，读取候选实际剧情存档；产物单独写入 `.local/pal/candidate-scene-wave/solo-<入口>-browser/`。候选支持六神丹、玉佛珠、血池会合、五灵珠对白、土灵珠使用和鬼阴山入口，是否验收通过以各入口实际结果为准。鬼阴山使用 `PAL_BOARD_ENTRY=solo-ghost-mountain` 和 `python3 -B games/pal/tests/solo-board-browser.py --ghost-mountain`，从灵儿告别实际续玩到原版休息房间，验证逍遥／月如队伍和救治；不能改写初始存档的脚本阶段来制造对白入口。

候选边界证据中，`initial-view.json` 为实际冻结画面，`initial-restored-boundary-view.json` 为恢复后尚未在线推进的画面，`resumed-view.json` 为加载就绪后的实时画面。前两个逐项核对；最后一个的波动及原版菜单高亮允许正常在线继续。单人页面还执行第二次移动后保存，通用 `frozen-view.json` 与 `restored-boundary-view.json` 因而对应第二次保存。

扬州页面使用 `PAL_BOARD_ENTRY=solo-yangzhou` 与 `python3 -B games/pal/tests/solo-board-browser.py --yangzhou`，两边均设置 `PAL_BOARD_VARIANT=scene-wave`。入口为原版无罪释放对白的实际存档，验证单独逍遥续玩后恢复本人操作的逍遥／月如队伍、真实手机阅读、键鼠／触屏移动和最大比例画布。该页面不验证可选赏金。

蛤蟆山后续页面使用 `PAL_BOARD_ENTRY=solo-gaijiao` 与 `python3 -B games/pal/tests/solo-board-browser.py --gaijiao`，两边均设置 `PAL_BOARD_VARIANT=scene-wave`。入口为救醒盖罗娇后的实际对白，验证灵儿被剑圣带走的交代、前往京城及原版月如／逍遥顺序。苗人自动战斗的原始脚本 `0x008A` 与 UI 自动攻击开关不是同一标志；剧情验收通过无战斗选招的实际续行和受伤后续档核对，不因 UI 自动攻击标志缺失而改写原版演出。

尚书府法事页面使用 `PAL_BOARD_ENTRY=solo-capital-ritual` 与 `python3 -B games/pal/tests/solo-board-browser.py --capital-ritual`，两边均设置 `PAL_BOARD_VARIANT=scene-wave`。入口为同次第 13 段重跑的原始对白存档，四项道具已经交付，原版舞台人物仍在、操作队伍只留逍遥。验证法事、师父睡着、月如离开的 12 次触屏阅读，以及个人续档和最大比例画布。调试可用 `PAL_CAPITAL_MANOR_CHECKPOINT=step-N` 或 `failure` 从实际检查点续跑；报告标记 `resumedTestRoute: true`，不能代替从抵京终点整段重跑。

彩依页面使用 `PAL_BOARD_ENTRY=solo-caiyi-sacrifice` 与 `python3 -B games/pal/tests/solo-board-browser.py --caiyi-sacrifice`，两边均设置 `PAL_BOARD_VARIANT=scene-wave`。入口为同次第 14 段重跑的最后心愿，恢复时保留原版正在出字的续行，随后进入实际阅读。36 次触屏阅读经过牺牲、三段回忆、赴蜀山和山门对白，恢复逍遥／月如队伍。调试入口为 `PAL_CAIYI_FOREST_CHECKPOINT=step-N` 或 `failure`；同样不能用调试续跑增加整段完成数。

蜀山授艺页面使用 `PAL_BOARD_ENTRY=solo-shu-gifts` 与 `python3 -B games/pal/tests/solo-board-browser.py --shu-gifts`，两边均设置 `PAL_BOARD_VARIANT=scene-wave`。入口为同次第 15 段酒神限制说明的真实续行，8 次手机画面点按后恢复逍遥／月如；操作方式见[画面直接操作](SCREEN_INPUT.md)。调试可设 `PAL_SHU_TOWER_CHECKPOINT=step-N` 或 `failure`，报告须标记调试续跑，不能替代整段验收。

锁妖塔前半从第 15 段实际终点执行，完整命令和调试边界见[锁妖塔前半验收](TOWER_UPPER_TESTING.md)。同次书中仙对白页面使用 `PAL_BOARD_ENTRY=solo-tower-sage` 与 `python3 -B games/pal/tests/solo-board-browser.py --tower-sage`，两边均设置 `PAL_BOARD_VARIANT=scene-wave`。原版 RGM 69、两次画面阅读及个人续档通过。

桥道触控另使用 `PAL_BOARD_ENTRY=solo-tower-bridges` 与 `--tower-bridges`，读取保留的实际 `tower-layer-input-source-state-1.json`。原版层级 48 用于遮挡计算和排序，不改变人物绘制位置，不能加到点击位置；按精灵实际矩形验证长按、四向移动、释放、原版菜单和个人保存。该独立功能回归不增加剧情段数。

蜀山炼丹房 7 处隐藏物品由 `TestOriginalShuAlchemySevenHiddenRewardsAndSavedNoDuplication` 单独验证，使用实际入室存档、28 次续档和每处 3 次重复搜索，不并入主线背包。详情见[隐藏物品清单](HIDDEN_ITEMS.md)。

## 验收口径

- 加血运行可验证实际地图、事件、队伍聚散、道具、奖励及存档续行；报告须标明开发加血模式。
- 按用户最新确认，加血流程通过即计入本次剧情验收，不再要求原版难度托管通关。它仍不代表原版体力下托管已经获胜，测试构建不进入正式游戏。
- 剧情完成数统一遵循[剧情进度表](STORY_PROGRESS.md)，已通过的加血流程可计入 `a/24`；新核心重新验证的进度另列，不重复增加剧情段数。
- 正式与测试存档互拒只证明隔离有效。正式版本之间的兼容继续按[平台存档规范](../../rules/saved-games.md)验收。

## 新核心的后段根档绑定

后段测试原来固定场景波动核心的根档摘要。不同候选必须登记自己实际通过的上一段终点，不能沿用旧根档、修改内存 BuildID 或删除摘要校验。`tests/campaign-roots.json` 按候选与开发配置保留核心摘要、根档摘要及通过报告摘要；只在实际完整分段成功后追加对应记录。旧场景波动路径继续使用原先固定摘要。

调整根档检查前须覆盖：未知候选、错误开发配置、核心字节变化、根档缺失或变化、报告缺失或变化、未通过报告、调试续跑报告、无效文件名及无效登记格式。候选不得因为恰好匹配旧摘要而回退到旧配置。该记录是测试来源绑定，不迁移、改写或重新标记游戏存档。
